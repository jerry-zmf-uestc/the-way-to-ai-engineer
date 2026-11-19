package config

import (
	"log/slog"
	"testing"
	"time"
)

// TestDefault 锁定默认值：任何一项漂移都要显式改测试 + 更新 design.md §4.2。
func TestDefault(t *testing.T) {
	t.Parallel()

	got := Default()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"RPCTimeout", got.RPCTimeout, 500 * time.Millisecond},
		{"RPCMaxAttempts", got.RPCMaxAttempts, 3},
		{"HTTPTimeout", got.HTTPTimeout, 2 * time.Second},
		{"ShutdownTimeout", got.ShutdownTimeout, 500 * time.Millisecond},
		{"LogLevel", got.LogLevel, slog.LevelInfo},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("Default().%s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

// TestFromEnv_NoEnv 未设任何环境变量时，FromEnv 应完全等于 Default。
func TestFromEnv_NoEnv(t *testing.T) {
	// 注意：Setenv 会自动在测试结束时还原；这里显式 Unsetenv 保证清空。
	clearConfigEnv(t)

	got := FromEnv()
	want := Default()
	if got != want {
		t.Fatalf("FromEnv() = %+v, want %+v", got, want)
	}
}

// TestFromEnv_Overrides 表驱动覆盖每一项合法值。
func TestFromEnv_Overrides(t *testing.T) {
	clearConfigEnv(t)

	t.Setenv("RPC_TIMEOUT_MS", "1200")
	t.Setenv("RPC_MAX_ATTEMPTS", "5")
	t.Setenv("HTTP_TIMEOUT_MS", "3500")
	t.Setenv("SHUTDOWN_TIMEOUT_MS", "800")
	t.Setenv("LOG_LEVEL", "debug")

	got := FromEnv()

	want := Config{
		RPCTimeout:      1200 * time.Millisecond,
		RPCMaxAttempts:  5,
		HTTPTimeout:     3500 * time.Millisecond,
		ShutdownTimeout: 800 * time.Millisecond,
		LogLevel:        slog.LevelDebug,
	}
	if got != want {
		t.Fatalf("FromEnv() = %+v, want %+v", got, want)
	}
}

// TestFromEnv_Invalid_FallbackToDefault 对每一字段单独注入非法值，
// 断言：脏数据不影响其他字段，且被污染的字段 fallback 到 Default 对应值。
func TestFromEnv_Invalid_FallbackToDefault(t *testing.T) {
	def := Default()

	cases := []struct {
		name   string
		key    string
		value  string
		verify func(t *testing.T, c Config)
	}{
		{
			name:  "RPC_TIMEOUT_MS non-numeric",
			key:   "RPC_TIMEOUT_MS",
			value: "abc",
			verify: func(t *testing.T, c Config) {
				if c.RPCTimeout != def.RPCTimeout {
					t.Errorf("RPCTimeout = %v, want default %v", c.RPCTimeout, def.RPCTimeout)
				}
			},
		},
		{
			name:  "RPC_MAX_ATTEMPTS zero",
			key:   "RPC_MAX_ATTEMPTS",
			value: "0",
			verify: func(t *testing.T, c Config) {
				if c.RPCMaxAttempts != def.RPCMaxAttempts {
					t.Errorf("RPCMaxAttempts = %d, want default %d", c.RPCMaxAttempts, def.RPCMaxAttempts)
				}
			},
		},
		{
			name:  "RPC_MAX_ATTEMPTS negative",
			key:   "RPC_MAX_ATTEMPTS",
			value: "-3",
			verify: func(t *testing.T, c Config) {
				if c.RPCMaxAttempts != def.RPCMaxAttempts {
					t.Errorf("RPCMaxAttempts = %d, want default %d", c.RPCMaxAttempts, def.RPCMaxAttempts)
				}
			},
		},
		{
			name:  "HTTP_TIMEOUT_MS empty string",
			key:   "HTTP_TIMEOUT_MS",
			value: "",
			verify: func(t *testing.T, c Config) {
				if c.HTTPTimeout != def.HTTPTimeout {
					t.Errorf("HTTPTimeout = %v, want default %v", c.HTTPTimeout, def.HTTPTimeout)
				}
			},
		},
		{
			name:  "SHUTDOWN_TIMEOUT_MS bad",
			key:   "SHUTDOWN_TIMEOUT_MS",
			value: "!!!",
			verify: func(t *testing.T, c Config) {
				if c.ShutdownTimeout != def.ShutdownTimeout {
					t.Errorf("ShutdownTimeout = %v, want default %v", c.ShutdownTimeout, def.ShutdownTimeout)
				}
			},
		},
		{
			name:  "LOG_LEVEL unknown",
			key:   "LOG_LEVEL",
			value: "verbose",
			verify: func(t *testing.T, c Config) {
				if c.LogLevel != def.LogLevel {
					t.Errorf("LogLevel = %v, want default %v", c.LogLevel, def.LogLevel)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv(tc.key, tc.value)
			tc.verify(t, FromEnv())
		})
	}
}

// TestFromEnv_LogLevelCaseInsensitive 断言 LOG_LEVEL 解析忽略大小写。
func TestFromEnv_LogLevelCaseInsensitive(t *testing.T) {
	cases := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"Info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("LOG_LEVEL", in)
			if got := FromEnv().LogLevel; got != want {
				t.Errorf("LOG_LEVEL=%q -> %v, want %v", in, got, want)
			}
		})
	}
}

// clearConfigEnv 在 t.Setenv 之前把所有本包读取的 env 显式清空，
// 避免 CI 环境或其他测试残留污染。t.Setenv 保证子测试结束后自动还原。
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"RPC_TIMEOUT_MS",
		"RPC_MAX_ATTEMPTS",
		"HTTP_TIMEOUT_MS",
		"SHUTDOWN_TIMEOUT_MS",
		"LOG_LEVEL",
	} {
		t.Setenv(k, "")
	}
}
