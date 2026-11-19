package user

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/cloudwego/kitex/client"
	kitexserver "github.com/cloudwego/kitex/server"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/chain"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/obs"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/rpc"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce/orderservice"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce/userservice"
)

// Start launches UserService on a loopback random port.
func Start(ctx context.Context, logger *slog.Logger, cfg config.Config, orderAddr string) (chain.Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	orderClient, err := orderservice.NewClient("order",
		client.WithHostPorts(orderAddr),
		client.WithRPCTimeout(cfg.RPCTimeout),
		client.WithFailureRetry(rpc.FailureRetryPolicy(cfg.RPCMaxAttempts)),
		client.WithMiddleware(obs.KitexClientObserve(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("new order client: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen user: %w", err)
	}
	srv := userservice.NewServer(NewHandler(orderClient),
		kitexserver.WithListener(ln),
		kitexserver.WithMiddleware(obs.KitexServerTraceFromMetainfo()),
	)
	return startKitexServer("user", ln, srv, logger), nil
}

type kitexServer struct {
	name   string
	addr   string
	server kitexserver.Server
	done   chan error
	logger *slog.Logger
}

func startKitexServer(name string, ln net.Listener, srv kitexserver.Server, logger *slog.Logger) *kitexServer {
	if logger == nil {
		logger = slog.Default()
	}
	ks := &kitexServer{
		name:   name,
		addr:   ln.Addr().String(),
		server: srv,
		done:   make(chan error, 1),
		logger: logger,
	}
	go func() {
		ks.done <- srv.Run()
	}()
	return ks
}

// Addr returns the bound listener address.
func (s *kitexServer) Addr() string { return s.addr }

// Shutdown stops the Kitex server and waits for Run to return.
func (s *kitexServer) Shutdown(ctx context.Context) error {
	if err := s.server.Stop(); err != nil {
		return fmt.Errorf("stop %s: %w", s.name, err)
	}
	select {
	case err := <-s.done:
		if err != nil {
			return fmt.Errorf("run %s: %w", s.name, err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
