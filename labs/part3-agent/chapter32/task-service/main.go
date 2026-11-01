// Command task-service exposes the same-process task contract through bounded HTTP requests.
// Demo bearer strings are public fixture labels, not production credentials.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

type service struct {
	manager    *tasks.Manager
	identities map[string]tasks.Scope
	ready      atomic.Bool
	shutdown   sync.Once
	drained    chan struct{}
}

func newService(manager *tasks.Manager, identities map[string]tasks.Scope) *service {
	copied := map[string]tasks.Scope{}
	for token, scope := range identities {
		copied[token] = scope
	}
	s := &service{manager: manager, identities: copied, drained: make(chan struct{})}
	s.ready.Store(true)
	return s
}

type taskView struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Revision uint64 `json:"revision"`
	Attempt  int    `json:"attempt"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

func project(t tasks.Task) taskView {
	return taskView{ID: t.ID, State: t.State, Revision: t.Revision, Attempt: t.Attempt, Output: t.Output, Error: t.Failure}
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // Response failure must not undo an already admitted task.
}
func reject(w http.ResponseWriter, status int, code string) {
	respond(w, status, map[string]string{"error": code})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		reject(w, 415, "json_required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	data, err := io.ReadAll(r.Body)
	if err == nil && (!utf8.Valid(data) || uniqueObjectFields(json.NewDecoder(bytes.NewReader(data)), 0) != nil) {
		err = errAmbiguousJSON
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			reject(w, 413, "body_too_large")
		} else {
			reject(w, 400, "invalid_body")
		}
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(v)
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		reject(w, 413, "body_too_large")
	} else {
		reject(w, 400, "invalid_body")
	}
	return false
}
func taskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		reject(w, 404, "not_found")
	case errors.Is(err, tasks.ErrConflict):
		reject(w, 409, "conflict")
	case errors.Is(err, tasks.ErrInput):
		reject(w, 400, "invalid_task")
	case errors.Is(err, tasks.ErrCapacity):
		w.Header().Set("Retry-After", "1")
		reject(w, 503, "capacity")
	case errors.Is(err, tasks.ErrClosed):
		reject(w, 503, "draining")
	default:
		reject(w, 500, "internal")
	}
}
func (s *service) authenticated(handler func(http.ResponseWriter, *http.Request, tasks.Scope)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", rand.Text())
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		scope, known := s.identities[token]
		if !ok || !known {
			w.Header().Set("WWW-Authenticate", "Bearer")
			reject(w, 401, "unauthorized")
			return
		}
		if r.Context().Err() != nil {
			reject(w, 408, "request_canceled")
			return
		}
		handler(w, r, scope)
	}
}
func (s *service) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]bool{"live": true}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !s.ready.Load() {
			reject(w, 503, "draining")
			return
		}
		respond(w, 200, map[string]bool{"ready": true})
	})
	mux.HandleFunc("POST /tasks", s.authenticated(s.start))
	mux.HandleFunc("GET /tasks", s.authenticated(s.list))
	mux.HandleFunc("GET /tasks/{id}", s.authenticated(s.check))
	mux.HandleFunc("PATCH /tasks/{id}", s.authenticated(s.update))
	mux.HandleFunc("POST /tasks/{id}/cancel", s.authenticated(s.cancel))
	return mux
}
func (s *service) start(w http.ResponseWriter, r *http.Request, scope tasks.Scope) {
	if !s.ready.Load() {
		reject(w, 503, "draining")
		return
	}
	var input struct {
		Input string `json:"input"`
	}
	if !decode(w, r, &input) {
		return
	}
	if r.Context().Err() != nil {
		reject(w, 408, "request_canceled")
		return
	}
	task, err := s.manager.Start(scope, r.Header.Get("Idempotency-Key"), strings.TrimSpace(input.Input))
	if err != nil {
		taskError(w, err)
		return
	}
	w.Header().Set("Location", "/tasks/"+task.ID)
	respond(w, 202, project(task))
}
func (s *service) check(w http.ResponseWriter, r *http.Request, scope tasks.Scope) {
	task, err := s.manager.Check(scope, r.PathValue("id"))
	if err != nil {
		taskError(w, err)
		return
	}
	respond(w, 200, project(task))
}
func (s *service) list(w http.ResponseWriter, r *http.Request, scope tasks.Scope) {
	views := []taskView{}
	for _, task := range s.manager.List(scope) {
		views = append(views, project(task))
	}
	respond(w, 200, views)
}
func (s *service) update(w http.ResponseWriter, r *http.Request, scope tasks.Scope) {
	if !s.ready.Load() {
		reject(w, 503, "draining")
		return
	}
	var input struct {
		Input    string `json:"input"`
		Revision uint64 `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	task, err := s.manager.Update(scope, r.PathValue("id"), input.Revision, strings.TrimSpace(input.Input))
	if err != nil {
		taskError(w, err)
		return
	}
	respond(w, 202, project(task))
}
func (s *service) cancel(w http.ResponseWriter, r *http.Request, scope tasks.Scope) {
	var input struct {
		Revision uint64 `json:"revision"`
	}
	if !decode(w, r, &input) {
		return
	}
	task, err := s.manager.Cancel(scope, r.PathValue("id"), input.Revision)
	if err != nil {
		taskError(w, err)
		return
	}
	respond(w, 202, project(task))
}

// BeginDrain closes admission before cancellation, and never declares success until workers exit.
func (s *service) BeginDrain() {
	s.shutdown.Do(func() {
		s.ready.Store(false)
		go func() { s.manager.Close(); close(s.drained) }()
	})
}
func (s *service) WaitDrain(ctx context.Context) error {
	select {
	case <-s.drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *service) Shutdown(ctx context.Context, httpServer *http.Server) error {
	s.BeginDrain()
	httpErr := httpServer.Shutdown(ctx)
	return errors.Join(httpErr, s.WaitDrain(ctx))
}
func fakeWorker(ctx context.Context, t tasks.Task) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(300 * time.Millisecond):
		return "evidence: " + t.Input, nil
	}
}
func main() {
	addr := flag.String("addr", "127.0.0.1:18082", "local listen address")
	flag.Parse()
	host, _, err := net.SplitHostPort(*addr)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		log.Fatal("demo service requires a literal loopback address")
	}
	manager, err := tasks.NewManager(2, 5*time.Second, fakeWorker)
	if err != nil {
		log.Fatal(err)
	}
	s := newService(manager, map[string]tasks.Scope{"demo-alice": {Tenant: "demo", User: "alice"}, "demo-bob": {Tenant: "demo", User: "bob"}})
	httpServer := &http.Server{Addr: *addr, Handler: s.mux(), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		manager.Close()
		log.Fatal("listen failed")
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(listener) }()
	fmt.Printf("task service listening on http://%s\n", *addr)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var listenFailure error
serve:
	for {
		select {
		case <-signalCtx.Done():
			break serve
		case err := <-serveErr:
			if !errors.Is(err, http.ErrServerClosed) {
				listenFailure = err
			}
			break serve
		case now := <-ticker.C:
			_, _ = manager.Cleanup(now, 5*time.Minute)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownCtx, httpServer); err != nil {
		log.Fatal("shutdown incomplete")
	}
	if listenFailure != nil {
		log.Fatal("serve failed")
	}
	log.Print("service drained")
}
