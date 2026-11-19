// This file implements the client side:
//
//   - RetryPolicy         —— MaxAttempts / BaseDelay / MaxDelay / Jitter / IsRetryable.
//   - Client              —— wraps *http.Client with policy-driven retry.
//   - DoWithRetry         —— main entry, honors ctx / Retry-After / idempotency.
//   - DefaultIsRetryable  —— GET/HEAD/PUT/DELETE + (transport error || 5xx).
//   - parseRetryAfter     —— second-granularity Retry-After header parser.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy declares the retry cadence and the "retryable" predicate.
// The zero value means "no retry".
type RetryPolicy struct {
	// MaxAttempts is the total number of tries including the first one.
	// Values <= 1 disable retry.
	MaxAttempts int
	// BaseDelay is the base for the exponential backoff.
	BaseDelay time.Duration
	// MaxDelay caps the per-attempt backoff.
	MaxDelay time.Duration
	// Jitter is the upper bound of the randomized jitter added to each backoff
	// to avoid thundering herd.
	Jitter time.Duration
	// IsRetryable decides whether the (resp, err) pair should trigger retry.
	// When nil, DefaultIsRetryable is used.
	IsRetryable func(*http.Response, error) bool
}

// Client wraps *http.Client with a RetryPolicy. It is safe for concurrent use
// as long as the underlying HTTP client is.
type Client struct {
	// HTTP is the underlying client. Callers must set Timeout in production.
	HTTP *http.Client
	// Policy governs retry cadence and retryability.
	Policy RetryPolicy
	// Logger receives one debug line per attempt. Optional.
	Logger *slog.Logger

	// now is injectable for tests; nil means time.Now.
	now func() time.Time
	// sleep is injectable for tests; nil means ctx-aware time.After.
	sleep func(context.Context, time.Duration) error
}

// DoWithRetry sends req respecting c.Policy and ctx.
//
// Semantics mirror http.Client.Do: when the returned error is nil the caller
// must close resp.Body. Bodies of intermediate (retried) responses are closed
// and drained by DoWithRetry itself.
//
// ctx cancellation aborts pending backoff and returns context.Canceled or
// context.DeadlineExceeded, whichever fires first.
func (c *Client) DoWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	policy := c.Policy
	maxAttempts := policy.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	isRetryable := policy.IsRetryable
	if isRetryable == nil {
		isRetryable = DefaultIsRetryable
	}
	sleep := c.sleep
	if sleep == nil {
		sleep = sleepContext
	}

	methodRetryable := isIdempotent(req.Method)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		attemptReq := req.Clone(ctx)
		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("reset request body: %w", err)
			}
			attemptReq.Body = body
		}

		resp, err := httpClient.Do(attemptReq)
		if err != nil {
			lastErr = err
		}

		shouldRetry := methodRetryable && attempt < maxAttempts && isRetryable(resp, err)
		if !shouldRetry {
			if err != nil {
				return resp, fmt.Errorf("attempt %d: %w", attempt, err)
			}
			return resp, nil
		}

		drainAndClose(resp)
		delay := retryDelay(policy, attempt, resp, c.now)
		if c.Logger != nil {
			c.Logger.LogAttrs(ctx, slog.LevelDebug, "http_retry",
				slog.Int("attempt", attempt),
				slog.String("method", req.Method),
				slog.String("url", req.URL.String()),
				slog.Int64("retry_in_ms", delay.Milliseconds()),
			)
		}
		if err := sleep(ctx, delay); err != nil {
			return nil, fmt.Errorf("retry backoff: %w", err)
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("attempt %d: %w", maxAttempts, lastErr)
	}
	return nil, nil
}

// DefaultIsRetryable returns true when the request method is idempotent
// (GET / HEAD / PUT / DELETE) and either the transport failed or the server
// responded with 5xx (or 429).
func DefaultIsRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode >= http.StatusInternalServerError && resp.StatusCode <= 599)
}

// isIdempotent reports whether HTTP method m is safe to replay.
func isIdempotent(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

// parseRetryAfter parses a Retry-After header value assuming seconds.
// Returns (0, false) when the header is empty, negative, or not a valid int.
// HTTP-date form (RFC 7231) is intentionally out of scope for this topic.
func parseRetryAfter(header string) (time.Duration, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, false
	}
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds < 0 {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

func retryDelay(policy RetryPolicy, attempt int, resp *http.Response, now func() time.Time) time.Duration {
	if resp != nil {
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return d
		}
	}

	delay := policy.BaseDelay
	if delay < 0 {
		delay = 0
	}
	for i := 1; i < attempt; i++ {
		delay *= 2
	}
	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}
	if policy.Jitter > 0 {
		seedTime := time.Now()
		if now != nil {
			seedTime = now()
		}
		r := rand.New(rand.NewSource(seedTime.UnixNano() + int64(attempt)))
		delay += time.Duration(r.Int63n(int64(policy.Jitter) + 1))
	}
	return delay
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
