package product

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	kitexserver "github.com/cloudwego/kitex/server"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/chain"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/obs"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce/productservice"
)

// Start launches ProductService on a loopback random port.
func Start(ctx context.Context, logger *slog.Logger) (chain.Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen product: %w", err)
	}
	srv := productservice.NewServer(NewHandler(),
		kitexserver.WithListener(ln),
		kitexserver.WithMiddleware(obs.KitexServerTraceFromMetainfo()),
	)
	return startKitexServer("product", ln, srv, logger), nil
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
