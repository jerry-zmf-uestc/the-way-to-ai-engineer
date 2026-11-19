package order

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/cloudwego/kitex/client"
	kitexserver "github.com/cloudwego/kitex/server"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce/orderservice"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce/productservice"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/obs"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/rpc"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/runtime"
)

// Start launches OrderService on a loopback random port.
func Start(ctx context.Context, logger *slog.Logger, cfg config.Config, productAddr string) (runtime.Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	productClient, err := productservice.NewClient("product",
		client.WithHostPorts(productAddr),
		client.WithRPCTimeout(cfg.RPCTimeout),
		client.WithFailureRetry(rpc.FailureRetryPolicy(cfg.RPCMaxAttempts)),
		client.WithMiddleware(obs.KitexClientObserve(logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("new product client: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen order: %w", err)
	}
	srv := orderservice.NewServer(NewHandler(productClient),
		kitexserver.WithListener(ln),
		kitexserver.WithMiddleware(obs.KitexServerTraceFromMetainfo()),
	)
	return startKitexServer("order", ln, srv, logger), nil
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
