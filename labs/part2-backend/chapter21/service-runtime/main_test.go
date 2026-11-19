package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDefaultsAndValidation(t *testing.T) {
	cfg, err := LoadConfig(map[string]string{})
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.ServiceName != "checkout-runtime" {
		t.Fatalf("ServiceName = %q, want checkout-runtime", cfg.ServiceName)
	}
	if cfg.Environment != "dev" {
		t.Fatalf("Environment = %q, want dev", cfg.Environment)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.RequestTimeout != 2*time.Second {
		t.Fatalf("RequestTimeout = %v, want 2s", cfg.RequestTimeout)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("ShutdownTimeout = %v, want 3s", cfg.ShutdownTimeout)
	}

	_, err = LoadConfig(map[string]string{
		"REQUEST_TIMEOUT":  "0s",
		"SHUTDOWN_TIMEOUT": "1s",
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("LoadConfig() invalid timeout error = %v, want ErrInvalidConfig", err)
	}
}

func TestHandlerHealthAndReadiness(t *testing.T) {
	app := newTestApp(t, Config{
		ServiceName:     "checkout-runtime",
		Environment:     "test",
		HTTPAddr:        ":0",
		RequestTimeout:  20 * time.Millisecond,
		ShutdownTimeout: 50 * time.Millisecond,
	}, fakeCheckoutService{})

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	assertProbe(t, server.URL+"/healthz", http.StatusOK, "ok")
	assertProbe(t, server.URL+"/readyz", http.StatusServiceUnavailable, "starting")

	app.SetReady(true)
	assertProbe(t, server.URL+"/readyz", http.StatusOK, "ready")
}

func TestPlaceOrderHonorsTimeoutAndLogsFailure(t *testing.T) {
	var logs bytes.Buffer
	app := newTestApp(t, Config{
		ServiceName:     "checkout-runtime",
		Environment:     "test",
		HTTPAddr:        ":0",
		RequestTimeout:  10 * time.Millisecond,
		ShutdownTimeout: 50 * time.Millisecond,
	}, fakeCheckoutService{
		placeOrder: func(ctx context.Context, req CheckoutRequest) (OrderSummary, error) {
			<-ctx.Done()
			return OrderSummary{}, ctx.Err()
		},
	}, &logs)
	app.SetReady(true)

	_, err := app.PlaceOrder(context.Background(), CheckoutRequest{
		OrderID:    "ORD-timeout",
		CustomerID: "CUST-7",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("PlaceOrder() error = %v, want context.DeadlineExceeded", err)
	}

	logText := logs.String()
	if !strings.Contains(logText, `"msg":"checkout_failed"`) {
		t.Fatalf("logs missing checkout_failed: %s", logText)
	}
	if !strings.Contains(logText, `"order_id":"ORD-timeout"`) {
		t.Fatalf("logs missing order_id: %s", logText)
	}
}

func TestShutdownMarksNotReadyAndRunsHooks(t *testing.T) {
	var logs bytes.Buffer
	hookCalled := false
	deadlineSeen := false
	app := newTestApp(t, Config{
		ServiceName:     "checkout-runtime",
		Environment:     "test",
		HTTPAddr:        ":0",
		RequestTimeout:  20 * time.Millisecond,
		ShutdownTimeout: 40 * time.Millisecond,
	}, fakeCheckoutService{}, &logs)
	app.AddShutdownHook(func(ctx context.Context) error {
		hookCalled = true
		_, deadlineSeen = ctx.Deadline()
		return nil
	})
	app.SetReady(true)

	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if !hookCalled {
		t.Fatalf("Shutdown() did not call hook")
	}
	if !deadlineSeen {
		t.Fatalf("Shutdown() hook context missing deadline")
	}
	if app.Ready() {
		t.Fatalf("Ready() = true after Shutdown, want false")
	}
	if !strings.Contains(logs.String(), `"msg":"shutdown_complete"`) {
		t.Fatalf("logs missing shutdown_complete: %s", logs.String())
	}
}

type fakeCheckoutService struct {
	placeOrder func(context.Context, CheckoutRequest) (OrderSummary, error)
}

func (f fakeCheckoutService) PlaceOrder(ctx context.Context, req CheckoutRequest) (OrderSummary, error) {
	if f.placeOrder != nil {
		return f.placeOrder(ctx, req)
	}
	return OrderSummary{OrderID: req.OrderID, Status: "created"}, nil
}

func newTestApp(t *testing.T, cfg Config, service CheckoutService, sinks ...*bytes.Buffer) *App {
	t.Helper()

	var logger *slog.Logger
	if len(sinks) > 0 {
		logger = slog.New(slog.NewJSONHandler(sinks[0], nil))
	}

	app, err := NewApp(cfg, logger, service)
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	return app
}

func assertProbe(t *testing.T, url string, wantStatus int, wantState string) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s error = %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s status = %d, want %d", url, resp.StatusCode, wantStatus)
	}

	var payload probePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode(%s) error = %v", url, err)
	}
	if payload.Status != wantState {
		t.Fatalf("GET %s payload status = %q, want %q", url, payload.Status, wantState)
	}
}
