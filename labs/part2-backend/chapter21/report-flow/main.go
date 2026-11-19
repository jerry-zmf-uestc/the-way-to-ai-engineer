package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var page []byte

var faults = []string{"none", "stale-cache", "lost-ack", "instance-down", "lost-response", "overload", "backlog"}

type job struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Instance     string `json:"instance,omitempty"`
	Result       string `json:"result,omitempty"`
	ResultWrites int    `json:"result_writes"`
	owner        string
	revision     int
	enqueued     time.Time
}

type submissionKey struct{ owner, key string }

type executor struct {
	name      string
	available bool
}

var errUnavailable = errors.New("executor unavailable")

func (e executor) export(id string) (string, error) {
	if !e.available {
		return "", errUnavailable
	}
	return "report_id,total\n" + id + ",42\n", nil
}

type counters struct {
	Accepted        int   `json:"accepted"`
	Reused          int   `json:"reused"`
	Rejected        int   `json:"rejected"`
	ResponseDrops   int   `json:"response_drops"`
	Deliveries      int   `json:"deliveries"`
	AckDrops        int   `json:"ack_drops"`
	Duplicates      int   `json:"duplicates"`
	Completed       int   `json:"completed"`
	Depth           int   `json:"depth"`
	PeakDepth       int   `json:"peak_depth"`
	OldestWaitMS    int64 `json:"oldest_wait_ms"`
	CacheStale      int   `json:"cache_stale"`
	InstanceFailure int   `json:"instance_failures"`
}

type lab struct {
	store     *sqliteStore
	recovered int
	mu        sync.Mutex
	fault     string
	runID     string
	paused    bool
	draining  bool
	capacity  int
	wake      chan struct{}
	jobs      map[string]job
	keys      map[submissionKey]string
	cache     map[string]job
	queue     []string
	retry     string
	ready     []executor
	counts    counters
	logger    *slog.Logger
}

func newLab(fault string, logger *slog.Logger) *lab {
	capacity := 8
	if fault == "overload" {
		capacity = 2
	}
	return &lab{
		fault: fault, runID: rand.Text(), paused: fault == "backlog" || fault == "overload" || strings.HasPrefix(fault, "crash-"), capacity: capacity,
		wake: make(chan struct{}, 1), jobs: make(map[string]job), keys: make(map[submissionKey]string),
		cache: make(map[string]job), ready: []executor{{"A", fault != "instance-down"}, {"B", true}}, logger: logger,
	}
}

func (l *lab) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *lab) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(page)
	})
	mux.HandleFunc("POST /exports", l.submit)
	mux.HandleFunc("GET /exports/{id}", l.query)
	mux.HandleFunc("GET /metrics", l.metrics)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		l.mu.Lock()
		draining := l.draining
		l.mu.Unlock()
		if draining {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /lab/drain", func(w http.ResponseWriter, _ *http.Request) {
		l.mu.Lock()
		l.draining = true
		l.paused = false
		l.signal()
		l.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /lab/resume", func(w http.ResponseWriter, _ *http.Request) {
		l.mu.Lock()
		l.paused = false
		l.signal()
		l.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// A fixed local identity fixture. Production identity comes from authentication.
func learner(r *http.Request) string {
	switch r.Header.Get("X-Lab-User") {
	case "", "alice":
		return "alice"
	case "bob":
		return "bob"
	default:
		return ""
	}
}

func (l *lab) submit(w http.ResponseWriter, r *http.Request) {
	owner := learner(r)
	if owner == "" {
		http.Error(w, "unknown lab identity", http.StatusUnauthorized)
		return
	}
	key := submissionKey{owner, strings.TrimSpace(r.Header.Get("Idempotency-Key"))}
	if key.key == "" || len(key.key) > 128 {
		http.Error(w, "Idempotency-Key required (1–128 bytes)", http.StatusBadRequest)
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	var input struct {
		Format string `json:"format"`
	}
	if err := decoder.Decode(&input); err != nil || input.Format != "csv" || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "expected a single JSON object with format csv", http.StatusBadRequest)
		return
	}

	l.mu.Lock()
	if id, ok := l.keys[key]; ok {
		l.counts.Reused++
		j := l.jobs[id]
		l.mu.Unlock()
		writeJSON(w, http.StatusAccepted, j)
		return
	}
	if l.draining {
		l.mu.Unlock()
		http.Error(w, "export server draining", http.StatusServiceUnavailable)
		return
	}
	if len(l.queue) >= l.capacity {
		l.counts.Rejected++
		l.mu.Unlock()
		http.Error(w, "export queue full", http.StatusServiceUnavailable)
		return
	}
	id := "export-" + strconv.Itoa(l.counts.Accepted+1)
	j := job{ID: id, Status: "queued", owner: owner, enqueued: time.Now()}
	if l.store != nil {
		if err := l.store.accept(l.counts.Accepted+1, j, key); err != nil {
			// A commit error can leave its outcome unknown. Reload from SQLite.
			l.persistenceFailure(err)
		}
	}
	l.jobs[id], l.keys[key] = j, id
	l.queue = append(l.queue, id)
	l.counts.Accepted++
	l.counts.PeakDepth = max(l.counts.PeakDepth, len(l.queue))
	drop := l.fault == "lost-response" && l.counts.Accepted == 1
	l.logger.Info("enqueued", "job", id)
	l.signal()
	l.mu.Unlock()

	if drop {
		conn, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			http.Error(w, "fault injection failed", http.StatusInternalServerError)
			return
		}
		l.mu.Lock()
		l.counts.ResponseDrops++
		l.mu.Unlock()
		l.logger.Info("response_dropped", "job", id)
		if r.Header.Get("X-Report-Lab-Client") == "browser" {
			// Chrome can replay a request if the connection closes before headers.
			// Send an incomplete body so the learner can explicitly retry.
			fmt.Fprint(conn, "HTTP/1.1 202 Accepted\r\nContent-Type: application/json\r\nContent-Length: 32\r\nConnection: close\r\n\r\n{")
		}
		conn.Close()
		return
	}
	writeJSON(w, http.StatusAccepted, j)
}

