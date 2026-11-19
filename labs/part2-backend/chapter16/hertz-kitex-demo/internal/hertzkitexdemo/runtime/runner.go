package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

type namedServer struct {
	name   string
	server Server
}

type Runner struct {
	logger  *slog.Logger
	servers []namedServer
}

func NewRunner(logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{logger: logger}
}

func (r *Runner) Add(name string, server Server) {
	if server == nil {
		return
	}
	r.servers = append(r.servers, namedServer{name: name, server: server})
}

func (r *Runner) Len() int {
	return len(r.servers)
}

func (r *Runner) Shutdown(ctx context.Context) error {
	var joined error
	for i := len(r.servers) - 1; i >= 0; i-- {
		entry := r.servers[i]
		r.logger.LogAttrs(ctx, slog.LevelInfo, "shutdown_server",
			slog.String("name", entry.name),
			slog.String("addr", entry.server.Addr()),
		)
		if err := entry.server.Shutdown(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("%s shutdown: %w", entry.name, err))
		}
	}
	return joined
}
