package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestExperienceSelectionPreservesProvenanceAndExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	scope := ExperienceScope{"run-a", "lookup", "data-v1", "tool-v1", "rubric-v1"}
	memo := FeedbackMemo{Scope: scope, SourceAttempt: 1, FeedbackCode: "missing_evidence", Advice: "A directory points to a policy; read the policy before answering.", ExpiresAt: now.Add(time.Hour)}
	out, err := SelectExperience(context.Background(), scope, 2, now, []FeedbackMemo{memo}, 1024)
	if err != nil || len(out) != 1 || out[0] != memo {
		t.Fatal(out, err)
	}
	out[0].Advice = "changed"
	if memo.Advice == out[0].Advice {
		t.Fatal("selected memo aliases source")
	}
	for _, changed := range []ExperienceScope{
		{"run-b", "lookup", "data-v1", "tool-v1", "rubric-v1"},
		{"run-a", "compare", "data-v1", "tool-v1", "rubric-v1"},
		{"run-a", "lookup", "data-v2", "tool-v1", "rubric-v1"},
		{"run-a", "lookup", "data-v1", "tool-v2", "rubric-v1"},
		{"run-a", "lookup", "data-v1", "tool-v1", "rubric-v2"},
	} {
		out, err := SelectExperience(context.Background(), changed, 2, now, []FeedbackMemo{memo}, 1024)
		if err != nil || len(out) != 0 {
			t.Fatal("inapplicable memo selected", changed, out, err)
		}
	}
	for _, c := range []struct {
		attempt int
		now     time.Time
	}{{1, now}, {2, memo.ExpiresAt}} {
		out, err := SelectExperience(context.Background(), scope, c.attempt, c.now, []FeedbackMemo{memo}, 1024)
		if err != nil || len(out) != 0 {
			t.Fatal("current/future or expired experience selected", out, err)
		}
	}
}

func TestExperienceBudgetAndMalformedRecordReturnNoPartialInput(t *testing.T) {
	now := time.Now()
	scope := ExperienceScope{"r", "kind", "d", "t", "e"}
	first := FeedbackMemo{Scope: scope, SourceAttempt: 1, FeedbackCode: "task_failed", Advice: "first advice", ExpiresAt: now.Add(time.Hour)}
	second := first
	second.SourceAttempt = 2
	second.Advice = "second advice"
	payload, err := json.Marshal([]FeedbackMemo{first})
	if err != nil {
		t.Fatal(err)
	}
	one, err := SelectExperience(context.Background(), scope, 3, now, []FeedbackMemo{first}, len(payload))
	if err != nil || len(one) != 1 {
		t.Fatal("exact serialized budget rejected", one, err)
	}
	out, err := SelectExperience(context.Background(), scope, 3, now, []FeedbackMemo{first, second}, len(payload))
	if !errors.Is(err, ErrBudget) || out != nil {
		t.Fatal("partial experience published", out, err)
	}
	second.FeedbackCode = "model_claims_success"
	out, err = SelectExperience(context.Background(), scope, 3, now, []FeedbackMemo{first, second}, 1024)
	if !errors.Is(err, ErrInput) || out != nil {
		t.Fatal("unvalidated feedback promoted", out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err = SelectExperience(ctx, scope, 3, now, []FeedbackMemo{first}, 1024)
	if !errors.Is(err, context.Canceled) || out != nil {
		t.Fatal(out, err)
	}
}
