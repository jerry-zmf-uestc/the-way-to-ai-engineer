package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func syntheticObservedDriver(ctx context.Context, kind, input string) (ObservedReply, error) {
	text, err := decision(ctx, "baseline", kind, input)
	m := CallMetrics{Origin: "application"}
	if kind == "tool-read" {
		m.Origin = "tool"
	} else if kind == "model-final" {
		m = CallMetrics{Origin: "synthetic-model", ModelRevision: "synthetic-v1", UsageSource: "synthetic", UsageKnown: true, InputTokens: 20, OutputTokens: 5}
	}
	return ObservedReply{text, m}, err
}

func TestObservedArchiveSeparatesSyntheticAndActualUsage(t *testing.T) {
	a, err := recordObserved(context.Background(), "baseline", syntheticObservedDriver)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := load(strings.NewReader(encoded(a)))
	if err != nil {
		t.Fatal(err)
	}
	r, err := evaluate(context.Background(), loaded)
	if err != nil || !r.Gate || r.CallUsage == nil || r.CallUsage.ApplicationCalls != 4 || r.CallUsage.ToolCalls != 4 || r.CallUsage.SyntheticCalls != 4 || r.CallUsage.SyntheticInputTokens != 80 || r.CallUsage.SyntheticOutputTokens != 20 || r.CallUsage.ModelCalls != 0 || r.UsageKnown || r.CostKnown {
		t.Fatal(r, err)
	}
	loaded.Schema = 2
	if _, err := evaluate(context.Background(), loaded); !errors.Is(err, ErrRecord) {
		t.Fatal("legacy accepted metrics", err)
	}
}

func TestObservedFailureRetainsKnownAndUnknownCounters(t *testing.T) {
	for _, known := range []bool{false, true} {
		a, err := recordObserved(context.Background(), "baseline", func(ctx context.Context, kind, input string) (ObservedReply, error) {
			reply, err := syntheticObservedDriver(ctx, kind, input)
			if kind == "model-final" {
				reply.Output = "discarded output"
				if !known {
					reply.Metrics.UsageKnown = false
					reply.Metrics.UsageSource = ""
					reply.Metrics.InputTokens = 0
					reply.Metrics.OutputTokens = 0
				}
				err = ErrRecord
			}
			return reply, err
		})
		if err != nil {
			t.Fatal(err)
		}
		r, err := evaluate(context.Background(), a)
		if err != nil || r.ExecutionFailed != 4 || r.Gate || r.CallUsage.SyntheticCalls != 4 || a.Runs[0].Calls[2].Output != "" {
			t.Fatal(r, err)
		}
		if known && r.CallUsage.SyntheticOutputTokens != 20 || !known && (r.CallUsage.UnknownSyntheticUsage != 4 || r.CallUsage.SyntheticOutputTokens != 0) {
			t.Fatal(r.CallUsage)
		}
	}
}

func TestObservedMetadataMismatchRejectsWithoutPartialReport(t *testing.T) {
	for _, edit := range []func(*CallMetrics){
		func(m *CallMetrics) { m.Origin = "tool" }, func(m *CallMetrics) { m.ModelRevision = "" }, func(m *CallMetrics) { m.UsageKnown = false }, func(m *CallMetrics) { m.InputTokens = -1 }, func(m *CallMetrics) { m.UsageSource = "provider-reported" },
	} {
		a, _ := recordObserved(context.Background(), "baseline", syntheticObservedDriver)
		edit(a.Runs[0].Calls[2].Metrics)
		if r, err := evaluate(context.Background(), a); !errors.Is(err, ErrRecord) || r.Total != 0 {
			t.Fatal(r, err)
		}
	}
	if a, err := recordObserved(context.Background(), "baseline", func(context.Context, string, string) (ObservedReply, error) { return ObservedReply{}, nil }); !errors.Is(err, ErrRecord) || len(a.Runs) != 0 {
		t.Fatal(a, err)
	}
}

func TestObservedLateCancellationRetainsUsageWithoutAnswer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := recordObserved(ctx, "baseline", func(ctx context.Context, kind, input string) (ObservedReply, error) {
		r, err := syntheticObservedDriver(ctx, kind, input)
		if kind == "model-final" {
			cancel()
		}
		return r, err
	})
	if err != nil || a.Runs[0].Answer != (Answer{}) || a.Runs[0].Calls[2].Output != "" {
		t.Fatal("late answer committed", a, err)
	}
	r, err := evaluate(context.Background(), a)
	if err != nil || r.ExecutionFailed != 4 || r.CallUsage.SyntheticCalls != 1 || r.CallUsage.SyntheticInputTokens != 20 || r.CallUsage.SyntheticOutputTokens != 5 || r.Gate {
		t.Fatal("canceled usage disappeared", r, err)
	}
}
