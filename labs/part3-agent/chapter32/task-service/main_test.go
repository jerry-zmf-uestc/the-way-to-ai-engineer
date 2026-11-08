package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

func testService(t *testing.T, worker tasks.Worker) *service {
	t.Helper()
	m, err := tasks.NewManager(2, time.Second, worker)
	if err != nil {
		t.Fatal(err)
	}
	s := newService(m, map[string]tasks.Scope{"demo-alice": {Tenant: "demo", User: "alice"}, "demo-bob": {Tenant: "demo", User: "bob"}})
	t.Cleanup(func() {
		s.BeginDrain()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := s.WaitDrain(ctx); err != nil {
			t.Error(err)
		}
	})
	return s
}
func call(s *service, method, path, token, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	return rec
}
func view(t *testing.T, r *httptest.ResponseRecorder) taskView {
	t.Helper()
	var task taskView
	if err := json.Unmarshal(r.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	return task
}
func TestBoundaryValidationAndOwnership(t *testing.T) {
	s := testService(t, func(context.Context, tasks.Task) (string, error) { return "ok", nil })
	if r := call(s, "POST", "/tasks", "", "key", `{"input":"x"}`); r.Code != 401 {
		t.Fatal(r.Code)
	}
	for _, body := range []string{`{"input":"x","user":"bob"}`, `{"input":"x"} {}`, `{`, `{"input":5}`, `{"input":""}`} {
		if r := call(s, "POST", "/tasks", "demo-alice", "key", body); r.Code != 400 {
			t.Fatal(body, r.Code, r.Body.String())
		}
	}
	if r := call(s, "POST", "/tasks", "demo-alice", "key", `{"input":"`+strings.Repeat("a", 9000)+`"}`); r.Code != 413 {
		t.Fatal(r.Code)
	}
	req := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"input":"x"}`))
	req.Header.Set("Authorization", "Bearer demo-alice")
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != 415 {
		t.Fatal(rec.Code)
	}
	taskRec := call(s, "POST", "/tasks", "demo-alice", "key", `{"input":"x"}`)
	if taskRec.Code != 202 {
		t.Fatal(taskRec.Code)
	}
	task := view(t, taskRec)
	if taskRec.Header().Get("Location") != "/tasks/"+task.ID || taskRec.Header().Get("X-Request-ID") == "" {
		t.Fatal(taskRec.Header())
	}
	for _, method := range []string{"GET", "PATCH", "POST"} {
		path := "/tasks/" + task.ID
		body := `{"input":"new","revision":1}`
		if method == "POST" {
			path += "/cancel"
			body = `{"revision":1}`
		}
		if r := call(s, method, path, "demo-bob", "", body); r.Code != 404 {
			t.Fatal(method, r.Code)
		}
	}
	bob := call(s, "GET", "/tasks", "demo-bob", "", "")
	if strings.TrimSpace(bob.Body.String()) != "[]" {
		t.Fatal(bob.Body.String())
	}
	if strings.Contains(taskRec.Body.String(), "Tenant") || strings.Contains(taskRec.Body.String(), "Scope") {
		t.Fatal("private task record exposed")
	}
}
func TestConcurrentIdempotencyAndCapacity(t *testing.T) {
	s := testService(t, func(ctx context.Context, t tasks.Task) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	var wg sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 12)
	for range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- call(s, "POST", "/tasks", "demo-alice", "key", `{"input":"x"}`) }()
	}
	wg.Wait()
	close(responses)
	id := ""
	for r := range responses {
		if r.Code != 202 {
			t.Fatal(r.Code)
		}
		task := view(t, r)
		if id != "" && id != task.ID {
			t.Fatal("duplicate tasks")
		}
		id = task.ID
	}
	if r := call(s, "POST", "/tasks", "demo-alice", "key", `{"input":"different"}`); r.Code != 409 {
		t.Fatal(r.Code)
	}
	if r := call(s, "POST", "/tasks", "demo-bob", "key", `{"input":"y"}`); r.Code != 202 {
		t.Fatal(r.Code)
	}
	if r := call(s, "POST", "/tasks", "demo-alice", "third", `{"input":"z"}`); r.Code != 503 || r.Header().Get("Retry-After") == "" {
		t.Fatal(r.Code, r.Header())
	}
	if r := call(s, "POST", "/tasks", "demo-alice", "key", `{"input":"x"}`); r.Code != 202 {
		t.Fatal("retry lost under capacity", r.Code)
	}
}
func TestDisconnectDoesNotCancelAdmittedTask(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	s := testService(t, func(ctx context.Context, t tasks.Task) (string, error) {
		close(entered)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-release:
			return "ok", nil
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"input":"x"}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer demo-alice")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "key")
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	task := view(t, rec)
	<-entered
	cancel()
	r := call(s, "GET", "/tasks/"+task.ID, "demo-alice", "", "")
	if r.Code != 200 || view(t, r).State != "running" {
		t.Fatal(r.Body.String())
	}
	close(release)
	result, err := s.manager.Wait(context.Background(), tasks.Scope{Tenant: "demo", User: "alice"}, task.ID)
	if err != nil || result.State != "completed" {
		t.Fatal(result, err)
	}
}
func TestRevisionUpdateCancelAndShutdown(t *testing.T) {
	s := testService(t, func(ctx context.Context, t tasks.Task) (string, error) { <-ctx.Done(); return "", ctx.Err() })
	task := view(t, call(s, "POST", "/tasks", "demo-alice", "key", `{"input":"x"}`))
	updated := call(s, "PATCH", "/tasks/"+task.ID, "demo-alice", "", `{"input":"new","revision":1}`)
	if updated.Code != 202 || view(t, updated).Revision != 2 {
		t.Fatal(updated.Body.String())
	}
	if r := call(s, "PATCH", "/tasks/"+task.ID, "demo-alice", "", `{"input":"old","revision":1}`); r.Code != 409 {
		t.Fatal(r.Code)
	}
	canceled := call(s, "POST", "/tasks/"+task.ID+"/cancel", "demo-alice", "", `{"revision":2}`)
	if canceled.Code != 202 {
		t.Fatal(canceled.Code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Shutdown(ctx, &http.Server{}); err != nil {
		t.Fatal(err)
	}
	if r := call(s, "GET", "/readyz", "", "", ""); r.Code != 503 {
		t.Fatal(r.Code)
	}
	if r := call(s, "GET", "/livez", "", "", ""); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if r := call(s, "POST", "/tasks", "demo-alice", "new", `{"input":"x"}`); r.Code != 503 {
		t.Fatal(r.Code)
	}
}
func TestShutdownTimeoutIsNotWorkerCompletion(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	s := testService(t, func(context.Context, tasks.Task) (string, error) { close(entered); <-release; return "late", nil })
	task := view(t, call(s, "POST", "/tasks", "demo-alice", "key", `{"input":"x"}`))
	<-entered
	s.BeginDrain()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := s.WaitDrain(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		close(release)
		t.Fatal(err)
	}
	close(release)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if err := s.WaitDrain(ctx2); err != nil {
		t.Fatal(err)
	}
	result, err := s.manager.Check(tasks.Scope{Tenant: "demo", User: "alice"}, task.ID)
	if err != nil || result.State != "canceled" || result.Output != "" {
		t.Fatal(fmt.Sprint(result, err))
	}
}
