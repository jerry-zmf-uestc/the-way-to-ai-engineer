package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

// TestHTTP distinguishes occupied execution slots from retained terminal records.
func TestHTTPExecutionAndRetentionCapacity(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int64
	m, err := tasks.NewManager(2, time.Minute, func(ctx context.Context, task tasks.Task) (string, error) {
		calls.Add(1)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-release:
			return "evidence", nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	scope := tasks.Scope{Tenant: "demo", User: "alice"}
	s := newService(m, map[string]tasks.Scope{"demo-alice": scope})
	server := httptest.NewServer(s.mux())
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type reply struct {
		key, retry string
		status     int
		task       taskView
		err        error
	}
	submit := func(key string) reply {
		out := reply{key: key}
		req, err := http.NewRequestWithContext(ctx, "POST", server.URL+"/tasks", strings.NewReader(`{"input":"capacity"}`))
		if err != nil {
			out.err = err
			return out
		}
		req.Header.Set("Authorization", "Bearer demo-alice")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		response, err := server.Client().Do(req)
		if err != nil {
			out.err = err
			return out
		}
		defer response.Body.Close()
		out.status, out.retry = response.StatusCode, response.Header.Get("Retry-After")
		body, err := io.ReadAll(io.LimitReader(response.Body, 8193))
		if err != nil || len(body) > 8192 {
			out.err = fmt.Errorf("response bound/read: %v", err)
			return out
		}
		if out.status == 202 {
			out.err = json.Unmarshal(body, &out.task)
		} else {
			var rejected struct{ Error string }
			out.err = json.Unmarshal(body, &rejected)
			if out.err == nil && (out.status != 503 || rejected.Error != "capacity" || out.retry != "1") {
				out.err = fmt.Errorf("unexpected rejection: %d %s", out.status, rejected.Error)
			}
		}
		return out
	}
	start := make(chan struct{})
	responses := make(chan reply, 12)
	for n := range 12 {
		go func() {
			<-start
			responses <- submit(fmt.Sprintf("burst-%d", n))
		}()
	}
	close(start)
	accepted := []reply{}
	refused := 0
	var requestError error
	for range 12 {
		r := <-responses
		if r.err != nil {
			if requestError == nil {
				requestError = r.err
			}
			continue
		}
		if r.status == 202 {
			accepted = append(accepted, r)
		} else {
			refused++
		}
	}
	if requestError != nil {
		t.Fatal(requestError)
	}
	if len(accepted) != 2 || refused != 10 {
		t.Fatalf("accepted=%d refused=%d", len(accepted), refused)
	}
	first := accepted[0]
	if repeat := submit(first.key); repeat.err != nil || repeat.status != 202 || repeat.task.ID != first.task.ID {
		t.Fatal("retry lost while execution full", repeat)
	}
	close(release)
	for _, r := range accepted {
		if task, err := m.Wait(ctx, scope, r.task.ID); err != nil || task.State != "completed" {
			t.Fatal(task, err)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("duplicate worker", calls.Load())
	}
	for n := 2; n < 16; n++ {
		r := submit(fmt.Sprintf("retained-%d", n))
		if r.err != nil || r.status != 202 {
			t.Fatal(r)
		}
		if task, err := m.Wait(ctx, scope, r.task.ID); err != nil || task.State != "completed" {
			t.Fatal(task, err)
		}
	}
	if full := submit("seventeenth"); full.err != nil || full.status != 503 || len(m.List(scope)) != 16 || calls.Load() != 16 {
		t.Fatal("retention capacity bypassed", full, calls.Load())
	}
	if repeat := submit(first.key); repeat.err != nil || repeat.status != 202 || repeat.task.ID != first.task.ID {
		t.Fatal("retry lost while retention full", repeat)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/readyz", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 8193))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != 200 {
		t.Fatal("capacity must not change admission readiness flag", response.StatusCode, readErr, closeErr)
	}
	// Application maintenance advances the test clock; no five-minute wall wait is claimed.
	removed, err := m.Cleanup(time.Now().Add(6*time.Minute), 5*time.Minute)
	if err != nil || removed != 16 {
		t.Fatal(removed, err)
	}
	fresh := submit(first.key)
	if fresh.err != nil || fresh.status != 202 || fresh.task.ID == first.task.ID {
		t.Fatal("new admission after cleanup", fresh)
	}
	if task, err := m.Wait(ctx, scope, fresh.task.ID); err != nil || task.State != "completed" || calls.Load() != 17 {
		t.Fatal(task, err, calls.Load())
	}
	t.Log("HTTP burst=12 accepted=2 capacity_rejected=10; retained=16 new_request=503; cleanup=16 new_id=true; worker_calls=17")
}
