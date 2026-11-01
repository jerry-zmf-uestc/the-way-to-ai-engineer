package main

import (
	"context"
	"errors"
	"testing"
)

func TestAcceptanceDoesNotReuseVerdictFromDifferentTask(t *testing.T) {
	input, policy := acceptanceFixture()
	generate := func(context.Context, TaskInput, []CriterionDecision) (string, error) {
		return "timeout=30 retry=2", nil
	}
	var cached AcceptanceVerdict
	firstReview := func(ctx context.Context, c AcceptanceCandidate) (AcceptanceVerdict, error) {
		v, err := numericReportReview(ctx, c)
		cached = v
		return v, err
	}
	first, err := RunAcceptance(context.Background(), input, policy, generate, firstReview)
	if err != nil || first.Status != "passed" {
		t.Fatal(first, err)
	}
	input.Input = "Report timeout and retry for the current production version"
	review := func(context.Context, AcceptanceCandidate) (AcceptanceVerdict, error) { return cached, nil }
	current, err := RunAcceptance(context.Background(), input, policy, generate, review)
	if !errors.Is(err, ErrAcceptance) || current.Output != "" {
		t.Fatal("old verdict accepted after task requirements changed", current.Status, err)
	}
}

func TestAcceptanceBindingChangesWithPolicyAndCapabilities(t *testing.T) {
	input, policy := acceptanceFixture()
	generate := func(context.Context, TaskInput, []CriterionDecision) (string, error) {
		return "timeout=30 retry=2", nil
	}
	var old AcceptanceVerdict
	base, err := RunAcceptance(context.Background(), input, policy, generate, func(ctx context.Context, c AcceptanceCandidate) (AcceptanceVerdict, error) {
		v, err := numericReportReview(ctx, c)
		old = v
		return v, err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*TaskInput, *AcceptancePolicy){
		func(i *TaskInput, _ *AcceptancePolicy) { i.ID = "different-task" },
		func(i *TaskInput, _ *AcceptancePolicy) { i.AllowedSources = []string{"current-guide"} },
		func(i *TaskInput, _ *AcceptancePolicy) { i.AllowedTools = []string{"read"} },
		func(_ *TaskInput, p *AcceptancePolicy) { p.Version = "report-v2" },
		func(_ *TaskInput, p *AcceptancePolicy) { p.MaxCandidates = 3 },
		func(_ *TaskInput, p *AcceptancePolicy) { p.Criteria = []string{"retry", "timeout"} },
	} {
		i, p := input, policy
		change(&i, &p)
		r, err := RunAcceptance(context.Background(), i, p, generate, func(context.Context, AcceptanceCandidate) (AcceptanceVerdict, error) { return old, nil })
		if !errors.Is(err, ErrAcceptance) || r.Binding == base.Binding || r.Output != "" || r.GenerationCalls != 1 {
			t.Fatal("changed binding used old verdict", r, err)
		}
	}
	repeat, err := RunAcceptance(context.Background(), input, policy, generate, numericReportReview)
	if err != nil || repeat.Binding != base.Binding {
		t.Fatal("binding is a context snapshot, not unique Run identity", repeat, err)
	}
}

func TestAcceptanceRejectsEarlierAttemptForSameText(t *testing.T) {
	input, policy := acceptanceFixture()
	generate := func(context.Context, TaskInput, []CriterionDecision) (string, error) {
		return "timeout=30 retry=2", nil
	}
	var earlier AcceptanceVerdict
	review := func(ctx context.Context, c AcceptanceCandidate) (AcceptanceVerdict, error) {
		if c.Attempt == 1 {
			v, err := numericReportReview(ctx, c)
			earlier = v
			v.Checks = append([]CriterionDecision(nil), v.Checks...)
			v.Checks[1] = CriterionDecision{ID: "retry", Status: "fail", Gap: "fixture requests a fresh check"}
			return v, err
		}
		return earlier, nil
	}
	r, err := RunAcceptance(context.Background(), input, policy, generate, review)
	if !errors.Is(err, ErrAcceptance) || r.Output != "" || r.GenerationCalls != 2 || r.ReviewCalls != 2 || len(r.Attempts) != 2 || r.Attempts[0].Candidate.Digest != r.Attempts[1].Candidate.Digest {
		t.Fatal("same text bypassed new attempt check", r, err)
	}
}
