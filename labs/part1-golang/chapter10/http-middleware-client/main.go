// main.go wires the middleware chain to an httptest.NewServer, then drives
// four demo branches through Client.DoWithRetry to prove the end-to-end path:
//
//	demo=echo         status=200 body={"echo":"ok"}   attempts=1
//	demo=recover      status=500 body=internal error  attempts=1
//	demo=timeout      status=503 body=timeout         attempts=1
//	demo=retry-then-ok status=200 body={"echo":"ok"}  attempts=3
//
// Implementation lands in task 2.6.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// run is the demo entry so main_test.go can capture its stdout deterministically.
func run() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	attempts := newAttemptCounter()
	mux := http.NewServeMux()

	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		attempts.Add("echo")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"echo":"ok"}`))
	})
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		attempts.Add("recover")
		panic("demo panic")
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		attempts.Add("timeout")
		<-r.Context().Done()
	})
	mux.HandleFunc("/flaky", func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add("retry-then-ok") < 3 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"echo":"ok"}`))
	})

	handler := NewChain(
		Logging(logger),
		Timeout(50*time.Millisecond, "timeout"),
		Recover(logger),
	).Then(mux)
	server := httptest.NewServer(handler)
	defer server.Close()

	noRetry := demoClient(1)
	retry := demoClient(3)
	printDemo("echo", noRetry, server.URL+"/echo", attempts)
	printDemo("recover", noRetry, server.URL+"/panic", attempts)
	printDemo("timeout", noRetry, server.URL+"/slow", attempts)
	printDemo("retry-then-ok", retry, server.URL+"/flaky", attempts)
}

func main() {
	run()
}

func demoClient(maxAttempts int) *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 2 * time.Second},
		Policy: RetryPolicy{
			MaxAttempts: maxAttempts,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    10 * time.Millisecond,
		},
	}
}

func printDemo(name string, client *Client, url string, attempts *attemptCounter) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("demo=%s status=0 body=%s attempts=%d\n", name, err, attempts.Get(name))
		return
	}
	resp, err := client.DoWithRetry(context.Background(), req)
	if err != nil {
		fmt.Printf("demo=%s status=0 body=%s attempts=%d\n", name, err, attempts.Get(name))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("demo=%s status=%d body=%s attempts=%d\n", name, resp.StatusCode, err, attempts.Get(name))
		return
	}
	fmt.Printf("demo=%s status=%d body=%s attempts=%d\n", name, resp.StatusCode, strings.TrimSpace(string(body)), attempts.Get(name))
}

type attemptCounter struct {
	mu sync.Mutex
	m  map[string]int
}

func newAttemptCounter() *attemptCounter {
	return &attemptCounter{m: map[string]int{}}
}

func (c *attemptCounter) Add(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[name]++
	return c.m[name]
}

func (c *attemptCounter) Get(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[name]
}
