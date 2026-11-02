package obs

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

// newTestEngine 装配一个只带 obs middleware 的 Hertz Engine，方便 ut.PerformRequest。
func newTestEngine(mws ...app.HandlerFunc) *route.Engine {
	e := route.NewEngine(config.NewOptions(nil))
	for _, mw := range mws {
		e.Use(mw)
	}
	return e
}

// bufferLogger 建立一个把 JSON 落到 bytes.Buffer 的 slog.Logger，方便 test 断言字段。
func bufferLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return NewLogger(buf, slog.LevelDebug), buf
}

// decodeLastLog 把 buffer 里最后一条 JSON 日志解出，避免 handler / recover
// 打了多条时抓错行。
func decodeLastLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("no log lines captured")
	}
	last := lines[len(lines)-1]
	rec := map[string]any{}
	if err := json.Unmarshal([]byte(last), &rec); err != nil {
		t.Fatalf("last log %q not JSON: %v", last, err)
	}
	return rec
}

// TestHertzInjectTraceID_GeneratesWhenMissing 断言：没带 X-Trace-ID 时，
// middleware 生成一个 32 hex 的 id，写回 response header，且 downstream ctx 拿得到。
func TestHertzInjectTraceID_GeneratesWhenMissing(t *testing.T) {
	t.Parallel()

	var captured string
	e := newTestEngine(HertzInjectTraceID())
	e.GET("/echo", func(c context.Context, ctx *app.RequestContext) {
		if id, ok := TraceID(c); ok {
			captured = id
		}
		ctx.SetStatusCode(200)
	})

	w := ut.PerformRequest(e, "GET", "/echo", nil)
	respID := string(w.Header().Get(TraceIDHeader))

	if respID == "" {
		t.Fatalf("response missing %s header", TraceIDHeader)
	}
	if len(respID) != 32 {
		t.Errorf("trace id len = %d, want 32", len(respID))
	}
	if _, err := hex.DecodeString(respID); err != nil {
		t.Errorf("trace id %q not hex: %v", respID, err)
	}
	if captured != respID {
		t.Errorf("ctx trace id = %q, response header = %q; expected equal", captured, respID)
	}
}

// TestHertzInjectTraceID_ReusesInboundHeader 断言：request 带 X-Trace-ID 时复用它，
// 不重新生成。
func TestHertzInjectTraceID_ReusesInboundHeader(t *testing.T) {
	t.Parallel()

	const inbound = "abcdef0123456789abcdef0123456789"
	var captured string

	e := newTestEngine(HertzInjectTraceID())
	e.GET("/echo", func(c context.Context, ctx *app.RequestContext) {
		captured, _ = TraceID(c)
		ctx.SetStatusCode(200)
	})

	w := ut.PerformRequest(e, "GET", "/echo", nil, ut.Header{Key: TraceIDHeader, Value: inbound})

	if got := string(w.Header().Get(TraceIDHeader)); got != inbound {
		t.Errorf("response header trace id = %q, want %q", got, inbound)
	}
	if captured != inbound {
		t.Errorf("ctx trace id = %q, want %q", captured, inbound)
	}
}

// TestHertzLogging_Fields 断言 HertzLogging 打出的 JSON 里五字段齐全，且带 trace_id。
func TestHertzLogging_Fields(t *testing.T) {
	t.Parallel()

	logger, buf := bufferLogger()
	e := newTestEngine(HertzInjectTraceID(), HertzLogging(logger))
	e.GET("/echo", func(c context.Context, ctx *app.RequestContext) {
		ctx.SetStatusCode(201)
		ctx.String(201, "hello") // 5 bytes payload
	})

	ut.PerformRequest(e, "GET", "/echo", nil)

	rec := decodeLastLog(t, buf)
	if rec["msg"] != "http_request" {
		t.Errorf("msg = %v, want http_request", rec["msg"])
	}
	if rec["method"] != "GET" {
		t.Errorf("method = %v, want GET", rec["method"])
	}
	if rec["path"] != "/echo" {
		t.Errorf("path = %v, want /echo", rec["path"])
	}
	if v, ok := rec["status"].(float64); !ok || int(v) != 201 {
		t.Errorf("status = %v, want 201", rec["status"])
	}
	if _, ok := rec["duration_ms"].(float64); !ok {
		t.Errorf("missing/typed duration_ms: %v", rec["duration_ms"])
	}
	if v, ok := rec["bytes"].(float64); !ok || int(v) != 5 {
		t.Errorf("bytes = %v, want 5", rec["bytes"])
	}
	if s, ok := rec["trace_id"].(string); !ok || len(s) != 32 {
		t.Errorf("trace_id = %v, want 32-hex string", rec["trace_id"])
	}
}

