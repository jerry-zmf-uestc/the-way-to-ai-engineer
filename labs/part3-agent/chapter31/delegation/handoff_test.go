package main

import (
	"context"
	"errors"
	"testing"
)

func transferRequest(h *Handoff, to string) HandoffRequest {
	s := h.Snapshot()
	return HandoffRequest{ID: s.ID, Scope: s.Scope, From: s.Owner, To: to, Revision: s.Revision}
}
func handoffFixture(t *testing.T, maxHops int) *Handoff {
	t.Helper()
	h, err := NewHandoff(task(), dispatch(), "timeout-reader", map[string][]string{"timeout-reader": {"retry-reader"}, "retry-reader": {"timeout-reader"}}, maxHops, 4)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func TestHandoffPreservesRunScopeFactsAndBudget(t *testing.T) {
	state, err := handoffDemo()
	if err != nil || state.ID != "report" || state.Owner != "retry-reader" || state.Status != "completed" || state.Hops != 1 || state.Turns != 2 || len(state.Facts) != 2 {
		t.Fatal(state, err)
	}
	h := handoffFixture(t, 2)
	old := transferRequest(h, "retry-reader")
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); err != nil {
		t.Fatal(err)
	}
	if err = h.Transfer(context.Background(), old); !errors.Is(err, ErrHandoff) {
		t.Fatal("stale transfer", err)
	}
	if err = h.Transfer(context.Background(), transferRequest(h, "retry-reader")); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); err != nil {
		t.Fatal(err)
	}
	s := h.Snapshot()
	s.Facts[0].Value = "changed"
	if h.Snapshot().Facts[0].Value == "changed" {
		t.Fatal("snapshot shared")
	}
	if err = h.Transfer(context.Background(), transferRequest(h, "timeout-reader")); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); !errors.Is(err, ErrBudget) || h.Snapshot().Status != "failed" {
		t.Fatal("budget reset across handoff", err)
	}
}
func TestTransferPolicyScopeHopAndTerminal(t *testing.T) {
	h := handoffFixture(t, 1)
	for _, kind := range []string{"scope", "owner", "id", "target", "self"} {
		req := transferRequest(h, "retry-reader")
		switch kind {
		case "scope":
			req.Scope.User = "other"
		case "owner":
			req.From = "retry-reader"
		case "id":
			req.ID = "other"
		case "target":
			req.To = "missing"
		case "self":
			req.To = req.From
		}
		if err := h.Transfer(context.Background(), req); !errors.Is(err, ErrHandoff) {
			t.Fatal(kind, err)
		}
	}
	if err := h.Transfer(context.Background(), transferRequest(h, "retry-reader")); err != nil {
		t.Fatal(err)
	}
	if err := h.Transfer(context.Background(), transferRequest(h, "timeout-reader")); !errors.Is(err, ErrBudget) {
		t.Fatal("hop bound", err)
	}
	if err := h.Complete(context.Background(), transferRequest(h, "")); err != nil {
		t.Fatal(err)
	}
	if err := h.Transfer(context.Background(), transferRequest(h, "timeout-reader")); !errors.Is(err, ErrHandoff) {
		t.Fatal("terminal takeover", err)
	}
}

func TestHandoffTurnsBoundEvenWithoutTools(t *testing.T) {
	d := dispatch()
	d.Roles["timeout-reader"] = Role{Name: "timeout-reader", Run: func(_ context.Context, t Task, _ *Toolbox) (Result, error) {
		return Result{TaskID: t.ID, Parent: t.Parent, Actor: "timeout-reader", Scope: t.Scope}, nil
	}}
	h, err := NewHandoff(task(), d, "timeout-reader", nil, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); !errors.Is(err, ErrBudget) || h.Snapshot().Status != "failed" {
		t.Fatal("tool-free unbounded turns", err)
	}
}
func TestRunningOwnerCannotBeTransferredAndLateCancelNotCommitted(t *testing.T) {
	d := dispatch()
	started := make(chan struct{})
	release := make(chan struct{})
	d.Roles["timeout-reader"] = Role{Name: "timeout-reader", Tools: []string{"read-timeout"}, Run: func(ctx context.Context, t Task, box *Toolbox) (Result, error) {
		close(started)
		<-release
		fact, err := box.Read(ctx, "read-timeout")
		return Result{TaskID: t.ID, Parent: t.Parent, Actor: "timeout-reader", Scope: t.Scope, Facts: []Fact{fact}}, err
	}}
	h, err := NewHandoff(task(), d, "timeout-reader", map[string][]string{"timeout-reader": {"retry-reader"}}, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := transferRequest(h, "")
	done := make(chan error, 1)
	go func() { _, err := h.Execute(ctx, req); done <- err }()
	<-started
	if err = h.Transfer(context.Background(), transferRequest(h, "retry-reader")); !errors.Is(err, ErrHandoff) {
		t.Fatal("running transferred", err)
	}
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); !errors.Is(err, ErrHandoff) {
		t.Fatal("two active owners", err)
	}
	cancel()
	close(release)
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	state := h.Snapshot()
	if state.Status != "canceled" || len(state.Facts) != 1 || state.Turns != 1 {
		t.Fatal("late commit", state)
	}
}
func TestHandoffSnapshotsCapabilitiesAndRejectsInventedFacts(t *testing.T) {
	d := dispatch()
	edges := map[string][]string{"timeout-reader": {"retry-reader"}}
	h, err := NewHandoff(task(), d, "timeout-reader", edges, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	edges["timeout-reader"][0] = "missing"
	delete(d.Roles, "retry-reader")
	delete(d.ParentTools, "read-timeout")
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); err != nil {
		t.Fatal("policy mutable", err)
	}
	if err = h.Transfer(context.Background(), transferRequest(h, "retry-reader")); err != nil {
		t.Fatal("edge mutable", err)
	}
	d = dispatch()
	d.Roles["timeout-reader"] = Role{Name: "timeout-reader", Run: func(_ context.Context, t Task, _ *Toolbox) (Result, error) {
		return Result{TaskID: t.ID, Parent: t.Parent, Actor: "timeout-reader", Scope: t.Scope, Facts: []Fact{{"timeout", "999", "invented"}}}, nil
	}}
	h, err = NewHandoff(task(), d, "timeout-reader", map[string][]string{}, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Execute(context.Background(), transferRequest(h, "")); !errors.Is(err, ErrResult) || h.Snapshot().Status != "failed" {
		t.Fatal("invented fact committed", err)
	}
}
