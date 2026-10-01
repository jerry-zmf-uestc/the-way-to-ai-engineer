package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChain_Order(t *testing.T) {
	var got []string
	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = append(got, "enter-"+name)
				next.ServeHTTP(w, r)
				got = append(got, "exit-"+name)
			})
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, "handler")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/order", nil)
	rec := httptest.NewRecorder()
	NewChain(mw("a"), mw("b"), mw("c")).Then(handler).ServeHTTP(rec, req)

	want := []string{"enter-a", "enter-b", "enter-c", "handler", "exit-c", "exit-b", "exit-a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("middleware order = %v, want %v", got, want)
	}
}

func TestLogging_Fields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/log", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	record := decodeLogLine(t, buf.String())
	if record["msg"] != "http_request" {
		t.Fatalf("log msg = %v, want http_request", record["msg"])
	}
	for _, field := range []string{"method", "path", "status", "duration_ms", "bytes", "remote"} {
		if _, ok := record[field]; !ok {
			t.Fatalf("log missing field %q: %v", field, record)
		}
	}
	if record["method"] != http.MethodPost || record["path"] != "/log" || record["remote"] != "127.0.0.1:12345" {
		t.Fatalf("log request fields = %v", record)
	}
	if got := int(record["status"].(float64)); got != http.StatusCreated {
		t.Fatalf("status = %d, want %d", got, http.StatusCreated)
	}
	if got := int(record["bytes"].(float64)); got != len("hello") {
		t.Fatalf("bytes = %d, want %d", got, len("hello"))
	}
}

func TestRecover_Panic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := Recover(logger)(mux)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(strings.TrimSpace(rec.Body.String()), "internal error") {
		t.Fatalf("panic body = %q, want internal error", rec.Body.String())
	}
	record := decodeLogLine(t, buf.String())
	if record["level"] != "ERROR" || !strings.Contains(record["msg"].(string), "panic") {
		t.Fatalf("panic log = %v", record)
	}
	if _, ok := record["stack"].(string); !ok {
		t.Fatalf("panic log missing stack: %v", record)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("subsequent status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestTimeout_ExceedsThresholdAndCancelsContext(t *testing.T) {
	observed := make(chan error, 1)
	handler := Timeout(50*time.Millisecond, "timeout")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			observed <- r.Context().Err()
		case <-time.After(time.Second):
			observed <- nil
		}
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slow", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("timeout status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "timeout") {
		t.Fatalf("timeout body = %q, want contains timeout", rec.Body.String())
	}

	select {
	case err := <-observed:
		if err != context.DeadlineExceeded {
			t.Fatalf("handler context err = %v, want DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not observe context cancellation")
	}
}

func TestStatusRecorder_FirstStatusAndImplicitOK(t *testing.T) {
	rec := httptest.NewRecorder()
	status := &statusRecorder{ResponseWriter: rec}

	n, err := status.Write([]byte("abc"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	status.WriteHeader(http.StatusTeapot)

	if n != 3 {
		t.Fatalf("Write() bytes = %d, want 3", n)
	}
	if status.status != http.StatusOK {
		t.Fatalf("status = %d, want implicit 200", status.status)
	}
	if status.bytes != 3 {
		t.Fatalf("bytes = %d, want 3", status.bytes)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("recorder code = %d, want 200", rec.Code)
	}
}

func decodeLogLine(t *testing.T, raw string) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("no log lines captured")
	}
	record := map[string]any{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &record); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, raw)
	}
	return record
}
