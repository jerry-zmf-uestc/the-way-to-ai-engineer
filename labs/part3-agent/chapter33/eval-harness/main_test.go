package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func load(t *testing.T) Fixture {
	t.Helper()
	f, err := Load(fixtureBytes)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func TestFailuresReviewsAndUsageCannotDisappear(t *testing.T) {
	f := load(t)
	r, err := Evaluate(f.Version, "candidate", f.Cases, f.Candidate, f.Labels)
	if err != nil || r.Total != 3 || r.Passed != 1 || r.Failed != 1 || r.Review != 1 || r.UnknownUsage != 1 || r.SupportCoverage != 0 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	r, err = Evaluate(f.Version, "missing", f.Cases, f.Candidate[:2], f.Labels)
	if err != nil || r.Total != 3 || r.Failed != 2 || r.Rows[2].Status != "failed" {
		t.Fatal("missing case dropped")
	}
}
func TestUnknownLabelCannotBecomeSupported(t *testing.T) {
	f := load(t)
	r, _ := Evaluate(f.Version, "base", f.Cases, f.Baseline, f.Labels)
	if r.Passed != 2 || r.Review != 1 || r.SupportRate != 1 || r.SupportCoverage != 0.5 {
		t.Fatalf("report=%+v", r)
	}
	f.Labels[1].Verdict = "unsupported"
	r, _ = Evaluate(f.Version, "bad", f.Cases, f.Baseline, f.Labels)
	if r.Review != 0 || r.Failed != 1 || r.SupportRate != 0.5 || r.SupportCoverage != 1 {
		t.Fatal("unsupported label ignored")
	}
}
func TestToolFailureOverridesSuccessfulTextAndPairedGate(t *testing.T) {
	f := load(t)
	base, _ := Evaluate(f.Version, "base", f.Cases, f.Baseline, f.Labels)
	next, _ := Evaluate(f.Version, "next", f.Cases, f.Candidate, f.Labels)
	reg, err := Regressions(base, next)
	if err != nil || len(reg) != 1 || reg[0] != "timeout" {
		t.Fatalf("reg=%v err=%v", reg, err)
	}
	next.DatasetVersion = "other"
	if _, err := Regressions(base, next); !errors.Is(err, ErrInvalid) {
		t.Fatal("mismatched dataset accepted")
	}
}
func TestDuplicateAndUnknownResultsFailClosed(t *testing.T) {
	f := load(t)
	duplicate := append(append([]Result(nil), f.Baseline...), f.Baseline[0])
	if _, err := Evaluate(f.Version, "x", f.Cases, duplicate, f.Labels); !errors.Is(err, ErrInvalid) {
		t.Fatal("duplicate result accepted")
	}
	f.Baseline[0].CaseID = "unknown"
	if _, err := Evaluate(f.Version, "x", f.Cases, f.Baseline, f.Labels); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown case accepted")
	}
	if _, err := Load(append(fixtureBytes, []byte(`{}`)...)); !errors.Is(err, ErrInvalid) {
		t.Fatal("trailing payload accepted")
	}
}
func TestRecallDeduplicatesAndIntervalsShowUncertainty(t *testing.T) {
	if got := recall([]string{"a", "b", "b"}, []string{"a", "a"}); got != 0.5 {
		t.Fatalf("recall=%v", got)
	}
	low := wilson(1, 3)
	if low[0] >= 1.0/3 || low[1] <= 1.0/3 || low[0] < 0 || low[1] > 1 {
		t.Fatalf("interval=%v", low)
	}
}

func TestRunnerBindsIdentityAndRetainsCanceledCases(t *testing.T) {
	f := load(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	results, err := RunCases(ctx, f.Cases, time.Second, func(context.Context, TaskInput) (Result, error) {
		calls++
		cancel()
		return Result{CaseID: "spoof", RunID: "spoof", Claims: []Claim{{Text: "late"}}, Tokens: 5, TokensKnown: true}, nil
	})
	if err != nil || calls != 1 || len(results) != 3 || results[0].CaseID != "timeout" || results[0].Error == "" || results[1].Error != "canceled_before_start" || results[0].Tokens != 5 {
		t.Fatalf("results=%+v calls=%d err=%v", results, calls, err)
	}
	report, err := Evaluate(f.Version, "canceled", f.Cases, results, f.Labels)
	if err != nil || report.Total != 3 || report.Failed != 3 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func TestAllowedCitationMustHaveBeenProvidedToRun(t *testing.T) {
	f := load(t)
	f.Baseline[0].Evidence = nil
	r, err := Evaluate(f.Version, "missing_context", f.Cases, f.Baseline, f.Labels)
	if err != nil || r.Rows[0].Status != "failed" {
		t.Fatal("citation accepted without run evidence")
	}
}

func TestRunnerValidatesCasesBeforeExecutionAndCopiesConstraints(t *testing.T) {
	f := load(t)
	calls := 0
	invalid := append(append([]Case(nil), f.Cases...), f.Cases[0])
	task := func(context.Context, TaskInput) (Result, error) { calls++; return Result{}, nil }
	if _, err := RunCases(context.Background(), invalid, time.Second, task); !errors.Is(err, ErrInvalid) || calls != 0 {
		t.Fatal("invalid cases executed")
	}
	before := f.Cases[0].AllowedSources[0]
	_, err := RunCases(context.Background(), f.Cases, time.Second, func(_ context.Context, c TaskInput) (Result, error) {
		if len(c.AllowedSources) > 0 {
			c.AllowedSources[0] = "changed"
		}
		return Result{}, nil
	})
	if err != nil || f.Cases[0].AllowedSources[0] != before {
		t.Fatal("execution changed rubric")
	}
}
