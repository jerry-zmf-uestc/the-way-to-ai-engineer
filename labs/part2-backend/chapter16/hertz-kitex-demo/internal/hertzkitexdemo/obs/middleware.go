package obs

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// TraceIDHeader 是 gateway 侧 HTTP 请求 / 响应里携带 trace-id 的 header 名。
// Kitex 客户端 middleware 会把同一值同步塞进 metainfo，透传到下游 RPC。
const TraceIDHeader = "X-Trace-ID"

// HertzInjectTraceID 是 gateway 入口第一个 middleware：
//   - 若 request header 已有 TraceIDHeader，则复用；
//   - 否则调 NewTraceID() 生成一个；
//   - 塞进 context，并回写 response header，方便 client 侧串联。
func HertzInjectTraceID() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		id := string(ctx.GetHeader(TraceIDHeader))
		if id == "" {
			id = NewTraceID()
		}
		ctx.Header(TraceIDHeader, id)
		ctx.Next(WithTraceID(c, id))
	}
}

// HertzRecover 是 panic 兜底 middleware：捕获 handler / 后续 middleware 抛出的
// panic，打一条 error 级别日志（含 stack + trace_id），并写 500 响应。
// 后续请求不受影响。
func HertzRecover(logger *slog.Logger) app.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c context.Context, ctx *app.RequestContext) {
		defer func() {
			if r := recover(); r != nil {
				trace, _ := TraceID(c)
				logger.LogAttrs(c, slog.LevelError, "panic recovered",
					slog.Any("cause", r),
					slog.String("stack", string(debug.Stack())),
					slog.String("trace_id", trace),
				)
				ctx.AbortWithMsg("internal server error", 500)
			}
		}()
		ctx.Next(c)
	}
}

// HertzLogging 打请求出入日志（msg="http_request"，字段：method / path /
// status / duration_ms / bytes / trace_id）；不打敏感 header（Authorization / Cookie）。
func HertzLogging(logger *slog.Logger) app.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c context.Context, ctx *app.RequestContext) {
		start := time.Now()
		ctx.Next(c)
		trace, _ := TraceID(c)
		logger.LogAttrs(c, slog.LevelInfo, "http_request",
			slog.String("method", string(ctx.Method())),
			slog.String("path", string(ctx.Path())),
			slog.Int("status", ctx.Response.StatusCode()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.Int("bytes", len(ctx.Response.Body())),
			slog.String("trace_id", trace),
		)
	}
}

// KitexClientObserve 是 Kitex 客户端 middleware：
//   - 从 ctx 读 trace_id，塞进 metainfo.WithPersistentValue 透传到 server；
//   - 计时并打 msg="rpc_call"（字段：rpc_service / rpc_method / status /
//     duration_ms / attempts / trace_id）。attempts 目前占位 1，真正的
//     重试计数由 kitex retry 中间件叠加。
func KitexClientObserve(logger *slog.Logger) endpoint.Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			trace, _ := TraceID(ctx)
			if trace != "" {
				ctx = metainfo.WithPersistentValue(ctx, TraceIDHeader, trace)
			}

			start := time.Now()
			err := next(ctx, req, resp)
			service, method := rpcTarget(ctx)
			status := "ok"
			if err != nil {
				status = fmt.Sprintf("error: %v", err)
			}
			logger.LogAttrs(ctx, slog.LevelInfo, "rpc_call",
				slog.String("rpc_service", service),
				slog.String("rpc_method", method),
				slog.String("status", status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.Int("attempts", 1),
				slog.String("trace_id", trace),
			)
			return err
		}
	}
}

// KitexServerTraceFromMetainfo restores persistent trace metadata into ctx.
func KitexServerTraceFromMetainfo() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			if trace, ok := metainfo.GetPersistentValue(ctx, TraceIDHeader); ok && trace != "" {
				ctx = WithTraceID(ctx, trace)
			}
			return next(ctx, req, resp)
		}
	}
}

func rpcTarget(ctx context.Context) (service, method string) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil {
		return "", ""
	}
	if to := ri.To(); to != nil {
		service = to.ServiceName()
		method = to.Method()
	}
	if inv := ri.Invocation(); inv != nil {
		if service == "" {
			service = inv.ServiceName()
		}
		if method == "" {
			method = inv.MethodName()
		}
	}
	return service, method
}
