package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/kitex/client"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/chain"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/obs"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/rpc"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce/userservice"
)

// Start launches the Hertz gateway on a loopback random port.
func Start(ctx context.Context, logger *slog.Logger, cfg config.Config, userAddr string) (chain.Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	userClient, err := userservice.NewClient("user",
		client.WithHostPorts(userAddr),
		client.WithRPCTimeout(cfg.RPCTimeout),
		client.WithFailureRetry(rpc.FailureRetryPolicy(cfg.RPCMaxAttempts)),
		client.WithMiddleware(obs.KitexClientObserve(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("new user client: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen gateway: %w", err)
	}
	h := server.New(
		server.WithListener(ln),
		server.WithTransport(standard.NewTransporter),
		server.WithExitWaitTime(cfg.ShutdownTimeout),
	)
	h.Use(obs.HertzInjectTraceID(), obs.HertzRecover(logger), obs.HertzLogging(logger))
	h.GET("/healthz", func(ctx context.Context, c *app.RequestContext) {
		c.String(consts.StatusOK, "ok")
	})
	h.GET("/api/users/:userID/orders", getUserOrders(userClient))

	gs := &hertzServer{
		addr:   ln.Addr().String(),
		engine: h,
		done:   make(chan error, 1),
	}
	go func() {
		gs.done <- h.Run()
	}()
	return gs, nil
}

type hertzServer struct {
	addr   string
	engine *server.Hertz
	done   chan error
}

// Addr returns the bound listener address.
func (s *hertzServer) Addr() string { return s.addr }

// Shutdown gracefully stops the Hertz gateway.
func (s *hertzServer) Shutdown(ctx context.Context) error {
	if err := s.engine.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown gateway: %w", err)
	}
	select {
	case err := <-s.done:
		if err != nil {
			return fmt.Errorf("run gateway: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func getUserOrders(userClient userservice.Client) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		userID := c.Param("userID")
		resp, err := userClient.GetUserOrders(ctx, &commerce.UserOrdersRequest{UserId: userID})
		if err != nil {
			status := statusFromError(err)
			c.JSON(status, map[string]any{
				"status": status,
				"error":  err.Error(),
			})
			return
		}

		trace, _ := obs.TraceID(ctx)
		c.JSON(consts.StatusOK, map[string]any{
			"status":      consts.StatusOK,
			"user_id":     resp.GetUser().GetId(),
			"user_name":   resp.GetUser().GetName(),
			"orders":      len(resp.GetOrders()),
			"total_cents": total(resp.GetOrders()),
			"trace":       trace,
			"attempts":    1,
		})
	}
}

func total(orders []*commerce.Order) int64 {
	var sum int64
	for _, order := range orders {
		sum += order.GetTotalCents()
	}
	return sum
}

func statusFromError(err error) int {
	switch {
	case errors.Is(err, errcode.ErrInvalidInput):
		return consts.StatusBadRequest
	case errors.Is(err, errcode.ErrNotFound):
		return consts.StatusNotFound
	case errors.Is(err, errcode.ErrUpstream):
		return consts.StatusBadGateway
	default:
		return consts.StatusInternalServerError
	}
}
