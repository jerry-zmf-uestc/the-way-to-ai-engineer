package main

import (
	"context"
	"errors"
	"testing"
)

func initial() Plan {
	return Plan{Version: 1, Nodes: []Node{{ID: "retry", Goal: "retry", Query: "retry"}, {ID: "timeout", Goal: "timeout", Query: "E42", Depends: []string{"retry"}}}}
}
func goals() map[string]string { return map[string]string{"retry": "2", "timeout": "30"} }
func TestReplanPreservesCompletedWorkAndIndependentGoal(t *testing.T) {
	r, err := Run(context.Background(), initial(), goals(), Limits{5, 1}, read, repair)
	if !Complete(goals(), r, err) || r.Attempts != 3 || r.Replans != 1 || r.Plan.Version != 2 || r.Progress["retry"].Fact.Value != "2" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	_, err = Run(context.Background(), initial(), goals(), Limits{5, 0}, read, nil)
	if !errors.Is(err, ErrFailed) {
		t.Fatal("failed plan reported success")
	}
	wrong := goals()
	wrong["timeout"] = "60"
	if Complete(wrong, r, nil) {
		t.Fatal("all completed substituted for goal correctness")
	}
}
func TestRejectsInvalidGraphBeforeExecution(t *testing.T) {
	for _, p := range []Plan{
		{Version: 1, Nodes: []Node{{ID: "x", Goal: "timeout", Query: "q", Depends: []string{"missing"}}}},
		{Version: 1, Nodes: []Node{{ID: "x", Goal: "timeout", Query: "q", Depends: []string{"x"}}}},
		{Version: 1, Nodes: []Node{{ID: "x", Goal: "timeout", Query: "q"}, {ID: "x", Goal: "timeout", Query: "q"}}},
	} {
		calls := 0
		_, err := Run(context.Background(), p, goals(), Limits{5, 1}, func(context.Context, Node) (Fact, error) { calls++; return Fact{}, nil }, repair)
		if !errors.Is(err, ErrPlan) || calls != 0 {
			t.Fatal("invalid graph executed")
		}
	}
}
func TestRevisionCannotRewriteCompletedEvidence(t *testing.T) {
	old := initial()
	progress := map[string]Progress{"retry": {State: "completed", Fact: Fact{Key: "retry", Value: "2", Source: "s"}}, "timeout": {State: "failed"}}
	next := clonePlan(old)
	next.Version++
	next.Nodes[0].Query = "changed"
	if _, err := Revise(old, next, progress, goals()); !errors.Is(err, ErrRevision) {
		t.Fatal("completed node changed")
	}
	next = clonePlan(old)
	if _, err := Revise(old, next, progress, goals()); !errors.Is(err, ErrRevision) {
		t.Fatal("stale version accepted")
	}
}
func TestReplanningBudgetAndSnapshotOwnership(t *testing.T) {
	count := 0
	replan := func(_ context.Context, p Plan, state map[string]Progress) (Plan, error) {
		count++
		state["retry"] = Progress{State: "failed"}
		p.Version++
		return p, nil
	}
	r, err := Run(context.Background(), initial(), goals(), Limits{8, 1}, read, replan)
	if !errors.Is(err, ErrFailed) || count != 1 || r.Attempts != 3 || r.Progress["retry"].State != "completed" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	r, err = Run(context.Background(), initial(), goals(), Limits{2, 1}, read, repair)
	if !errors.Is(err, ErrBudget) || r.Attempts != 2 {
		t.Fatal("attempt budget ignored")
	}
}
func TestCancellationAndWrongFactNeverCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := Run(ctx, initial(), goals(), Limits{5, 1}, func(context.Context, Node) (Fact, error) {
		cancel()
		return Fact{Key: "retry", Value: "2", Source: "late"}, nil
	}, repair)
	if !errors.Is(err, context.Canceled) || r.Progress["retry"].State == "completed" {
		t.Fatal("late fact committed")
	}
	cause := errors.New("downstream")
	_, err = Run(context.Background(), initial(), goals(), Limits{5, 0}, func(context.Context, Node) (Fact, error) { return Fact{}, cause }, nil)
	if !errors.Is(err, cause) {
		t.Fatal("cause lost")
	}
}
