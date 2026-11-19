package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

func TestOldChildCompletionCannotFillRevisedParentGoal(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	m, err := tasks.NewManager(1, time.Second, func(ctx context.Context, task tasks.Task) (string, error) {
		close(entered)
		select {
		case <-release:
			return "old-scope evidence", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	goal := ParentGoal{ID: "report", Scope: tasks.Scope{Tenant: "demo", User: "reader"}, Revision: 1}
	child, err := m.Start(goal.Scope, "request", "all years")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := BindChild(goal, child)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("worker not started")
	}
	oldGoal := goal
	goal.Revision++
	close(release)
	result, err := m.Wait(ctx, goal.Scope, child.ID)
	if err != nil || result.State != "completed" {
		t.Fatal(result, err)
	}
	if output, err := AdmitChild(goal, binding, result); !errors.Is(err, ErrParentResult) || output != "" {
		t.Fatal("late old result admitted", output, err)
	}
	if output, err := AdmitChild(oldGoal, binding, result); err != nil || output != result.Output {
		t.Fatal("original goal lost its result", output, err)
	}
	for _, mutate := range []func(*tasks.Task){
		func(r *tasks.Task) { r.ID = "other-child" },
		func(r *tasks.Task) { r.Scope.User = "other" },
		func(r *tasks.Task) { r.Revision++ },
		func(r *tasks.Task) { r.State = "running" },
		func(r *tasks.Task) { r.Failure = "worker_failed" },
	} {
		bad := result
		mutate(&bad)
		if _, err := AdmitChild(oldGoal, binding, bad); !errors.Is(err, ErrParentResult) {
			t.Fatal("invalid child admitted", bad, err)
		}
	}
}

func TestUpdatedChildRequiresNewVersionBinding(t *testing.T) {
	entered := make(chan struct{})
	m, err := tasks.NewManager(1, time.Second, func(ctx context.Context, task tasks.Task) (string, error) {
		if task.Revision == 1 {
			close(entered)
			<-ctx.Done()
			return "", ctx.Err()
		}
		return "evidence: " + task.Input, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	goal := ParentGoal{ID: "report", Scope: tasks.Scope{Tenant: "demo", User: "reader"}, Revision: 1}
	child, err := m.Start(goal.Scope, "request", "all years")
	if err != nil {
		t.Fatal(err)
	}
	oldBinding, _ := BindChild(goal, child)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("worker not started")
	}
	updated, err := m.Update(goal.Scope, child.ID, child.Revision, "this year")
	if err != nil {
		t.Fatal(err)
	}
	if err := CancelBoundChildren(ctx, m, goal, []ChildBinding{oldBinding}); !errors.Is(err, tasks.ErrConflict) {
		t.Fatal("old binding canceled updated child", err)
	}
	newBinding, err := BindChild(goal, updated)
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Wait(ctx, goal.Scope, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdmitChild(goal, oldBinding, result); !errors.Is(err, ErrParentResult) {
		t.Fatal("old child binding accepted", err)
	}
	if output, err := AdmitChild(goal, newBinding, result); err != nil || output != "evidence: this year" {
		t.Fatal(output, err)
	}
}
