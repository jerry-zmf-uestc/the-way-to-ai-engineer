package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func acceptanceFixture() (TaskInput, AcceptancePolicy) {
	return TaskInput{ID: "report", Input: "Report timeout and retry", AllowedSources: []string{"guide"}}, AcceptancePolicy{Version: "numeric-report-v1", Criteria: []string{"timeout", "retry"}, MaxCandidates: 2}
}

func numericReportReview(_ context.Context, candidate AcceptanceCandidate) (AcceptanceVerdict, error) {
	checks := []CriterionDecision{}
	for _, key := range []string{"timeout", "retry"} {
		check := CriterionDecision{ID: key, Status: "fail", Gap: "missing " + key}
		if strings.Contains(candidate.Text, key+"=") {
			check.Status, check.Gap, check.Evidence = "pass", "", "literal field found in current candidate"
		}
		checks = append(checks, check)
	}
	return AcceptanceVerdict{Attempt: candidate.Attempt, Digest: candidate.Digest, Binding: candidate.Binding, Checks: checks}, nil
}

func TestAcceptanceFeedbackRevisesThenRechecks(t *testing.T) {
	input, policy := acceptanceFixture()
	calls := 0
	generate := func(_ context.Context, view TaskInput, feedback []CriterionDecision) (string, error) {
		calls++
		if view.AllowedSources[0] != "guide" {
			t.Fatal("capability copy changed")
		}
		view.AllowedSources[0] = "changed"
		if calls == 1 {
			if len(feedback) != 0 {
				t.Fatal("unexpected initial feedback")
			}
			return "timeout=30", nil
		}
		if len(feedback) != 1 || feedback[0].ID != "retry" || feedback[0].Gap != "missing retry" {
			t.Fatal("actual gap did not reach generator", feedback)
		}
		return "timeout=30 retry=2", nil
	}
	report, err := RunAcceptance(context.Background(), input, policy, generate, numericReportReview)
	if err != nil || report.Status != "passed" || report.Output != "timeout=30 retry=2" || report.GenerationCalls != 2 || report.ReviewCalls != 2 || len(report.Attempts) != 2 || input.AllowedSources[0] != "guide" {
		t.Fatal(report, err)
	}
	if report.Attempts[0].Candidate.Text != "timeout=30" || report.Attempts[0].Checks[1].Status != "fail" || report.Attempts[0].Candidate.Digest == report.Attempts[1].Candidate.Digest {
		t.Fatal("old failure or candidate identity lost", report)
	}
}

func TestAcceptanceNeverPublishesUnverifiedCandidate(t *testing.T) {
	input, policy := acceptanceFixture()
	generate := func(context.Context, TaskInput, []CriterionDecision) (string, error) { return "timeout=30", nil }
	report, err := RunAcceptance(context.Background(), input, policy, generate, numericReportReview)
	if !errors.Is(err, ErrAcceptanceBudget) || report.Output != "" || report.GenerationCalls != 2 || report.ReviewCalls != 2 {
		t.Fatal("budget delivered failed draft", report, err)
	}
	for _, mode := range []string{"unknown", "missing", "duplicate", "wrong-digest", "no-evidence", "extra", "gapless", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			review := func(ctx context.Context, c AcceptanceCandidate) (AcceptanceVerdict, error) {
				v, _ := numericReportReview(ctx, c)
				switch mode {
				case "unknown":
					v.Checks[1] = CriterionDecision{ID: "retry", Status: "unknown"}
				case "missing":
					v.Checks = v.Checks[:1]
				case "duplicate":
					v.Checks[1].ID = "timeout"
				case "wrong-digest":
					v.Digest = "old"
				case "no-evidence":
					v.Checks[0].Evidence = ""
				case "extra":
					v.Checks[1].ID = "new-standard"
				case "gapless":
					v.Checks[1].Gap = ""
				case "oversized":
					v.Checks[0].Evidence = strings.Repeat("x", 1025)
				}
				return v, nil
			}
			r, err := RunAcceptance(context.Background(), input, policy, generate, review)
			wanted := ErrAcceptance
			if mode == "unknown" {
				wanted = ErrAcceptanceReview
				if r.Status != "review" {
					t.Fatal(r.Status)
				}
			}
			if !errors.Is(err, wanted) || r.Output != "" || r.GenerationCalls != 1 || r.ReviewCalls != 1 {
				t.Fatal("unverified draft published or retried", r, err)
			}
		})
	}
}

func TestAcceptanceRejectsPolicyBeforeCallbacks(t *testing.T) {
	input, policy := acceptanceFixture()
	calls := 0
	generate := func(context.Context, TaskInput, []CriterionDecision) (string, error) { calls++; return "x", nil }
	for _, bad := range []AcceptancePolicy{
		{Version: "v", Criteria: []string{"x"}, MaxCandidates: 0},
		{Version: "v", Criteria: []string{"x"}, MaxCandidates: 5},
		{Version: "v", Criteria: []string{"x", "x"}, MaxCandidates: 2},
		{Version: "v", Criteria: []string{" "}, MaxCandidates: 2},
	} {
		if r, err := RunAcceptance(context.Background(), input, bad, generate, numericReportReview); !errors.Is(err, ErrAcceptance) || calls != 0 || r.Output != "" {
			t.Fatal(r, err)
		}
	}
	input.Input = strings.Repeat("x", 4097)
	if _, err := RunAcceptance(context.Background(), input, policy, generate, numericReportReview); !errors.Is(err, ErrAcceptance) || calls != 0 {
		t.Fatal(err)
	}
}

func TestAcceptanceCancellationAndErrorCause(t *testing.T) {
	input, policy := acceptanceFixture()
	cause := errors.New("fixture generation unavailable")
	generate := func(context.Context, TaskInput, []CriterionDecision) (string, error) { return "partial", cause }
	r, err := RunAcceptance(context.Background(), input, policy, generate, numericReportReview)
	if !errors.Is(err, cause) || r.GenerationCalls != 1 || r.ReviewCalls != 0 || r.Output != "" {
		t.Fatal(r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	review := func(ctx context.Context, c AcceptanceCandidate) (AcceptanceVerdict, error) {
		v, _ := numericReportReview(ctx, c)
		cancel()
		return v, nil
	}
	generate = func(context.Context, TaskInput, []CriterionDecision) (string, error) {
		return "timeout=30 retry=2", nil
	}
	r, err = RunAcceptance(ctx, input, policy, generate, review)
	if !errors.Is(err, context.Canceled) || r.Output != "" || r.Status == "passed" {
		t.Fatal("canceled approval published", r, err)
	}
}