func (l *lab) query(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	l.mu.Lock()
	j, ok := l.jobs[id]
	if !ok || j.owner != learner(r) {
		l.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	if cached, hit := l.cache[id]; hit {
		if cached.revision == j.revision {
			j = cached
		} else {
			l.counts.CacheStale++
			l.logger.Info("stale_cache_discarded", "job", id)
		}
	}
	if j.Status == "succeeded" {
		l.cache[id] = j
	}
	l.mu.Unlock()
	writeJSON(w, http.StatusOK, j)
}

func (l *lab) metrics(w http.ResponseWriter, _ *http.Request) {
	l.mu.Lock()
	m := struct {
		counters
		Fault     string `json:"fault"`
		RunID     string `json:"run_id"`
		Paused    bool   `json:"paused"`
		StorageID string `json:"storage_id,omitempty"`
		Recovered int    `json:"recovered_pending"`
	}{counters: l.counts, Fault: l.fault, RunID: l.runID, Paused: l.paused, Recovered: l.recovered}
	if l.store != nil {
		m.StorageID = l.store.id
	}
	m.Depth = len(l.queue)
	if m.Depth > 0 {
		m.OldestWaitMS = time.Since(l.jobs[l.queue[0]].enqueued).Milliseconds()
	}
	l.mu.Unlock()
	writeJSON(w, http.StatusOK, m)
}

func (l *lab) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		}
		for ctx.Err() == nil && l.step() {
		}
	}
}

func (l *lab) step() bool {
	l.mu.Lock()
	if l.paused || (len(l.queue) == 0 && l.retry == "") {
		l.mu.Unlock()
		return false
	}
	id := l.retry
	if id != "" {
		l.retry = ""
	} else {
		id = l.queue[0]
		l.queue = l.queue[1:]
	}
	j := l.jobs[id]
	l.counts.Deliveries++
	if j.Status == "succeeded" {
		l.counts.Duplicates++
		l.logger.Info("duplicate_delivery", "job", id)
		l.ack(id)
		l.mu.Unlock()
		return true
	}
	waitMS := time.Since(j.enqueued).Milliseconds()
	j.Status = "running"
	j.revision++
	l.jobs[id] = j
	l.logger.Info("running", "job", id)
	if l.fault == "stale-cache" && id == "export-1" {
		l.cache[id] = j
	}
	worker := l.ready[0]
	l.mu.Unlock()

	if l.fault == "crash-before-result" {
		l.logger.Info("injected_crash", "job", id, "window", "before_result")
		os.Exit(73)
	}
	result, err := worker.export(id)

	l.mu.Lock()
	defer l.mu.Unlock()
	if errors.Is(err, errUnavailable) {
		l.counts.InstanceFailure++
		l.logger.Info("instance_unavailable", "job", id, "instance", worker.name)
		l.ready = l.ready[1:]
		worker = l.ready[0] // B is always ready in this lab.
		result, _ = worker.export(id)
	}
	j.Instance, j.Result = worker.name, result
	if l.store != nil {
		if err := l.store.finish(j); err != nil {
			l.persistenceFailure(err)
		}
	}
	j.ResultWrites++
	j.Status = "succeeded"
	j.revision++
	l.jobs[id] = j
	l.counts.Completed++
	l.logger.Info("completed", "job", id, "instance", j.Instance,
		"queue_wait_ms", waitMS)
	if l.fault == "crash-after-result" {
		l.logger.Info("injected_crash", "job", id, "window", "after_result_before_ack")
		os.Exit(73)
	}
	if l.fault == "lost-ack" && id == "export-1" {
		l.counts.AckDrops++
		l.logger.Info("ack_dropped", "job", id)
		l.retry = id
	} else {
		l.ack(id)
	}
	return true
}

func (l *lab) persistenceFailure(err error) {
	l.logger.Error("sqlite_failure", "error", err)
	os.Exit(1)
}

func (l *lab) ack(id string) {
	if l.store != nil {
		if err := l.store.ack(id); err != nil {
			l.persistenceFailure(err)
		}
	}
}

func main() {
	port := flag.Int("port", 8080, "loopback HTTP port")
	fault := flag.String("fault", "none", "none, stale-cache, lost-ack, instance-down, lost-response, overload, backlog, crash-before-result, crash-after-result")
	db := flag.String("db", "", "SQLite database file (empty uses memory)")
	flag.Parse()
	if strings.HasPrefix(*fault, "crash-") && *db == "" {
		log.Fatal("crash modes require -db")
	}
	if !slices.Contains(faults, *fault) && *fault != "crash-before-result" && *fault != "crash-after-result" {
		log.Fatal("unknown fault")
	}
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	l := newLab(*fault, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if *db != "" {
		if err := l.openSQLite(*db); err != nil {
			log.Fatal(err)
		}
	}
	srv := &http.Server{Handler: l.routes(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		l.work(ctx)
	}()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		<-ctx.Done()
		deadline, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		srv.Shutdown(deadline)
	}()
	fmt.Printf("listening=http://%s fault=%s\n", listener.Addr(), *fault)
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	cancel()
	<-workerDone
	<-serverDone
}
