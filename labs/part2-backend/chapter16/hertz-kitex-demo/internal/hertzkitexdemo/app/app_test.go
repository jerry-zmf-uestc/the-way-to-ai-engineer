package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
)

func TestRunSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(&stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	want := "demo=user-orders status=200 orders=2 total_cents=2497 trace=Frontend→Hertz→UserSvc→OrderSvc→ProductSvc attempts=1\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunLogsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := RunWithConfig(ctx, &out, config.Default(), logger)
	if err == nil {
		t.Fatal("RunWithConfig() error = nil, want canceled context error")
	}
	if !strings.Contains(err.Error(), "start product") {
		t.Fatalf("RunWithConfig() error = %v, want contains start product", err)
	}
}

func TestRunWithConfig_PrintsDeterministicSummary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := RunWithConfig(ctx, &out, config.Default(), logger); err != nil {
		t.Fatalf("RunWithConfig() error = %v", err)
	}

	want := "demo=user-orders status=200 orders=2 total_cents=2497 trace=Frontend→Hertz→UserSvc→OrderSvc→ProductSvc attempts=1\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestShutdownBudgetScalesWithStartedServers(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	if got := ShutdownBudget(cfg, 4); got != 4*cfg.ShutdownTimeout {
		t.Fatalf("ShutdownBudget(4) = %v, want %v", got, 4*cfg.ShutdownTimeout)
	}
	if got := ShutdownBudget(cfg, 0); got != cfg.ShutdownTimeout {
		t.Fatalf("ShutdownBudget(0) = %v, want %v", got, cfg.ShutdownTimeout)
	}
}
