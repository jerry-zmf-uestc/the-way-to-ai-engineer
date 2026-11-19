package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func hypothesisForSelection() Hypotheses {
	return Hypotheses{Revision: generatorRevision, PromptVersion: "hypothetical-v1", Rows: []Hypothesis{{Text: "Request duration limit.", Trust: "hypothetical", InputTokens: 20, OutputTokens: 10}}}
}

func TestQueryFallbackRetainsOriginalAndGenerationCost(t *testing.T) {
	original := "  request duration  "
	h := hypothesisForSelection()
	r, err := SelectExpandedQuery(context.Background(), original, &h, nil)
	if err != nil || r.Origin != "hypothetical" || r.Text != h.Rows[0].Text || !r.GenerationAttempted || !r.GenerationUsageKnown {
		t.Fatal(r, err)
	}
	h.Rows[0].AtOutputLimit, h.Rows[0].OutputTokens = true, 64
	r, err = SelectExpandedQuery(context.Background(), original, &h, nil)
	if err != nil || r.Text != original || r.Reason != "output_limit" || !r.GenerationUsageKnown || r.OutputTokens != 64 || h.Rows[0].Text != "Request duration limit." {
		t.Fatal("fallback rewrote candidate or erased cost", r, h, err)
	}
	r, err = SelectExpandedQuery(context.Background(), original, nil, errors.New("synthetic internal cause"))
	if err != nil || r.Reason != "generation_failed" || !r.GenerationAttempted || r.GenerationUsageKnown {
		t.Fatal("failed generation became free", r, err)
	}
	r, err = SelectExpandedQuery(context.Background(), original, nil, nil)
	if err != nil || r.GenerationAttempted || r.Reason != "not_generated" {
		t.Fatal(r, err)
	}
}

func TestQueryFallbackStopsCancellationAndDistinguishesDeadlines(t *testing.T) {
	h := hypothesisForSelection()
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := SelectExpandedQuery(stopped, "query", &h, nil); !errors.Is(err, context.Canceled) || !r.GenerationUsageKnown || r.Text != "" || r.Origin != "stopped" {
		t.Fatal(r, err)
	}
	if r, err := SelectExpandedQuery(context.Background(), "query", nil, fmt.Errorf("child: %w", context.Canceled)); !errors.Is(err, context.Canceled) || r.Text != "" {
		t.Fatal("downstream cancellation continued", err)
	}
	r, err := SelectExpandedQuery(context.Background(), "query", nil, context.DeadlineExceeded)
	if err != nil || r.Reason != "generation_deadline" || r.GenerationUsageKnown {
		t.Fatal("step deadline cannot fall back", r, err)
	}
	expired, stop := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stop()
	if _, err := SelectExpandedQuery(expired, "query", &h, context.DeadlineExceeded); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("overall deadline continued", err)
	}
}

func TestQueryFallbackInvalidCandidatesDoNotBecomeEvidence(t *testing.T) {
	for _, edit := range []func(*Hypotheses){
		func(h *Hypotheses) { h.Revision = "other" }, func(h *Hypotheses) { h.Rows[0].Trust = "verified" },
		func(h *Hypotheses) { h.Rows[0].Text = "" }, func(h *Hypotheses) { h.Rows[0].Text = string([]byte{0xff}) },
		func(h *Hypotheses) { h.Rows = nil },
	} {
		h := hypothesisForSelection()
		edit(&h)
		r, err := SelectExpandedQuery(context.Background(), "original", &h, nil)
		if err != nil || r.Text != "original" || r.Origin != "original" || r.Reason != "invalid_hypothesis" {
			t.Fatal(r, err)
		}
	}
	for _, query := range []string{"", " ", string([]byte{0xff}), strings.Repeat("x", 4097)} {
		if _, err := SelectExpandedQuery(context.Background(), query, nil, nil); !errors.Is(err, ErrInput) {
			t.Fatal("invalid original rescued", err)
		}
	}
}

func TestQueryFallbackReusesOnlyOriginalAuthorizedCorpus(t *testing.T) {
	docs := []Document{
		{ID: "current", Tenant: "demo", Version: "v1", Text: "request timeout"},
		{ID: "other", Tenant: "other", Version: "v1", Text: "request timeout"},
		{ID: "old", Tenant: "demo", Version: "old", Text: "request timeout"},
	}
	h := hypothesisForSelection()
	h.Rows[0].AtOutputLimit, h.Rows[0].OutputTokens = true, 64
	h.Rows[0].Text = "discarded text requesting other scope"
	r, err := SelectExpandedQuery(context.Background(), "request timeout", &h, nil)
	if err != nil {
		t.Fatal(err)
	}
	hits := Lexical(docs, r.Text, "demo", "v1", 3)
	if len(hits) != 1 || hits[0].ID != "current" || h.Rows[0].Text != "discarded text requesting other scope" {
		t.Fatal("fallback changed corpus or scope", hits, h)
	}
}
