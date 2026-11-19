package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/codes"
)

func TestSDKSpansKeepRealParentAndRemoteContext(t *testing.T) {
	spans, err := SDKSpans(context.Background(), func(context.Context) (string, error) { return "", ErrTool })
	if err != nil || len(spans) != 4 {
		t.Fatal(spans, err)
	}
	ids := map[string]int{}
	for n, s := range spans {
		ids[s.Name] = n
		if !s.SpanContext.IsValid() {
			t.Fatal("SDK identity absent", s)
		}
	}
	run, model, tool, remote := spans[ids["agent.run"]], spans[ids["model.decide"]], spans[ids["tool.read"]], spans[ids["server.lookup"]]
	if run.Parent.IsValid() || model.Parent.SpanID() != run.SpanContext.SpanID() || tool.Parent.SpanID() != run.SpanContext.SpanID() || remote.Parent.SpanID() != tool.SpanContext.SpanID() || !remote.Parent.IsRemote() || remote.SpanContext.TraceID() != run.SpanContext.TraceID() {
		t.Fatal("causal context lost", spans)
	}
	if model.Status.Code == codes.Error || tool.Status.Code != codes.Error || remote.Status.Code != codes.Error || run.Status.Code != codes.Error {
		t.Fatal("component status confused with business failure", spans)
	}
}

func TestSDKProjectionDoesNotContainPrivatePayloads(t *testing.T) {
	spans, err := SDKSpans(context.Background(), func(context.Context) (string, error) {
		return "private-prompt-canary", errors.New("private-error-canary")
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(spans)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-prompt-canary") || strings.Contains(string(data), "private-error-canary") {
		t.Fatal("payload leaked")
	}
	for _, s := range spans {
		for _, a := range s.Attributes {
			if a.Key != "app.run.id" && a.Key != "app.call.id" && a.Key != "app.outcome" {
				t.Fatal("attribute outside allowlist", a)
			}
		}
		if len(s.Events) != 0 {
			t.Fatal("raw exception event recorded", s.Events)
		}
	}
}
