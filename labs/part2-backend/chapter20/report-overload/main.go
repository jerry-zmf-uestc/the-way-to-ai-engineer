package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const burst, capacity = 12, 4

type observation struct{ offered, success, rejected, peak, recovery int }

func exercise(limit int) (observation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	release := make(chan struct{})
	arrived := make(chan struct{}, burst+1)
	slots := make(chan struct{}, limit)
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limit > 0 {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			default:
				w.WriteHeader(http.StatusServiceUnavailable)
				arrived <- struct{}{}
				return
			}
		}
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		arrived <- struct{}{}
		select {
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
			return
		}
	}))
	defer server.Close()

	send := func() (int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exports", nil)
		if err != nil {
			return 0, err
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		return resp.StatusCode, nil
	}
	type reply struct {
		status int
		err    error
	}
	replies := make(chan reply, burst)
	var clients sync.WaitGroup
	for range burst {
		clients.Go(func() {
			status, err := send()
			replies <- reply{status, err}
		})
	}

	// Keep accepted work blocked until every request has reached admission.
	// No sleep or scheduler ordering is used to produce overload.
	for range burst {
		select {
		case <-arrived:
		case <-ctx.Done():
			close(release)
			clients.Wait()
			return observation{}, fmt.Errorf("burst did not reach admission: %w", ctx.Err())
		}
	}
	close(release)
	clients.Wait()
	m := observation{offered: burst, peak: int(peak.Load())}
	for range burst {
		r := <-replies
		if r.err != nil {
			return m, r.err
		}
		switch r.status {
		case http.StatusNoContent:
			m.success++
		case http.StatusServiceUnavailable:
			m.rejected++
		default:
			return m, fmt.Errorf("unexpected burst status %d", r.status)
		}
	}
	var err error
	m.recovery, err = send()
	return m, err
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "compare" {
		for _, queueCapacity := range []int{60, 4, 0} {
			if err := compare(queueCapacity); err != nil {
				log.Fatal(err)
			}
		}
		return
	}
	for _, limit := range []int{0, capacity} {
		m, err := exercise(limit)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("limit=%d offered=%d success=%d rejected=%d peak_inflight=%d recovery_status=%d\n",
			limit, m.offered, m.success, m.rejected, m.peak, m.recovery)
	}
}
