package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type sample struct {
	status int
	total  time.Duration
	err    error
}

// Each accepted request reserves a place, then waits for one of two workers.
func compare(queueCapacity int) error {
	const count = 60
	const interval = 2 * time.Millisecond
	places := make(chan struct{}, 2+queueCapacity)
	workers := make(chan struct{}, 2)
	var maxWait atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queued := time.Now()
		select {
		case places <- struct{}{}:
			defer func() { <-places }()
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		select {
		case workers <- struct{}{}:
			defer func() { <-workers }()
		case <-r.Context().Done():
			return
		}
		wait := time.Since(queued).Microseconds()
		for old := maxWait.Load(); wait > old && !maxWait.CompareAndSwap(old, wait); old = maxWait.Load() {
		}
		time.Sleep(12 * time.Millisecond) // synthetic export work
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 3 * time.Second
	send := func() sample {
		start := time.Now()
		resp, err := client.Post(server.URL+"/exports", "text/plain", nil)
		if err != nil {
			return sample{err: err}
		}
		resp.Body.Close()
		return sample{resp.StatusCode, time.Since(start), nil}
	}
	samples := make(chan sample, count)
	var clients sync.WaitGroup
	start := time.Now()
	for i := range count {
		time.Sleep(time.Until(start.Add(time.Duration(i) * interval)))
		clients.Go(func() { samples <- send() })
	}
	clients.Wait()
	elapsed := time.Since(start)
	var latencies []time.Duration
	rejected := 0
	for range count {
		m := <-samples
		if m.err != nil {
			return m.err
		}
		switch m.status {
		case http.StatusNoContent:
			latencies = append(latencies, m.total)
		case http.StatusServiceUnavailable:
			rejected++
		default:
			return fmt.Errorf("unexpected status %d", m.status)
		}
	}
	if len(latencies) == 0 {
		return fmt.Errorf("no successful requests")
	}
	slices.Sort(latencies)
	p95 := latencies[(95*len(latencies)+99)/100-1]
	recovery := send()
	if recovery.err != nil || recovery.status != http.StatusNoContent {
		return fmt.Errorf("recovery status=%d error=%v", recovery.status, recovery.err)
	}
	fmt.Printf("queue=%d offered=%d completed=%d rejected=%d rejection_pct=%.1f completed_per_s=%.1f success_p95=%s max_queue_wait=%s recovery=%d\n",
		queueCapacity, count, len(latencies), rejected, 100*float64(rejected)/count,
		float64(len(latencies))/elapsed.Seconds(), p95.Round(time.Millisecond),
		(time.Duration(maxWait.Load()) * time.Microsecond).Round(time.Millisecond), recovery.status)
	return nil
}
