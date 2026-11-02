package chain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Runner cascades shutdown across started servers.
type Runner struct {
	logger  *slog.Logger
	servers []namedServer
}

type namedServer struct {
	name string
	srv  Server
}

// NewRunner constructs an empty Runner.
func NewRunner(logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{logger: logger}
}

// Add appends srv to the startup order. Shutdown runs in reverse order.
func (r *Runner) Add(name string, srv Server) {
	if srv == nil {
		return
	}
	r.servers = append(r.servers, namedServer{name: name, srv: srv})
}

// Len returns the number of registered servers.
func (r *Runner) Len() int {
	return len(r.servers)
}

// Shutdown closes all registered servers in reverse order and joins errors.
func (r *Runner) Shutdown(ctx context.Context) error {
	var joined error
	for i := len(r.servers) - 1; i >= 0; i-- {
		entry := r.servers[i]
		r.logger.LogAttrs(ctx, slog.LevelInfo, "shutdown_server",
			slog.String("name", entry.name),
			slog.String("addr", entry.srv.Addr()),
		)
		if err := entry.srv.Shutdown(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("%s shutdown: %w", entry.name, err))
		}
	}
	return joined
}
