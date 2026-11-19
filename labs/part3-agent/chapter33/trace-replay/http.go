package main

import (
	"net/http"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceHTTP只传播观测关联；业务handler仍负责身份、授权和结果判断。
func TraceHTTP(tracer trace.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parent := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(parent, "http.lookup", trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
