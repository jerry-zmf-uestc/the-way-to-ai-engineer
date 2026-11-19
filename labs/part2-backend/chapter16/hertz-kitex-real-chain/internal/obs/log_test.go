package obs

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"
)

// TestNewLogger_JSONOutput 断言 NewLogger 返回 slog JSON handler，
// 且写入的记录能被 json.Unmarshal 解出 level / msg / 业务字段。
func TestNewLogger_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewLogger(&buf, slog.LevelInfo)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	logger.Info("http_request", slog.String("path", "/healthz"), slog.Int("status", 200))

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatalf("logger produced no output")
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("output %q is not valid JSON: %v", line, err)
	}
	if rec["msg"] != "http_request" {
		t.Errorf("msg = %v, want http_request", rec["msg"])
	}
	if rec["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", rec["level"])
	}
	if rec["path"] != "/healthz" {
		t.Errorf("path = %v, want /healthz", rec["path"])
	}
	// slog 里数字默认解出来是 float64。
	if v, ok := rec["status"].(float64); !ok || int(v) != 200 {
		t.Errorf("status = %v, want 200", rec["status"])
	}
}

// TestNewLogger_LevelFilter 断言 level 参数按 slog 语义生效：
// 传 LevelWarn 时 Info 记录应被丢弃。
func TestNewLogger_LevelFilter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewLogger(&buf, slog.LevelWarn)

	logger.Info("should be dropped")
	logger.Warn("should be kept")

	out := buf.String()
	if strings.Contains(out, "should be dropped") {
		t.Errorf("Info log leaked while level=Warn:\n%s", out)
	}
	if !strings.Contains(out, "should be kept") {
		t.Errorf("Warn log missing:\n%s", out)
	}
}

// TestNewTraceID_LengthAndCharset 断言 trace-id 长度 = 32 且是合法 hex。
func TestNewTraceID_LengthAndCharset(t *testing.T) {
	t.Parallel()

	id := NewTraceID()
	if len(id) != 32 {
		t.Fatalf("len(NewTraceID()) = %d, want 32", len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("NewTraceID() = %q, not hex: %v", id, err)
	}
	// 冗余守卫：确认不是全 0（极小概率但值得校验实现没写死）。
	if id == strings.Repeat("0", 32) {
		t.Fatalf("NewTraceID() unexpectedly returned all-zero id")
	}
	// 确认字符集只在 0-9a-f。
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
		t.Fatalf("NewTraceID() = %q, charset out of [0-9a-f]", id)
	}
}

// TestNewTraceID_Uniqueness 快速抽样 1000 次，全部互异；概率角度可视为确定性。
func TestNewTraceID_Uniqueness(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 1000)
	for i := range 1000 {
		id := NewTraceID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate trace id at i=%d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}

// TestWithTraceID_Roundtrip 断言 With/Get 可对称还原。
func TestWithTraceID_Roundtrip(t *testing.T) {
	t.Parallel()

	ctx := WithTraceID(context.Background(), "trace-abc")
	got, ok := TraceID(ctx)
	if !ok {
		t.Fatal("TraceID reported not set after WithTraceID")
	}
	if got != "trace-abc" {
		t.Errorf("TraceID = %q, want trace-abc", got)
	}
}

// TestTraceID_Missing 断言未 With 过的 context 返回 (\"\", false)。
func TestTraceID_Missing(t *testing.T) {
	t.Parallel()

	got, ok := TraceID(context.Background())
	if ok {
		t.Errorf("TraceID on empty ctx: ok=true, want false")
	}
	if got != "" {
		t.Errorf("TraceID on empty ctx: got=%q, want empty", got)
	}
}

// TestWithTraceID_Empty 断言空字符串按未设置对待，避免 downstream 打出空 trace_id。
func TestWithTraceID_Empty(t *testing.T) {
	t.Parallel()

	ctx := WithTraceID(context.Background(), "")
	got, ok := TraceID(ctx)
	if ok || got != "" {
		t.Errorf("TraceID after With(\"\") = (%q, %v), want (\"\", false)", got, ok)
	}
}
