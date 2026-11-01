package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunnerInputDoesNotExposeGoldCriteria(t *testing.T) {
	cases := []Case{{ID: "c", Split: "regression", Input: "public question", AllowedSources: []string{"allowed"}, AllowedTools: []string{"read"}, RequiredClaims: []string{"gold_answer_canary"}, Relevant: []string{"gold_relevance_canary"}, ExpectAbstain: true}}
	_, err := RunCases(context.Background(), cases, time.Second, func(_ context.Context, in TaskInput) (Result, error) {
		data, _ := json.Marshal(in)
		if strings.Contains(string(data), "gold_") || strings.Contains(string(data), "ExpectAbstain") || strings.Contains(string(data), "Split") {
			t.Fatal("scoring labels exposed", string(data))
		}
		in.AllowedTools[0] = "changed"
		return Result{}, nil
	})
	if err != nil || cases[0].AllowedTools[0] != "read" {
		t.Fatal("input policy shared", err)
	}
}
func TestRegressionRejectsDuplicateBaselineAndUnknownStatuses(t *testing.T) {
	base := Report{DatasetVersion: "v", ScorerVersion: "rules-v1", Rows: []Row{{CaseID: "a", Status: "passed"}, {CaseID: "a", Status: "passed"}}}
	next := Report{DatasetVersion: "v", ScorerVersion: "rules-v1", Rows: []Row{{CaseID: "a", Status: "passed"}, {CaseID: "b", Status: "passed"}}}
	if _, err := Regressions(base, next); !errors.Is(err, ErrInvalid) {
		t.Fatal("different case set accepted", err)
	}
	base.Rows[1].CaseID = "b"
	next.Rows[1].Status = "unrecognized"
	if _, err := Regressions(base, next); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown status passed", err)
	}
}
func TestKnownUsageOverflowDoesNotWrapReport(t *testing.T) {
	cases := []Case{{ID: "a", Split: "regression", Input: "q", ExpectAbstain: true}, {ID: "b", Split: "regression", Input: "q", ExpectAbstain: true}}
	rows := []Result{{CaseID: "a", RunID: "r1", Abstained: true, Tokens: int(^uint(0) >> 1), TokensKnown: true}, {CaseID: "b", RunID: "r2", Abstained: true, Tokens: 1, TokensKnown: true}}
	if _, err := Evaluate("v", "c", cases, rows, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("overflow treated as low usage", err)
	}
}
