package gateway_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/gateway"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/obs"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/order"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/product"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/user"
)

func TestGatewayRoute_RealKitex(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()

	productServer, err := product.Start(ctx, logger)
	if err != nil {
		t.Fatalf("start product: %v", err)
	}
	t.Cleanup(func() { _ = productServer.Shutdown(context.Background()) })

	orderServer, err := order.Start(ctx, logger, cfg, productServer.Addr())
	if err != nil {
		t.Fatalf("start order: %v", err)
	}
	t.Cleanup(func() { _ = orderServer.Shutdown(context.Background()) })

	userServer, err := user.Start(ctx, logger, cfg, orderServer.Addr())
	if err != nil {
		t.Fatalf("start user: %v", err)
	}
	t.Cleanup(func() { _ = userServer.Shutdown(context.Background()) })

	gatewayServer, err := gateway.Start(ctx, logger, cfg, userServer.Addr())
	if err != nil {
		t.Fatalf("start gateway: %v", err)
	}
	t.Cleanup(func() { _ = gatewayServer.Shutdown(context.Background()) })

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+gatewayServer.Addr()+"/api/users/u-100/orders", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(obs.TraceIDHeader, "Frontend->Hertz->UserSvc->OrderSvc->ProductSvc")
	resp, err := (&http.Client{Timeout: cfg.HTTPTimeout}).Do(req)
	if err != nil {
		t.Fatalf("GET gateway: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200, body=%s", resp.StatusCode, body)
	}
	if got := resp.Header.Get(obs.TraceIDHeader); got != "Frontend->Hertz->UserSvc->OrderSvc->ProductSvc" {
		t.Fatalf("trace header = %q", got)
	}
	var body struct {
		Status     int    `json:"status"`
		UserID     string `json:"user_id"`
		UserName   string `json:"user_name"`
		Orders     int    `json:"orders"`
		TotalCents int64  `json:"total_cents"`
		Trace      string `json:"trace"`
		Attempts   int    `json:"attempts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != http.StatusOK || body.UserID != "u-100" || body.UserName != "Ada" {
		t.Fatalf("identity body = %+v", body)
	}
	if body.Orders != 2 || body.TotalCents != 2497 {
		t.Fatalf("aggregate body = %+v", body)
	}
	if body.Trace != "Frontend->Hertz->UserSvc->OrderSvc->ProductSvc" || body.Attempts != 1 {
		t.Fatalf("observability body = %+v", body)
	}
}

func TestGatewayHealthz(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()

	productServer, err := product.Start(ctx, logger)
	if err != nil {
		t.Fatalf("start product: %v", err)
	}
	t.Cleanup(func() { _ = productServer.Shutdown(context.Background()) })

	orderServer, err := order.Start(ctx, logger, cfg, productServer.Addr())
	if err != nil {
		t.Fatalf("start order: %v", err)
	}
	t.Cleanup(func() { _ = orderServer.Shutdown(context.Background()) })

	userServer, err := user.Start(ctx, logger, cfg, orderServer.Addr())
	if err != nil {
		t.Fatalf("start user: %v", err)
	}
	t.Cleanup(func() { _ = userServer.Shutdown(context.Background()) })

	gatewayServer, err := gateway.Start(ctx, logger, cfg, userServer.Addr())
	if err != nil {
		t.Fatalf("start gateway: %v", err)
	}
	t.Cleanup(func() { _ = gatewayServer.Shutdown(context.Background()) })

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+gatewayServer.Addr()+"/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := (&http.Client{Timeout: cfg.HTTPTimeout}).Do(req)
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}
