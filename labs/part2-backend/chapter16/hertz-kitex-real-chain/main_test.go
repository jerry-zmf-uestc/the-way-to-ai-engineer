package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/config"
)

func TestRunAppExitCodes(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		code := runApp(&stdout, &stderr, func(ctx context.Context, out io.Writer, cfg config.Config, logger *slog.Logger) error {
			_, _ = io.WriteString(out, "ok\n")
			return nil
		})
		if code != 0 {
			t.Fatalf("runApp() code = %d, want 0", code)
		}
		if stdout.String() != "ok\n" {
			t.Fatalf("stdout = %q, want ok newline", stdout.String())
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()
		var stdout, stderr bytes.Buffer
		sentinel := errors.New("boom")
		code := runApp(&stdout, &stderr, func(ctx context.Context, out io.Writer, cfg config.Config, logger *slog.Logger) error {
			return sentinel
		})
		if code != 1 {
			t.Fatalf("runApp() code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "demo failed") || !strings.Contains(stderr.String(), "boom") {
			t.Fatalf("stderr = %q, want demo failed log with boom", stderr.String())
		}
	})
}

func TestRunDemo_PrintsDeterministicSummary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := runDemo(ctx, &out, config.Default(), logger)
	if err != nil {
		t.Fatalf("runDemo() error = %v", err)
	}

	want := "demo=user-orders status=200 orders=2 total_cents=2497 trace=Frontend→Hertz→UserSvc→OrderSvc→ProductSvc attempts=1\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestShutdownBudgetScalesWithStartedServers(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	if got := shutdownBudget(cfg, 4); got != 4*cfg.ShutdownTimeout {
		t.Fatalf("shutdownBudget(4) = %v, want %v", got, 4*cfg.ShutdownTimeout)
	}
	if got := shutdownBudget(cfg, 0); got != cfg.ShutdownTimeout {
		t.Fatalf("shutdownBudget(0) = %v, want %v", got, cfg.ShutdownTimeout)
	}
}

func TestFetchSummarySuccess(t *testing.T) {
	t.Parallel()

	var gotTrace string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTrace = r.Header.Get("X-Trace-ID")
		_, _ = io.WriteString(w, `{"status":200,"orders":2,"total_cents":2497,"trace":"Frontend→Hertz→UserSvc→OrderSvc→ProductSvc","attempts":1}`)
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	summary, err := fetchSummary(context.Background(), config.Default(), addr)
	if err != nil {
		t.Fatalf("fetchSummary() error = %v", err)
	}
	if summary.Status != http.StatusOK || summary.Orders != 2 || summary.TotalCents != 2497 || summary.Attempts != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if gotTrace != demoTrace {
		t.Fatalf("trace header = %q, want %q", gotTrace, demoTrace)
	}
}

func TestFetchSummaryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "bad json", status: http.StatusOK, body: "{", wantErr: "decode gateway response"},
		{name: "non ok", status: http.StatusBadGateway, body: `{"status":502}`, wantErr: "gateway status 502"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()

			addr := strings.TrimPrefix(srv.URL, "http://")
			_, err := fetchSummary(context.Background(), config.Default(), addr)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("fetchSummary() error = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}
