package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoWithRetry_HappyPath(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := newTestClient()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.DoWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("DoWithRetry() error = %v", err)
	}
	defer resp.Body.Close()

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDoWithRetry_Retries5xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := newTestClient()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.DoWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("DoWithRetry() error = %v", err)
	}
	defer resp.Body.Close()

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDoWithRetry_NonIdempotentNoRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "not replayed", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := newTestClient()
	req, err := http.NewRequest(http.MethodPost, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.DoWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("DoWithRetry() error = %v", err)
	}
	defer resp.Body.Close()

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestDoWithRetry_TransportError(t *testing.T) {
	sentinel := errors.New("dial failed")
	attempts := 0
	client := newTestClient()
	client.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, sentinel
	})}

	req, err := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.DoWithRetry(context.Background(), req)
	if !errors.Is(err, sentinel) {
		t.Fatalf("DoWithRetry() error = %v, want sentinel", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestDoWithRetry_CtxCancelInBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	client := newTestClient()
	client.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(nilReader{}),
			Request:    req,
		}, nil
	})}
	client.sleep = func(context.Context, time.Duration) error {
		cancel()
		return ctx.Err()
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.DoWithRetry(ctx, req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DoWithRetry() error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 before canceled backoff", attempts)
	}
}

func TestDoWithRetry_RetryAfterHeader(t *testing.T) {
	var sleeps []time.Duration
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestClient()
	client.sleep = func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.DoWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("DoWithRetry() error = %v", err)
	}
	defer resp.Body.Close()

	if len(sleeps) != 1 || sleeps[0] != time.Second {
		t.Fatalf("sleeps = %v, want [1s]", sleeps)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		in     string
		want   time.Duration
		wantOK bool
	}{
		{in: "1", want: time.Second, wantOK: true},
		{in: " 2 ", want: 2 * time.Second, wantOK: true},
		{in: "", wantOK: false},
		{in: "-1", wantOK: false},
		{in: "soon", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := parseRetryAfter(tt.in)
		if ok != tt.wantOK || got != tt.want {
			t.Fatalf("parseRetryAfter(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestDefaultRetryableAndIdempotentPredicates(t *testing.T) {
	sentinel := errors.New("network")
	if !DefaultIsRetryable(nil, sentinel) {
		t.Fatal("transport errors should be retryable")
	}
	if !DefaultIsRetryable(&http.Response{StatusCode: http.StatusTooManyRequests}, nil) {
		t.Fatal("429 should be retryable")
	}
	if !DefaultIsRetryable(&http.Response{StatusCode: http.StatusInternalServerError}, nil) {
		t.Fatal("5xx should be retryable")
	}
	if DefaultIsRetryable(&http.Response{StatusCode: http.StatusBadRequest}, nil) {
		t.Fatal("4xx except 429 should not be retryable")
	}

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete} {
		if !isIdempotent(method) {
			t.Fatalf("%s should be idempotent", method)
		}
	}
	if isIdempotent(http.MethodPost) {
		t.Fatal("POST should not be idempotent by default")
	}
}

func TestRetryDelayAndSleepContext(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"2"}}}
	if got := retryDelay(RetryPolicy{BaseDelay: time.Millisecond}, 1, resp, nil); got != 2*time.Second {
		t.Fatalf("retryDelay Retry-After = %v, want 2s", got)
	}

	policy := RetryPolicy{BaseDelay: 10 * time.Millisecond, MaxDelay: 15 * time.Millisecond}
	if got := retryDelay(policy, 3, nil, nil); got != 15*time.Millisecond {
		t.Fatalf("retryDelay capped = %v, want 15ms", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(sleepContext(ctx, time.Second), context.Canceled) {
		t.Fatal("sleepContext should return canceled context error")
	}
	if err := sleepContext(context.Background(), 0); err != nil {
		t.Fatalf("sleepContext zero delay = %v, want nil", err)
	}
}

func newTestClient() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 2 * time.Second},
		Policy: RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   time.Millisecond,
			MaxDelay:    time.Millisecond,
		},
		sleep: func(context.Context, time.Duration) error { return nil },
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type nilReader struct{}

func (nilReader) Read([]byte) (int, error) {
	return 0, io.EOF
}
