package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 汇聚 hertz-kitex-real-chain demo 里所有可外部化的参数：
// Kitex RPC 超时 / 重试次数、Hertz client 超时、级联 Shutdown 预算、
// slog level。所有字段都是值类型，方便拷贝到 goroutine 里读。
type Config struct {
	RPCTimeout      time.Duration
	RPCMaxAttempts  int
	HTTPTimeout     time.Duration
	ShutdownTimeout time.Duration
	LogLevel        slog.Level
}

// Default 返回默认 Config，供 main / 单测直接使用；所有默认值遵循
// requirements.md §Quality Attributes 的硬约束（bounded demo，总预算 8s）。
func Default() Config {
	return Config{
		RPCTimeout:      500 * time.Millisecond,
		RPCMaxAttempts:  3,
		HTTPTimeout:     2 * time.Second,
		ShutdownTimeout: 500 * time.Millisecond,
		LogLevel:        slog.LevelInfo,
	}
}

// FromEnv 在 Default 之上叠加环境变量覆盖：
//
//	RPC_TIMEOUT_MS        int  -> RPCTimeout
//	RPC_MAX_ATTEMPTS      int  -> RPCMaxAttempts     (>=1，否则 fallback default)
//	HTTP_TIMEOUT_MS       int  -> HTTPTimeout
//	SHUTDOWN_TIMEOUT_MS   int  -> ShutdownTimeout
//	LOG_LEVEL             str  -> LogLevel           (debug/info/warn/error, 大小写不敏感)
//
// 任何字段解析失败一律 fallback 到 Default 对应字段，不 panic、不返回 error；
// 保证 demo 在环境变量脏数据下依然可跑。
func FromEnv() Config {
	c := Default()
	c.RPCTimeout = envDuration("RPC_TIMEOUT_MS", c.RPCTimeout)
	c.RPCMaxAttempts = envPositiveInt("RPC_MAX_ATTEMPTS", c.RPCMaxAttempts)
	c.HTTPTimeout = envDuration("HTTP_TIMEOUT_MS", c.HTTPTimeout)
	c.ShutdownTimeout = envDuration("SHUTDOWN_TIMEOUT_MS", c.ShutdownTimeout)
	c.LogLevel = envLogLevel("LOG_LEVEL", c.LogLevel)
	return c
}

func envDuration(key string, def time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms <= 0 {
		return def
	}
	return time.Duration(ms) * time.Millisecond
}

func envPositiveInt(key string, def int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func envLogLevel(key string, def slog.Level) slog.Level {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return def
	}
}