// TestHertzRecover_Panic 断言：handler panic 时返回 500，日志含 panic + stack，
// 且**同一 engine**下一次请求依然是 200，证明 recover 没把 goroutine 打崩。
func TestHertzRecover_Panic(t *testing.T) {
	t.Parallel()

	logger, buf := bufferLogger()
	e := newTestEngine(HertzInjectTraceID(), HertzRecover(logger))
	e.GET("/boom", func(c context.Context, ctx *app.RequestContext) {
		panic("kaboom")
	})
	e.GET("/ok", func(c context.Context, ctx *app.RequestContext) {
		ctx.SetStatusCode(200)
	})

	w1 := ut.PerformRequest(e, "GET", "/boom", nil)
	if w1.Code != 500 {
		t.Errorf("after panic status = %d, want 500", w1.Code)
	}

	// 日志层面：至少一条 error 级别、message 含 "panic"，stack 字段存在。
	rec := decodeLastLog(t, buf)
	if rec["level"] != "ERROR" {
		t.Errorf("panic log level = %v, want ERROR", rec["level"])
	}
	if msg, _ := rec["msg"].(string); !strings.Contains(strings.ToLower(msg), "panic") {
		t.Errorf("panic log msg = %v, want contains 'panic'", rec["msg"])
	}
	if _, ok := rec["stack"]; !ok {
		t.Errorf("panic log missing stack field: %v", rec)
	}

	// 后续请求仍正常。
	w2 := ut.PerformRequest(e, "GET", "/ok", nil)
	if w2.Code != 200 {
		t.Errorf("subsequent request status = %d, want 200", w2.Code)
	}
}

// TestKitexClientObserve_TraceIDPropagation 断言 client middleware 会把
// ctx 里的 trace_id 通过 metainfo.WithPersistentValue 塞进 outbound ctx；
// 下游 endpoint 内部用 metainfo.GetPersistentValue 能取到。
func TestKitexClientObserve_TraceIDPropagation(t *testing.T) {
	t.Parallel()

	logger, _ := bufferLogger()
	const wantTrace = "propagate-me-1234567890abcdef01"

	var seenInEndpoint string
	inner := func(ctx context.Context, req, resp any) error {
		if v, ok := metainfo.GetPersistentValue(ctx, TraceIDHeader); ok {
			seenInEndpoint = v
		}
		return nil
	}

	mw := KitexClientObserve(logger)
	ctx := WithTraceID(context.Background(), wantTrace)
	if err := mw(inner)(ctx, struct{}{}, struct{}{}); err != nil {
		t.Fatalf("middleware endpoint returned err = %v", err)
	}
	if seenInEndpoint != wantTrace {
		t.Errorf("downstream metainfo trace = %q, want %q", seenInEndpoint, wantTrace)
	}
}

// TestKitexClientObserve_LogsRPCCall 断言中间件会打 msg="rpc_call"，
// 字段：duration_ms / status / trace_id / attempts。
func TestKitexClientObserve_LogsRPCCall(t *testing.T) {
	t.Parallel()

	logger, buf := bufferLogger()
	inner := func(ctx context.Context, req, resp any) error { return nil }

	mw := KitexClientObserve(logger)
	ctx := WithTraceID(context.Background(), "trace-log-1")
	if err := mw(inner)(ctx, struct{}{}, struct{}{}); err != nil {
		t.Fatalf("mw returned err = %v", err)
	}

	rec := decodeLastLog(t, buf)
	if rec["msg"] != "rpc_call" {
		t.Errorf("msg = %v, want rpc_call", rec["msg"])
	}
	if rec["trace_id"] != "trace-log-1" {
		t.Errorf("trace_id = %v, want trace-log-1", rec["trace_id"])
	}
	if _, ok := rec["duration_ms"].(float64); !ok {
		t.Errorf("missing/typed duration_ms: %v", rec["duration_ms"])
	}
	if v, ok := rec["attempts"].(float64); !ok || int(v) < 1 {
		t.Errorf("attempts = %v, want int >= 1", rec["attempts"])
	}
	if rec["status"] == nil {
		t.Errorf("missing status field: %v", rec)
	}
	if rec["rpc_service"] == nil {
		t.Errorf("missing rpc_service field: %v", rec)
	}
	if rec["rpc_method"] == nil {
		t.Errorf("missing rpc_method field: %v", rec)
	}
}

// TestKitexClientObserve_ErrorStatus 断言：inner endpoint 报错时，
// 日志 status 标为非 OK（"error" / err.Error()），且中间件本身把错误原样返回。
func TestKitexClientObserve_ErrorStatus(t *testing.T) {
	t.Parallel()

	logger, buf := bufferLogger()
	sentinel := errors.New("upstream unavailable")
	inner := func(ctx context.Context, req, resp any) error { return sentinel }

	mw := KitexClientObserve(logger)
	err := mw(inner)(WithTraceID(context.Background(), "trace-err"), struct{}{}, struct{}{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("mw swallowed err; got %v, want %v", err, sentinel)
	}

	rec := decodeLastLog(t, buf)
	status, _ := rec["status"].(string)
	if status == "" || status == "ok" || status == "OK" {
		t.Errorf("error status = %q, want non-ok string", status)
	}
}

// TestKitexServerTraceFromMetainfo 断言 server middleware 会把收到的
// persistent metainfo trace 重新放入普通 context，供下游 client middleware 读取。
func TestKitexServerTraceFromMetainfo(t *testing.T) {
	t.Parallel()

	const wantTrace = "server-side-trace"
	var seen string
	inner := func(ctx context.Context, req, resp any) error {
		seen, _ = TraceID(ctx)
		return nil
	}
	ctx := metainfo.WithPersistentValue(context.Background(), TraceIDHeader, wantTrace)

	if err := KitexServerTraceFromMetainfo()(inner)(ctx, struct{}{}, struct{}{}); err != nil {
		t.Fatalf("middleware endpoint returned err = %v", err)
	}
	if seen != wantTrace {
		t.Fatalf("TraceID(ctx) = %q, want %q", seen, wantTrace)
	}
}
