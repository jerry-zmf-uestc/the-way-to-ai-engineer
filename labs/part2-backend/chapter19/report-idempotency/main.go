package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

type record struct{ payload, id string }

// The lab represents one caller. A real key namespace includes caller and operation.
type repository struct {
	mu   sync.Mutex
	jobs map[string]record
}

func (s *repository) submit(key, payload string) (record, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.jobs[key]; ok {
		return previous, false, previous.payload == payload
	}
	job := record{payload, fmt.Sprintf("export-%d", len(s.jobs)+1)}
	s.jobs[key] = job
	return job, true, true
}

type observation struct {
	responseLost             bool
	firstCommit, retryResult string
	commits                  int
}

func exercise(reuseKey bool) (observation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store := repository{jobs: make(map[string]record)}
	committed := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 128))
		key := r.Header.Get("Idempotency-Key")
		if err != nil || key == "" || len(body) == 0 {
			http.Error(w, "invalid submission", http.StatusBadRequest)
			return
		}
		job, created, matched := store.submit(key, string(body))
		if !matched {
			http.Error(w, "key already used with different input", http.StatusConflict)
			return
		}
		if created && job.id == "export-1" {
			conn, _, err := http.NewResponseController(w).Hijack()
			if err != nil {
				http.Error(w, "fault injection failed", http.StatusInternalServerError)
				return
			}
			committed <- job.id
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, job.id)
	}))
	defer server.Close()

	post := func(key string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/exports", strings.NewReader("team=demo&format=csv"))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Idempotency-Key", key)
		// This lab controls every retry explicitly.
		req.GetBody = nil
		return server.Client().Do(req)
	}

	first, err := post("submission-1")
	if err == nil {
		first.Body.Close()
		return observation{}, fmt.Errorf("expected a lost response, got %d", first.StatusCode)
	}
	var result observation
	select {
	case result.firstCommit = <-committed:
		result.responseLost = true
	default:
		return result, fmt.Errorf("request failed without commit evidence: %w", err)
	}

	key := "submission-2"
	if reuseKey {
		key = "submission-1"
	}
	retry, err := post(key)
	if err != nil {
		return result, err
	}
	defer retry.Body.Close()
	if retry.StatusCode != http.StatusAccepted {
		return result, fmt.Errorf("unexpected retry status %d", retry.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(retry.Body, 32))
	store.mu.Lock()
	result.retryResult, result.commits = string(body), len(store.jobs)
	store.mu.Unlock()
	return result, err
}

func main() {
	for _, reuse := range []bool{false, true} {
		m, err := exercise(reuse)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("reuse_key=%t response_lost=%t first_commit=%s retry_result=%s commits=%d\n",
			reuse, m.responseLost, m.firstCommit, m.retryResult, m.commits)
	}
}
