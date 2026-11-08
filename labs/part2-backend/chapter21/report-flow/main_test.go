package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReportFlow(t *testing.T) {
	for _, fault := range faults {
		t.Run(fault, func(t *testing.T) {
			l := newLab(fault, slog.New(slog.NewTextHandler(io.Discard, nil)))
			s := httptest.NewServer(l.routes())
			defer s.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			done := make(chan struct{})
			go func() { defer close(done); l.work(ctx) }()
			defer func() { cancel(); <-done }()

			call := func(method, path, key string, status int, dst any) {
				t.Helper()
				body := ""
				if key != "" {
					body = `{"format":"csv"}`
				}
				req, err := http.NewRequestWithContext(ctx, method, s.URL+path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				if key != "" {
					req.Header.Set("Idempotency-Key", key)
					req.GetBody = nil
				}
				resp, err := s.Client().Do(req)
				if err != nil && status == 0 && ctx.Err() == nil {
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != status {
					t.Fatalf("%s %s: got status %d, want %d", method, path, resp.StatusCode, status)
				}
				if dst != nil {
					if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
						t.Fatal(err)
					}
				}
			}
			getMetrics := func() counters {
				var m counters
				call("GET", "/metrics", "", 200, &m)
				return m
			}

			var first job
			if fault == "lost-response" {
				call("POST", "/exports", "one", 0, nil)
				if m := getMetrics(); m.Accepted != 1 || m.ResponseDrops != 1 {
					t.Fatalf("no evidence of commit before disconnect: %+v", m)
				}
			}
			call("POST", "/exports", "one", 202, &first)
			if first.ID != "export-1" {
				t.Fatalf("unexpected id: %+v", first)
			}
			ids := []string{first.ID}
			if fault == "backlog" || fault == "overload" {
				var second, third job
				call("POST", "/exports", "two", 202, &second)
				ids = append(ids, second.ID)
				if fault == "backlog" {
					call("POST", "/exports", "three", 202, &third)
					ids = append(ids, third.ID)
				} else {
					call("POST", "/exports", "three", 503, nil)
				}
				var queued job
				call("GET", "/exports/"+first.ID, "", 200, &queued)
				if m := getMetrics(); m.Depth != len(ids) || m.Completed != 0 || queued.Status != "queued" {
					t.Fatalf("expected queued tasks: %+v, %+v", m, queued)
				}
				call("POST", "/lab/resume", "", 204, nil)
			}

			waitResult := func(id string) {
				t.Helper()
				for {
					var j job
					call("GET", "/exports/"+id, "", 200, &j)
					if j.Status == "succeeded" {
						instance := "A"
						if fault == "instance-down" {
							instance = "B"
						}
						if j.ID != id || j.Instance != instance || j.Result != "report_id,total\n"+id+",42\n" {
							t.Fatalf("unexpected result: %+v", j)
						}
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
			for _, id := range ids {
				waitResult(id)
			}
			if fault == "overload" {
				var third job
				call("POST", "/exports", "three", 202, &third)
				waitResult(third.ID)
				ids = append(ids, third.ID)
			}
			m := getMetrics()
			for fault == "lost-ack" && m.Duplicates == 0 {
				time.Sleep(time.Millisecond)
				m = getMetrics()
			}
			if m.Completed != len(ids) || m.Accepted != len(ids) || m.Depth != 0 {
				t.Fatalf("unexpected final metrics: %+v", m)
			}
			switch fault {
			case "stale-cache":
				if m.CacheStale != 1 {
					t.Fatalf("expected stale read: %+v", m)
				}
			case "lost-ack":
				if m.Deliveries != 2 || m.AckDrops != 1 || m.Duplicates != 1 {
					t.Fatalf("expected redelivery: %+v", m)
				}
			case "instance-down":
				if m.InstanceFailure != 1 {
					t.Fatalf("expected failed instance: %+v", m)
				}
			case "lost-response":
				if m.Reused != 1 {
					t.Fatalf("expected same key reuse: %+v", m)
				}
			case "overload":
				if m.Rejected != 1 || m.PeakDepth != 2 {
					t.Fatalf("expected overload and recovery: %+v", m)
				}
			case "backlog":
				if m.PeakDepth != 3 {
					t.Fatalf("expected backlog and recovery: %+v", m)
				}
			}
		})
	}
}

func TestBrowserCanRetryTruncatedResponse(t *testing.T) {
	l := newLab("lost-response", slog.New(slog.NewTextHandler(io.Discard, nil)))
	s := httptest.NewServer(l.routes())
	defer s.Close()
	post := func() (string, error) {
		req, err := http.NewRequest(http.MethodPost, s.URL+"/exports", strings.NewReader(`{"format":"csv"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Idempotency-Key", "same-export")
		req.Header.Set("X-Report-Lab-Client", "browser")
		req.GetBody = nil
		resp, err := s.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("unexpected status: %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}
	if body, err := post(); body != "{" || err != io.ErrUnexpectedEOF {
		t.Fatalf("expected a truncated response, got %q, %v", body, err)
	}
	body, err := post()
	var j job
	if err != nil || json.Unmarshal([]byte(body), &j) != nil || j.ID != "export-1" {
		t.Fatalf("retry did not recover export-1: %q, %v", body, err)
	}
	resp, err := s.Client().Get(s.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var m counters
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil || m.Accepted != 1 || m.ResponseDrops != 1 || m.Reused != 1 {
		t.Fatalf("unexpected metrics: %+v, %v", m, err)
	}
}

func TestExportOwnership(t *testing.T) {
	l := newLab("none", slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := l.routes()
	call := func(method, path, owner string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(`{"format":"csv"}`))
		req.Header.Set("Idempotency-Key", "same-key")
		req.Header.Set("X-Lab-User", owner)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != want {
			t.Fatalf("%s %s as %s: status %d, want %d", method, path, owner, resp.Code, want)
		}
		return resp
	}
	call("POST", "/exports", "alice", 202)
	call("GET", "/exports/export-1", "bob", 404)
	call("GET", "/exports/export-1", "alice", 200)
	if resp := call("POST", "/exports", "bob", 202); !strings.Contains(resp.Body.String(), "export-2") {
		t.Fatal("two owners using the same key must not share a job")
	}
}

func TestDrainKeepsAcceptedWorkQueryable(t *testing.T) {
	l := newLab("backlog", slog.New(slog.NewTextHandler(io.Discard, nil)))
	server := httptest.NewServer(l.routes())
	defer server.Close()
	client := server.Client()
	client.Timeout = 2 * time.Second
	call := func(method, path, key string, want int) string {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(`{"format":"csv"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Idempotency-Key", key)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != want {
			t.Fatalf("%s %s: status=%d body=%q err=%v", method, path, resp.StatusCode, body, err)
		}
		return string(body)
	}
	call("GET", "/readyz", "", 204)
	call("POST", "/exports", "first", 202)
	call("POST", "/lab/drain", "", 204)
	call("GET", "/readyz", "", 503)
	call("POST", "/exports", "second", 503)
	call("POST", "/exports", "first", 202)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); l.work(ctx) }()
	defer func() { cancel(); <-done }()
	for deadline := time.Now().Add(time.Second); ; {
		if strings.Contains(call("GET", "/exports/export-1", "", 200), "export-1,42") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("accepted work did not drain")
		}
		time.Sleep(time.Millisecond)
	}
}
