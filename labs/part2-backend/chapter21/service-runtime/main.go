package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidConfig = errors.New("invalid config")
	ErrNilService    = errors.New("nil checkout service")
)

type Config struct {
	ServiceName     string
	Environment     string
	HTTPAddr        string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

type CheckoutRequest struct {
	OrderID    string
	CustomerID string
}

type OrderSummary struct {
	OrderID string
	Status  string
}

type CheckoutService interface {
	PlaceOrder(context.Context, CheckoutRequest) (OrderSummary, error)
}

type shutdownHook func(context.Context) error

type App struct {
	cfg     Config
	service CheckoutService
	logger  *slog.Logger
	mux     *http.ServeMux

	ready atomic.Bool

	hooksMu sync.Mutex
	hooks   []shutdownHook
}

type probePayload struct {
	Status      string `json:"status"`
	Service     string `json:"service"`
	Environment string `json:"environment"`
}

func DefaultConfig() Config {
	return Config{
		ServiceName:     "checkout-runtime",
		Environment:     "dev",
		HTTPAddr:        ":8080",
		RequestTimeout:  2 * time.Second,
		ShutdownTimeout: 3 * time.Second,
	}
}

func LoadConfig(env map[string]string) (Config, error) {
	cfg := DefaultConfig()

	if value := strings.TrimSpace(env["SERVICE_NAME"]); value != "" {
		cfg.ServiceName = value
	}
	if value := strings.TrimSpace(env["APP_ENV"]); value != "" {
		cfg.Environment = value
	}
	if value := strings.TrimSpace(env["HTTP_ADDR"]); value != "" {
		cfg.HTTPAddr = value
	}
	if value := strings.TrimSpace(env["REQUEST_TIMEOUT"]); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("%w: parse REQUEST_TIMEOUT: %v", ErrInvalidConfig, err)
		}
		cfg.RequestTimeout = parsed
	}
	if value := strings.TrimSpace(env["SHUTDOWN_TIMEOUT"]); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("%w: parse SHUTDOWN_TIMEOUT: %v", ErrInvalidConfig, err)
		}
		cfg.ShutdownTimeout = parsed
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.ServiceName) == "":
		return fmt.Errorf("%w: service name is required", ErrInvalidConfig)
	case strings.TrimSpace(c.Environment) == "":
		return fmt.Errorf("%w: environment is required", ErrInvalidConfig)
	case strings.TrimSpace(c.HTTPAddr) == "":
		return fmt.Errorf("%w: http address is required", ErrInvalidConfig)
	case c.RequestTimeout <= 0:
		return fmt.Errorf("%w: request timeout must be positive", ErrInvalidConfig)
	case c.ShutdownTimeout <= 0:
		return fmt.Errorf("%w: shutdown timeout must be positive", ErrInvalidConfig)
	default:
		return nil
	}
}

func NewApp(cfg Config, logger *slog.Logger, service CheckoutService) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if service == nil {
		return nil, ErrNilService
	}
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	app := &App{
		cfg:     cfg,
		service: service,
		logger:  logger,
		mux:     http.NewServeMux(),
	}
	app.routes()
	return app, nil
}

func (a *App) routes() {
	a.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		a.writeProbe(w, http.StatusOK, probePayload{
			Status:      "ok",
			Service:     a.cfg.ServiceName,
			Environment: a.cfg.Environment,
		})
	})
	a.mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		statusCode := http.StatusServiceUnavailable
		status := "starting"
		if a.Ready() {
			statusCode = http.StatusOK
			status = "ready"
		}
		a.writeProbe(w, statusCode, probePayload{
			Status:      status,
			Service:     a.cfg.ServiceName,
			Environment: a.cfg.Environment,
		})
	})
}

func (a *App) Handler() http.Handler {
	return a.mux
}

func (a *App) SetReady(ready bool) {
	a.ready.Store(ready)
}

func (a *App) Ready() bool {
	return a.ready.Load()
}

func (a *App) AddShutdownHook(hook func(context.Context) error) {
	if hook == nil {
		return
	}
	a.hooksMu.Lock()
	defer a.hooksMu.Unlock()
	a.hooks = append(a.hooks, hook)
}

func (a *App) PlaceOrder(ctx context.Context, req CheckoutRequest) (OrderSummary, error) {
	requestCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()

	a.logger.InfoContext(requestCtx, "checkout_started",
		slog.String("service", a.cfg.ServiceName),
		slog.String("order_id", req.OrderID),
		slog.String("customer_id", req.CustomerID),
	)

	summary, err := a.service.PlaceOrder(requestCtx, req)
	if err != nil {
		a.logger.ErrorContext(requestCtx, "checkout_failed",
			slog.String("service", a.cfg.ServiceName),
			slog.String("order_id", req.OrderID),
			slog.String("error", err.Error()),
		)
		return OrderSummary{}, err
	}

	a.logger.InfoContext(requestCtx, "checkout_completed",
		slog.String("service", a.cfg.ServiceName),
		slog.String("order_id", summary.OrderID),
		slog.String("status", summary.Status),
	)
	return summary, nil
}

func (a *App) Shutdown(ctx context.Context) error {
	a.SetReady(false)

	shutdownCtx, cancel := context.WithTimeout(ctx, a.cfg.ShutdownTimeout)
	defer cancel()

	a.hooksMu.Lock()
	hooks := append([]shutdownHook(nil), a.hooks...)
	a.hooksMu.Unlock()

	for _, hook := range hooks {
		if err := hook(shutdownCtx); err != nil {
			a.logger.ErrorContext(shutdownCtx, "shutdown_failed",
				slog.String("service", a.cfg.ServiceName),
				slog.String("error", err.Error()),
			)
			return err
		}
	}

	a.logger.InfoContext(shutdownCtx, "shutdown_complete",
		slog.String("service", a.cfg.ServiceName),
		slog.Duration("timeout", a.cfg.ShutdownTimeout),
	)
	return nil
}

func (a *App) writeProbe(w http.ResponseWriter, statusCode int, payload probePayload) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		a.logger.Error("write_probe_failed", slog.String("error", err.Error()))
	}
}

type demoCheckoutService struct{}

func (demoCheckoutService) PlaceOrder(ctx context.Context, req CheckoutRequest) (OrderSummary, error) {
	select {
	case <-ctx.Done():
		return OrderSummary{}, ctx.Err()
	case <-time.After(20 * time.Millisecond):
		return OrderSummary{OrderID: req.OrderID, Status: "created"}, nil
	}
}

func currentEnv() map[string]string {
	env := make(map[string]string)
	for _, pair := range os.Environ() {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		env[key] = value
	}
	return env
}

func main() {
	cfg, err := LoadConfig(currentEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	app, err := NewApp(cfg, nil, demoCheckoutService{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "new app: %v\n", err)
		os.Exit(1)
	}

	app.SetReady(true)
	summary, err := app.PlaceOrder(context.Background(), CheckoutRequest{
		OrderID:    "ORD-2001",
		CustomerID: "CUST-7",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "place order: %v\n", err)
		os.Exit(1)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf(
		"service=%s env=%s order=%s status=%s request_timeout=%s shutdown=graceful\n",
		cfg.ServiceName,
		cfg.Environment,
		summary.OrderID,
		summary.Status,
		cfg.RequestTimeout,
	)
}
