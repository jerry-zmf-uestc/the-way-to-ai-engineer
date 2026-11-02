package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/gateway"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/obs"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/order"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/product"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/runtime"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/summary"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/user"
)

func Run(stdout, stderr io.Writer, _ []string) int {
	cfg := config.FromEnv()
	logger := obs.NewLogger(stderr, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := RunWithConfig(ctx, stdout, cfg, logger); err != nil {
		logger.ErrorContext(ctx, "demo failed", slog.Any("error", err))
		return 1
	}
	return 0
}

func RunWithConfig(ctx context.Context, out io.Writer, cfg config.Config, logger *slog.Logger) (err error) {
	runner := runtime.NewRunner(logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownBudget(cfg, runner.Len()))
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

	result, err := summary.Fetch(ctx, cfg, gatewayServer.Addr())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "demo=user-orders status=%d orders=%d total_cents=%d trace=%s attempts=%d\n",
		result.Status,
		result.Orders,
		result.TotalCents,
		result.Trace,
		result.Attempts,
	)
	return err
}

func ShutdownBudget(cfg config.Config, servers int) time.Duration {
	if servers < 1 {
		servers = 1
	}
	return time.Duration(servers) * cfg.ShutdownTimeout
}
