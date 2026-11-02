// Package main hosts the http middleware + client demo.
//
// This file implements the middleware side:
//
//   - Middleware   —— func(http.Handler) http.Handler, onion-model unit.
//   - Chain        —— compose Middleware in declaration order.
//   - Logging      —— structured log for method/path/status/duration/bytes.
//   - Recover      —— catch panic in handler goroutine, return 500.
//   - Timeout      —— wrap http.TimeoutHandler for per-request deadline.
//   - statusRecorder —— ResponseWriter wrapper to observe status + bytes.
package main

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// Middleware decorates an http.Handler and returns the wrapped handler.
// Onion model: the first declared middleware is the outermost layer.
type Middleware func(http.Handler) http.Handler

// Chain composes Middleware values in declaration order.
//
// NewChain(a, b, c).Then(h) is equivalent to a(b(c(h))).
type Chain struct {
	mws []Middleware
}

// NewChain returns a Chain that will apply mws in the given order.
func NewChain(mws ...Middleware) Chain {
	copied := make([]Middleware, len(mws))
	copy(copied, mws)
	return Chain{mws: copied}
}

// Then wraps h with the middleware chain and returns the composed handler.
func (c Chain) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.NotFoundHandler()
	}
	for i := len(c.mws) - 1; i >= 0; i-- {
		if c.mws[i] == nil {
			continue
		}
		h = c.mws[i](h)
	}
	return h
}

// Logging returns a Middleware that emits one structured log line per request.
//
// Fields: method / path / status / duration_ms / bytes / remote.
// When logger is nil, slog.Default() is used.
func Logging(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			status := rec.status
			if !rec.written {
				status = http.StatusOK
			}
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.Int64("bytes", rec.bytes),
				slog.String("remote", r.RemoteAddr),
			)
		})
	}
}

// Recover returns a Middleware that recovers from panics in the handler
// goroutine, writes HTTP 500, and logs the panic value plus stack trace.
func Recover(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("panic", recovered),
						slog.String("stack", string(debug.Stack())),
					)
					http.Error(w, "internal error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout returns a Middleware that bounds each request to d.
// On timeout the wrapped handler is abandoned and 503 msg is written.
//
// Implemented on top of http.TimeoutHandler; msg is the body sent on timeout.
func Timeout(d time.Duration, msg string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, msg)
	}
}

// statusRecorder wraps http.ResponseWriter and captures the first status code
// plus the accumulated body byte count.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	bytes   int64
	written bool
}

// WriteHeader records the status the first time it is called and forwards it.
func (r *statusRecorder) WriteHeader(code int) {
	if r.written {
		return
	}
	r.status = code
	r.written = true
	r.ResponseWriter.WriteHeader(code)
}

// Write forwards to the underlying ResponseWriter and counts written bytes.
// If WriteHeader was not called explicitly, it is implicitly 200.
func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}
