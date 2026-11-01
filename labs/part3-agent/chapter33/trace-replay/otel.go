package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// SDKSpans使用真实SDK和内存Exporter，不连接采集平台或注册全局Provider。
func SDKSpans(ctx context.Context, live func(context.Context) (string, error)) (spans tracetest.SpanStubs, err error) {
	if live == nil {
		return nil, ErrTrace
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter), sdktrace.WithResource(resource.Empty()))
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if stop := provider.Shutdown(shutdown); err == nil {
			err = stop
		}
	}()
	tracer := provider.Tracer("course.trace")
	runCtx, run := tracer.Start(ctx, "agent.run", trace.WithAttributes(attribute.String("app.run.id", "demo-run")))
	_, model := tracer.Start(runCtx, "model.decide", trace.WithAttributes(attribute.String("app.call.id", "model-1")))
	model.End()
	toolCtx, tool := tracer.Start(runCtx, "tool.read", trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attribute.String("app.call.id", "tool-1")))
	carrier := propagation.MapCarrier{}
	propagator := propagation.TraceContext{}
	propagator.Inject(toolCtx, carrier)
	remoteCtx := propagator.Extract(context.Background(), carrier)
	callCtx, remote := tracer.Start(remoteCtx, "server.lookup", trace.WithSpanKind(trace.SpanKindServer))
	_, callErr := live(callCtx)
	code := classify("tool", callErr)
	if code != "" {
		remote.SetStatus(codes.Error, code)
	}
	remote.End()
	if code != "" {
		tool.SetStatus(codes.Error, code)
	}
	tool.End()
	if code != "" {
		run.SetAttributes(attribute.String("app.outcome", "task_failed"))
		run.SetStatus(codes.Error, "task_failed")
	}
	run.End()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return exporter.GetSpans(), nil
}

func otelDemo() {
	spans, err := SDKSpans(context.Background(), func(context.Context) (string, error) { return "", ErrTool })
	if err != nil {
		panic(err)
	}
	fmt.Printf("sdk_spans=%d exporter=in_memory propagation=trace_context network=false\n", len(spans))
}
