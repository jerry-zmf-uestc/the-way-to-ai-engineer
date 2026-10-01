// Package main is a bounded Hertz + Kitex demo that spins up 4 real servers
// in a single process, drives one HTTP request through the Hertz gateway into
// a 3-hop Kitex chain, prints one deterministic stdout line, and cascades a
// graceful shutdown.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/chain"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/gateway"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/obs"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/order"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/product"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/user"
)

const demoTrace = "Frontend→Hertz→UserSvc→OrderSvc→ProductSvc"

func main() {
	os.Exit(runApp(os.Stdout, os.Stderr, runDemo))
}

type demoRunner func(context.Context, io.Writer, config.Config, *slog.Logger) error

func runApp(stdout, stderr io.Writer, run demoRunner) int {
	cfg := config.FromEnv()
	logger := obs.NewLogger(stderr, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := run(ctx, stdout, cfg, logger); err != nil {
		logger.ErrorContext(ctx, "demo failed", slog.Any("error", err))
		return 1
	}
	return 0
}

func runDemo(ctx context.Context, out io.Writer, cfg config.Config, logger *slog.Logger) (err error) {
	runner := chain.NewRunner(logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownBudget(cfg, runner.Len()))
		defer cancel()
		if shutdownErr := runner.Shutdown(shutdownCtx); shutdownErr != nil {
			if err == nil {
				err = shutdownErr
			} else {
				err = fmt.Errorf("%w; shutdown: %w", err, shutdownErr)
			}
		}
	}()

	productServer, err := product.Start(ctx, logger)
	if err != nil {
		return fmt.Errorf("start product: %w", err)
	}
	runner.Add("product", productServer)

	orderServer, err := order.Start(ctx, logger, cfg, productServer.Addr())
	if err != nil {
		return fmt.Errorf("start order: %w", err)
	}
	runner.Add("order", orderServer)

	userServer, err := user.Start(ctx, logger, cfg, orderServer.Addr())
	if err != nil {
		return fmt.Errorf("start user: %w", err)
	}
	runner.Add("user", userServer)

	gatewayServer, err := gateway.Start(ctx, logger, cfg, userServer.Addr())
	if err != nil {
		return fmt.Errorf("start gateway: %w", err)
	}
	runner.Add("gateway", gatewayServer)

	summary, err := fetchSummary(ctx, cfg, gatewayServer.Addr())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "demo=user-orders status=%d orders=%d total_cents=%d trace=%s attempts=%d\n",
		summary.Status,
		summary.Orders,
		summary.TotalCents,
		summary.Trace,
		summary.Attempts,
	)
	return err
}

type demoSummary struct {
	Status     int    `json:"status"`
	Orders     int    `json:"orders"`
	TotalCents int64  `json:"total_cents"`
	Trace      string `json:"trace"`
	Attempts   int    `json:"attempts"`
}

func fetchSummary(ctx context.Context, cfg config.Config, gatewayAddr string) (demoSummary, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+gatewayAddr+"/api/users/u-100/orders", nil)
	if err != nil {
		return demoSummary{}, fmt.Errorf("new demo request: %w", err)
	}
	req.Header.Set(obs.TraceIDHeader, demoTrace)

	resp, err := (&http.Client{Timeout: cfg.HTTPTimeout}).Do(req)
	if err != nil {
		return demoSummary{}, fmt.Errorf("call gateway: %w", err)
	}
	defer resp.Body.Close()

	var summary demoSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return demoSummary{}, fmt.Errorf("decode gateway response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return demoSummary{}, fmt.Errorf("gateway status %d", resp.StatusCode)
	}
	return summary, nil
}

func shutdownBudget(cfg config.Config, servers int) time.Duration {
	if servers < 1 {
		servers = 1
	}
	return time.Duration(servers) * cfg.ShutdownTimeout
}
