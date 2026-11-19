package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

func TestUpdateWithoutAcknowledgementQueriesCurrentState(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	m, err := tasks.NewManager(1, time.Minute, func(ctx context.Context, task tasks.Task) (string, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-ctx.Done()
			<-release
			return "old", ctx.Err()
		}
		return "evidence: " + task.Input, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	defer unblock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	scope := tasks.Scope{Tenant: "demo", User: "reader"}
	initial, err := m.Start(scope, "request-1", "old")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Discard acknowledgements; these are local state-contract checks, not network fault injection.
	if _, err := m.Update(scope, initial.ID, initial.Revision, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Update(scope, initial.ID, initial.Revision, "new"); !errors.Is(err, tasks.ErrConflict) {
		t.Fatal("stale update repeated", err)
	}
	current, err := m.Check(scope, initial.ID)
	if err != nil || current.State != "updating" || current.Revision != 2 || current.Input != "new" || current.Attempt != 1 || calls.Load() != 1 {
		t.Fatal(current, err, calls.Load())
	}
	retried, err := m.Start(scope, "request-1", "old")
	if err != nil || retried.ID != initial.ID || retried != current {
		t.Fatal("start retry created or reset task", retried, err)
	}
	if _, err := m.Start(scope, "request-1", "new"); !errors.Is(err, tasks.ErrConflict) {
		t.Fatal("initial request identity rewritten", err)
	}
	unblock()
	finished, err := m.Wait(ctx, scope, initial.ID)
	if err != nil || finished.State != "completed" || finished.Output != "evidence: new" || finished.Attempt != 2 || finished.Revision != 2 || calls.Load() != 2 {
		t.Fatal(finished, err, calls.Load())
	}
}

func TestCancelWithoutAcknowledgementDoesNotRepeatExecution(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	m, err := tasks.NewManager(1, time.Minute, func(ctx context.Context, _ tasks.Task) (string, error) {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		<-release
		return "late", ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	defer unblock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	scope := tasks.Scope{Tenant: "demo", User: "reader"}
	initial, err := m.Start(scope, "request-1", "old")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, err := m.Cancel(scope, initial.ID, initial.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Cancel(scope, initial.ID, initial.Revision); !errors.Is(err, tasks.ErrConflict) {
		t.Fatal("old version silently accepted", err)
	}
	current, err := m.Check(scope, initial.ID)
	if err != nil || current.State != "canceling" || current.Revision != 2 {
		t.Fatal(current, err)
	}
	duplicate, err := m.Cancel(scope, initial.ID, current.Revision)
	if err != nil || duplicate != current {
		t.Fatal("canceling duplicate changed state", duplicate, err)
	}
	if _, err := m.Start(scope, "another", "new"); !errors.Is(err, tasks.ErrCapacity) {
		t.Fatal("released live worker capacity", err)
	}
	unblock()
	finished, err := m.Wait(ctx, scope, initial.ID)
	if err != nil || finished.State != "canceled" || finished.Output != "" || finished.Attempt != 1 || finished.Revision != 2 || calls.Load() != 1 {
		t.Fatal(finished, err, calls.Load())
	}
}
