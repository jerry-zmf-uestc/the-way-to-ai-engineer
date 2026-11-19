package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

func TestParentCancelWaitsOnlyForItsBoundChildren(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	m, err := tasks.NewManager(2, time.Second, func(ctx context.Context, task tasks.Task) (string, error) {
		started <- task.Input
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-release:
			return "evidence", nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	goal := ParentGoal{ID: "report-a", Scope: tasks.Scope{Tenant: "demo", User: "reader"}, Revision: 1}
	first, _ := m.Start(goal.Scope, "a", "first")
	other, _ := m.Start(goal.Scope, "b", "other-parent")
	binding, _ := BindChild(goal, first)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("workers not started")
		}
	}
	stopped := goal
	stopped.Revision++
	if err := CancelBoundChildren(ctx, m, goal, []ChildBinding{binding}); err != nil {
		t.Fatal(err)
	}
	child, _ := m.Check(goal.Scope, first.ID)
	remaining, _ := m.Check(goal.Scope, other.ID)
	if child.State != "canceled" || remaining.State != "running" {
		t.Fatal(child, remaining)
	}
	if _, err := AdmitChild(stopped, binding, child); !errors.Is(err, ErrParentResult) {
		t.Fatal("stopped goal admitted output")
	}
	if err := CancelBoundChildren(ctx, m, goal, []ChildBinding{binding}); err != nil {
		t.Fatal("cancel retry broke", err)
	}
	close(release)
	if child, err := m.Wait(ctx, goal.Scope, other.ID); err != nil || child.State != "completed" {
		t.Fatal(child, err)
	}
}

func TestCancelBindingPreflightAndWaitStopDoNotReleaseWorker(t *testing.T) {
	started := make(chan struct{})
	observedCancel := make(chan struct{})
	release := make(chan struct{})
	m, _ := tasks.NewManager(1, time.Second, func(taskCtx context.Context, _ tasks.Task) (string, error) {
		close(started)
		<-taskCtx.Done()
		close(observedCancel)
		<-release
		return "late", nil
	})
	// Release precedes Close during cleanup, including assertion failures.
	defer m.Close()
	defer close(release)
	goal := ParentGoal{ID: "report", Scope: tasks.Scope{Tenant: "demo", User: "reader"}, Revision: 1}
	child, _ := m.Start(goal.Scope, "a", "first")
	binding, _ := BindChild(goal, child)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("worker not started")
	}
	if err := CancelBoundChildren(ctx, m, goal, []ChildBinding{binding, binding}); !errors.Is(err, ErrParentResult) {
		t.Fatal(err)
	}
	current, _ := m.Check(goal.Scope, child.ID)
	if current.State != "running" {
		t.Fatal("invalid batch partially canceled", current)
	}
	waitCtx, stopWait := context.WithCancel(context.Background())
	defer stopWait()
	result := make(chan error, 1)
	go func() { result <- CancelBoundChildren(waitCtx, m, goal, []ChildBinding{binding}) }()
	select {
	case <-observedCancel:
	case <-ctx.Done():
		t.Fatal("child cancellation not observed")
	}
	stopWait()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("wait cancellation lost", err)
		}
	case <-ctx.Done():
		t.Fatal("wait did not stop")
	}

	current, _ = m.Check(goal.Scope, child.ID)
	if current.State != "canceling" {
		t.Fatal("noncooperating worker claimed stopped", current)
	}
	if _, err := m.Start(goal.Scope, "new", "another"); !errors.Is(err, tasks.ErrCapacity) {
		t.Fatal("slot released early", err)
	}
}
