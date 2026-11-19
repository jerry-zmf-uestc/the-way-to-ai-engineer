package main

import (
	"context"
	"errors"
	"testing"
)

func TestObservationChangesActionAndGoalScore(t *testing.T) {
	task := Task{ID: "alias", Query: "E42", Fallback: "timeout", Want: "Timeout is 30 seconds."}
	fixed, err := Run(context.Background(), task, Limits{4, 3}, Choose(false), search)
	if err != nil || Complete(task, fixed, err) || !fixed.Refused {
		t.Fatal("fixed policy unexpectedly completed")
	}
	adaptive, err := Run(context.Background(), task, Limits{4, 3}, Choose(true), search)
	if !Complete(task, adaptive, err) || adaptive.Rounds != 3 || adaptive.Tools != 2 || adaptive.Decisions[1].Action.Query != "timeout" {
		t.Fatalf("report=%+v err=%v", adaptive, err)
	}
}
func TestStallAndIndependentBudgets(t *testing.T) {
	task := Task{ID: "x", Query: "unknown"}
	repeat := func(context.Context, Task, []Observation) (Decision, error) {
		return Decision{Action: &Action{Tool: "search", Query: "unknown"}}, nil
	}
	r, err := Run(context.Background(), task, Limits{8, 8}, repeat, search)
	if !errors.Is(err, ErrStalled) || r.Tools != 2 {
		t.Fatalf("stall=%+v %v", r, err)
	}
	r, err = Run(context.Background(), Task{ID: "alias", Query: "E42", Fallback: "timeout"}, Limits{4, 1}, Choose(true), search)
	if !errors.Is(err, ErrBudget) || r.Tools != 1 {
		t.Fatal("tool budget ignored")
	}
	r, err = Run(context.Background(), Task{ID: "x", Query: "timeout"}, Limits{1, 3}, Choose(true), search)
	if !errors.Is(err, ErrBudget) || r.Tools != 1 {
		t.Fatal("round budget ignored")
	}
}
func TestBadBranchesProvenanceAndUnknownTools(t *testing.T) {
	for _, d := range []Decision{{Final: "invented", Source: "missing"}, {Action: &Action{Tool: "delete", Query: "x"}}, {Action: &Action{Tool: "search", Query: "x"}, Final: "both"}} {
		calls := 0
		_, err := Run(context.Background(), Task{ID: "x", Query: "x"}, Limits{3, 3}, func(context.Context, Task, []Observation) (Decision, error) { return d, nil }, func(context.Context, Action) (Observation, error) { calls++; return Observation{}, nil })
		if !errors.Is(err, ErrProtocol) || calls != 0 {
			t.Fatal("invalid decision executed")
		}
	}
}
func TestFailureIsObservationButNeverEvidence(t *testing.T) {
	cause := errors.New("private downstream failure")
	r, err := Run(context.Background(), Task{ID: "x", Query: "timeout"}, Limits{3, 3}, Choose(false), func(context.Context, Action) (Observation, error) {
		return Observation{Source: "fake", Text: "false success"}, cause
	})
	if err != nil || !r.Refused || r.History[0].ErrorCode != "tool_failed" || r.History[0].Text != "" {
		t.Fatalf("failure lost: %+v %v", r, err)
	}
}
func TestCancellationRejectsLateDecisionAndTool(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := Run(ctx, Task{ID: "x", Query: "timeout"}, Limits{3, 3}, func(context.Context, Task, []Observation) (Decision, error) {
		cancel()
		return Decision{Refuse: true}, nil
	}, search)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("late decision accepted")
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	_, err = Run(ctx, Task{ID: "x", Query: "timeout"}, Limits{3, 3}, Choose(false), func(context.Context, Action) (Observation, error) {
		cancel()
		return Observation{Source: "s", Text: "late"}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("late tool accepted")
	}
}

func TestGoldAnswerIsNotDecisionInput(t *testing.T) {
	_, err := Run(context.Background(), Task{ID: "x", Query: "unknown", Want: "hidden gold"}, Limits{3, 3}, func(_ context.Context, task Task, _ []Observation) (Decision, error) {
		if task.Want != "" {
			t.Fatal("gold answer leaked")
		}
		return Decision{Refuse: true}, nil
	}, search)
	if err != nil {
		t.Fatal(err)
	}
}
