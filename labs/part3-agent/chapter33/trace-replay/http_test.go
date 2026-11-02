package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestActualHTTPPropagatesSDKContext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter), sdktrace.WithResource(resource.Empty()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	tracer := provider.Tracer("course.http")
	seen := make(chan trace.SpanContext, 2)
	server := httptest.NewServer(TraceHTTP(tracer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- trace.SpanFromContext(r.Context()).SpanContext()
		w.WriteHeader(http.StatusNoContent)
	})))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	callCtx, clientSpan := tracer.Start(ctx, "http.request", trace.WithSpanKind(trace.SpanKindClient))
	defer clientSpan.End()
	request, err := http.NewRequestWithContext(callCtx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	propagation.TraceContext{}.Inject(callCtx, propagation.HeaderCarrier(request.Header))
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal(readErr, closeErr)
	}
	clientSpan.End()
	if response.StatusCode != http.StatusNoContent {
		t.Fatal(response.StatusCode)
	}
	remote := <-seen
	if remote.TraceID() != clientSpan.SpanContext().TraceID() {
		t.Fatal("HTTP header context lost")
	}
	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatal(spans)
	}
	foundServer := false
	for _, s := range spans {
		if s.Name == "http.lookup" {
			foundServer = true
			if s.Parent.SpanID() != clientSpan.SpanContext().SpanID() || !s.Parent.IsRemote() || s.SpanKind != trace.SpanKindServer {
				t.Fatal("remote parent not preserved", s)
			}
		}
		if len(s.Attributes) != 0 || len(s.Events) != 0 {
			t.Fatal("HTTP details leaked", s)
		}
	}
	if !foundServer {
		t.Fatal("server span missing")
	}
	// Invalid observation headers are not business authorization and do not prevent the handler.
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("traceparent", "invalid")
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	fresh := <-seen
	if !fresh.IsValid() || fresh.TraceID() == remote.TraceID() || response.StatusCode != http.StatusNoContent {
		t.Fatal("invalid header changed business handling")
	}
}
