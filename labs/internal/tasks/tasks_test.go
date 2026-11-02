package tasks

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

var alice = Scope{"demo", "alice"}

func manager(t *testing.T, limit int, w Worker) *Manager {
	t.Helper()
	m, err := NewManager(limit, time.Second, w)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}
func wait(t *testing.T, m *Manager, task Task) Task {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := m.Wait(ctx, alice, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestDuplicateScopeAndSessionIndependence(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	m := manager(t, 1, func(ctx context.Context, t Task) (string, error) {
		close(entered)
		select {
		case <-release:
			return t.Input, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	task, err := m.Start(alice, "key", "timeout")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	duplicate, err := m.Start(alice, "key", "timeout")
	if err != nil || duplicate.ID != task.ID {
		t.Fatalf("duplicate %v %v", duplicate, err)
	}
	if _, err = m.Start(alice, "key", "retry"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = m.Check(Scope{"demo", "bob"}, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	// The main conversation can disappear: status and results are still looked up by trusted Scope/ID.
	if len(m.List(alice)) != 1 || len(m.List(Scope{"other", "alice"})) != 0 {
		t.Fatal("list scope")
	}
	close(release)
	result := wait(t, m, task)
	if result.State != "completed" || result.Output != "timeout" {
		t.Fatal(result)
	}
}
func TestUpdateDrainsAndRejectsLateOutput(t *testing.T) {
	started := make(chan Task, 2)
	releaseOld := make(chan struct{})
	m := manager(t, 1, func(ctx context.Context, t Task) (string, error) {
		started <- t
		if t.Revision == 1 {
			<-ctx.Done()
			<-releaseOld
			return "stale answer", nil
		}
		return t.Input, nil
	})
	task, _ := m.Start(alice, "key", "old")
	<-started
	updated, err := m.Update(alice, task.ID, 1, "new")
	if err != nil || updated.Revision != 2 || updated.State != "updating" {
		t.Fatal(updated, err)
	}
	if _, err = m.Update(alice, task.ID, 1, "again"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = m.Start(alice, "another", "x"); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	select {
	case <-started:
		t.Fatal("overlapping attempts")
	default:
	}
	close(releaseOld)
	current := <-started
	if current.Revision != 2 || current.Input != "new" {
		t.Fatal(current)
	}
	result := wait(t, m, task)
	if result.State != "completed" || result.Output != "new" || result.Attempt != 2 {
		t.Fatal(result)
	}
	if _, err = m.Update(alice, task.ID, result.Revision, "too late"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
func TestCancelKeepsSlotUntilDrain(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	m := manager(t, 1, func(ctx context.Context, t Task) (string, error) {
		close(entered)
		<-ctx.Done()
		<-release
		return "late", nil
	})
	task, _ := m.Start(alice, "key", "x")
	<-entered
	canceled, err := m.Cancel(alice, task.ID, 1)
	if err != nil || canceled.State != "canceling" {
		t.Fatal(canceled, err)
	}
	if _, err = m.Start(alice, "next", "x"); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	if _, err = m.Cancel(alice, task.ID, 1); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = m.Cancel(alice, task.ID, canceled.Revision); err != nil {
		t.Fatal(err)
	}
	close(release)
	result := wait(t, m, task)
	if result.State != "canceled" || result.Output != "" {
		t.Fatal(result)
	}
}
func TestCommitAndUpdateRace(t *testing.T) {
	for range 30 {
		entered := make(chan struct{})
		release := make(chan struct{})
		m, _ := NewManager(1, time.Second, func(ctx context.Context, t Task) (string, error) {
			if t.Revision == 1 {
				close(entered)
				<-release
			}
			return t.Input, nil
		})
		task, _ := m.Start(alice, "key", "old")
		<-entered
		changed := make(chan error, 1)
		go func() { _, err := m.Update(alice, task.ID, 1, "new"); changed <- err }()
		close(release)
		err := <-changed
		result := wait(t, m, task)
		if err == nil {
			if result.Output != "new" || result.Revision != 2 {
				t.Fatal(result)
			}
		} else if errors.Is(err, ErrConflict) {
			if result.Output != "old" || result.Revision != 1 {
				t.Fatal(result)
			}
		} else {
			t.Fatal(err)
		}
		m.Close()
	}
}
func TestDeadlineFailureAndCleanup(t *testing.T) {
	m, err := NewManager(1, 10*time.Millisecond, func(ctx context.Context, t Task) (string, error) { <-ctx.Done(); return "late", nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	task, _ := m.Start(alice, "key", "x")
	result := wait(t, m, task)
	if result.State != "failed" || result.Failure != "deadline_exceeded" || result.Output != "" {
		t.Fatal(result)
	}
	n, err := m.Cleanup(result.FinishedAt.Add(time.Second), time.Second)
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err = m.Check(alice, task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	next, err := m.Start(alice, "key", "x")
	if err != nil || next.ID == task.ID {
		t.Fatal(next, err)
	}
}
func TestUpdateBeforeFirstAttempt(t *testing.T) {
	for range 30 {
		var calls atomic.Int32
		m := manager(t, 1, func(ctx context.Context, t Task) (string, error) {
			calls.Add(1)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
				return t.Input, nil
			}
		})
		task, _ := m.Start(alice, "key", "old")
		_, err := m.Update(alice, task.ID, 1, "new")
		result := wait(t, m, task)
		if result.Attempt != int(calls.Load()) {
			t.Fatal("attempt count differs from callback entries", result, calls.Load())
		}
		if err == nil && (result.Output != "new" || result.Attempt < 1 || result.Attempt > 2) {
			t.Fatal(result)
		}
		if err != nil && (!errors.Is(err, ErrConflict) || result.Output != "old") {
			t.Fatal(result, err)
		}
		m.Close()
	}
}
func TestWorkerFailuresAndShutdown(t *testing.T) {
	for _, w := range []Worker{func(context.Context, Task) (string, error) { return "", nil }, func(context.Context, Task) (string, error) { return "text", errors.New("private address") }} {
		m := manager(t, 1, w)
		task, _ := m.Start(alice, "key", "x")
		result := wait(t, m, task)
		if result.State != "failed" || result.Output != "" || result.Failure == "private address" {
			t.Fatal(result)
		}
		m.Close()
		if _, err := m.Start(alice, "next", "x"); !errors.Is(err, ErrClosed) {
			t.Fatal(err)
		}
	}
	if _, err := NewManager(0, time.Second, func(context.Context, Task) (string, error) { return "x", nil }); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
}

func TestRetainedCapacityAndCancelDuringUpdate(t *testing.T) {
	m := manager(t, 1, func(context.Context, Task) (string, error) { return "ok", nil })
	for i := range 16 {
		task, err := m.Start(alice, fmt.Sprintf("k-%d", i), "x")
		if err != nil {
			t.Fatal(err)
		}
		wait(t, m, task)
	}
	if _, err := m.Start(alice, "full", "x"); !errors.Is(err, ErrCapacity) {
		t.Fatal(err)
	}
	if n, err := m.Cleanup(time.Now().Add(time.Hour), time.Second); err != nil || n != 16 {
		t.Fatal(n, err)
	}
	if _, err := m.Start(alice, "full", "x"); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	updated := manager(t, 1, func(ctx context.Context, t Task) (string, error) {
		close(entered)
		<-ctx.Done()
		<-release
		return "late", nil
	})
	task, _ := updated.Start(alice, "key", "old")
	<-entered
	next, err := updated.Update(alice, task.ID, 1, "new")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = updated.Cancel(alice, task.ID, next.Revision); err != nil {
		t.Fatal(err)
	}
	close(release)
	result := wait(t, updated, task)
	if result.State != "canceled" || result.Attempt != 1 || result.Output != "" {
		t.Fatal(result)
	}
}
