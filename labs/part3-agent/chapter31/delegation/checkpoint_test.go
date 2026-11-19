package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func checkpointFixture(t *testing.T) (Task, Dispatcher, map[string][]string, *Handoff) {
	t.Helper()
	task := Task{ID: "report", Parent: "request", Scope: Scope{"demo", "reader"}, Goal: "read timeout and retry"}
	d := Dispatcher{Roles: roles(), ParentTools: tools(), Ledger: &Ledger{Limit: 2}, MaxDepth: 2}
	edges := map[string][]string{"timeout-reader": {"retry-reader"}}
	h, err := NewHandoff(task, d, "timeout-reader", edges, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	return task, d, edges, h
}

func TestHandoffCheckpointRequiresQuiescentExecution(t *testing.T) {
	task, d, _, _ := checkpointFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	d.Roles["waiter"] = Role{Name: "waiter", Run: func(ctx context.Context, task Task, _ *Toolbox) (Result, error) {
		close(started)
		select {
		case <-release:
			return Result{TaskID: task.ID, Parent: task.Parent, Actor: "waiter", Scope: task.Scope}, nil
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}}
	h, err := NewHandoff(task, d, "waiter", nil, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := h.Execute(ctx, currentRequest(h, "")); done <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("worker not entered")
	}
	if data, err := CheckpointHandoff(h, "handoff-v1"); !errors.Is(err, ErrHandoff) || data != nil {
		t.Fatal("running exported as safe snapshot", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := CheckpointHandoff(h, "handoff-v1"); err != nil {
		t.Fatal("quiescent ready snapshot rejected", err)
	}
	if _, err := CheckpointHandoff(&Handoff{}, "handoff-v1"); !errors.Is(err, ErrHandoff) {
		t.Fatal("zero instance accepted")
	}
}

func TestHandoffCheckpointPreservesInitialFactsAndUTF8(t *testing.T) {
	task, d, edges, _ := checkpointFixture(t)
	task.Context = []Fact{{"basis", "stable", "source"}}
	h, err := NewHandoff(task, d, "timeout-reader", edges, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	data, err := CheckpointHandoff(h, "handoff-v1")
	if err != nil {
		t.Fatal(err)
	}
	var saved HandoffCheckpoint
	if json.Unmarshal(data, &saved) != nil {
		t.Fatal("invalid produced JSON")
	}
	saved.State.Facts = nil
	missing, _ := json.Marshal(saved)
	if _, err := RestoreHandoff(missing, task, d, "timeout-reader", edges, 2, 4, "handoff-v1"); !errors.Is(err, ErrHandoff) {
		t.Fatal("original context dropped on restore")
	}
	task.Context[0].Value = string([]byte{0xff})
	bad, err := NewHandoff(task, d, "timeout-reader", edges, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := CheckpointHandoff(bad, "handoff-v1"); !errors.Is(err, ErrHandoff) || data != nil {
		t.Fatal("JSON silently replaced invalid source bytes", err)
	}
}

func currentRequest(h *Handoff, to string) HandoffRequest {
	s := h.Snapshot()
	return HandoffRequest{ID: s.ID, Scope: s.Scope, From: s.Owner, To: to, Revision: s.Revision}
}

func TestHandoffCheckpointResumesRoleAndBudget(t *testing.T) {
	task, d, edges, h := checkpointFixture(t)
	if _, err := h.Execute(context.Background(), currentRequest(h, "")); err != nil {
		t.Fatal(err)
	}
	if err := h.Transfer(context.Background(), currentRequest(h, "retry-reader")); err != nil {
		t.Fatal(err)
	}
	data, err := CheckpointHandoff(h, "handoff-v1")
	if err != nil {
		t.Fatal(err)
	}
	d.Ledger = &Ledger{Limit: 2}
	restored, err := RestoreHandoff(data, task, d, "timeout-reader", edges, 2, 4, "handoff-v1")
	if err != nil || d.Ledger.Used != 1 || restored.Snapshot().Owner != "retry-reader" || restored.Snapshot().Revision != 4 || len(restored.Snapshot().Facts) != 1 {
		t.Fatal("role/consumption lost", err, d.Ledger.Used)
	}
	if _, err := restored.Execute(context.Background(), currentRequest(restored, "")); err != nil {
		t.Fatal(err)
	}
	if err := restored.Complete(context.Background(), currentRequest(restored, "")); err != nil {
		t.Fatal(err)
	}
	if d.Ledger.Used != 2 || len(restored.Snapshot().Facts) != 2 || restored.Snapshot().Revision != 7 {
		t.Fatal(restored.Snapshot(), d.Ledger.Used)
	}
	terminal, err := CheckpointHandoff(restored, "handoff-v1")
	if err != nil {
		t.Fatal(err)
	}
	d.Ledger = &Ledger{Limit: 2}
	complete, err := RestoreHandoff(terminal, task, d, "timeout-reader", edges, 2, 4, "handoff-v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := complete.Execute(context.Background(), currentRequest(complete, "")); !errors.Is(err, ErrHandoff) || d.Ledger.Used != 2 {
		t.Fatal("completed snapshot executed again", err)
	}
}

func TestHandoffCheckpointRejectsChangedStateAndConfiguration(t *testing.T) {
	task, d, edges, h := checkpointFixture(t)
	data, err := CheckpointHandoff(h, "handoff-v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*HandoffCheckpoint){
		func(s *HandoffCheckpoint) { s.State.Status = "running" },
		func(s *HandoffCheckpoint) { s.State.Owner = "retry-reader" },
		func(s *HandoffCheckpoint) { s.State.Revision++ },
		func(s *HandoffCheckpoint) { s.State.Scope.User = "other" },
		func(s *HandoffCheckpoint) { s.State.ID = "other" },
		func(s *HandoffCheckpoint) { s.State.Hops = 3 },
		func(s *HandoffCheckpoint) { s.Used = 3 },
		func(s *HandoffCheckpoint) { s.State.Facts = []Fact{{"x", "1", "s"}, {"x", "2", "s"}} },
	} {
		var altered HandoffCheckpoint
		if json.Unmarshal(data, &altered) != nil {
			t.Fatal("bad fixture")
		}
		change(&altered)
		bad, _ := json.Marshal(altered)
		if out, err := RestoreHandoff(bad, task, d, "timeout-reader", edges, 2, 4, "handoff-v1"); !errors.Is(err, ErrHandoff) || out != nil || d.Ledger.Used != 0 {
			t.Fatal("bad restore changed ledger or state", err)
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), data...), []byte(" {}")...), []byte(strings.Replace(string(data), `"Schema":1`, `"Schema":1,"schema":1`, 1)), []byte(strings.Repeat("x", (32<<10)+1))} {
		if _, err := RestoreHandoff(bad, task, d, "timeout-reader", edges, 2, 4, "handoff-v1"); !errors.Is(err, ErrHandoff) {
			t.Fatal("bad JSON accepted", err)
		}
	}
	if _, err := RestoreHandoff(data, task, d, "timeout-reader", edges, 2, 4, "handoff-v2"); !errors.Is(err, ErrHandoff) {
		t.Fatal("policy change accepted")
	}
	d.Ledger.Used = 2
	restored, err := RestoreHandoff(data, task, d, "timeout-reader", edges, 2, 4, "handoff-v1")
	if err != nil || d.Ledger.Used != 2 {
		t.Fatal("current budget moved backwards", err)
	}
	if _, err := restored.Execute(context.Background(), currentRequest(restored, "")); !errors.Is(err, ErrBudget) || d.Ledger.Used != 2 {
		t.Fatal("restore refreshed exhausted budget", err)
	}
}
