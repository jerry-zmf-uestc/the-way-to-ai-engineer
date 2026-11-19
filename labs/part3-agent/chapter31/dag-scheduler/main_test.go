package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestDependencyConcurrencyAndInputOwnership(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	var mu sync.Mutex
	running, peak := 0, 0
	worker := func(id string) func(context.Context, map[string]string) (string, error) {
		return func(ctx context.Context, _ map[string]string) (string, error) {
			mu.Lock()
			running++
			peak = max(peak, running)
			mu.Unlock()
			started <- id
			select {
			case <-release:
			case <-ctx.Done():
				return "", ctx.Err()
			}
			mu.Lock()
			running--
			mu.Unlock()
			return id, nil
		}
	}
	nodes := []Node{{ID: "a", Run: worker("A")}, {ID: "b", Run: worker("B")}, {ID: "join", Depends: []string{"a", "b"}, Run: func(_ context.Context, input map[string]string) (string, error) {
		out := input["a"] + input["b"]
		input["a"] = "changed"
		return out, nil
	}}}
	type outcome struct {
		r   Report
		err error
	}
	done := make(chan outcome, 1)
	go func() { r, err := Run(context.Background(), nodes, 2, 4); done <- outcome{r, err} }()
	for n := 0; n < 2; n++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("parallel task not started")
		}
	}
	close(release)
	got := <-done
	if got.err != nil || got.r.Results["join"] != "AB" || got.r.Results["a"] != "A" || got.r.Peak != 2 || peak != 2 {
		t.Fatalf("report=%+v err=%v", got.r, got.err)
	}
}
func TestFailureBlocksDescendantsButIndependentCompletes(t *testing.T) {
	cause := errors.New("read failed")
	called := false
	nodes := []Node{{ID: "bad", Run: func(context.Context, map[string]string) (string, error) { return "", cause }}, {ID: "dependent", Depends: []string{"bad"}, Run: func(context.Context, map[string]string) (string, error) { called = true; return "", nil }}, {ID: "independent", Run: func(context.Context, map[string]string) (string, error) { return "valid", nil }}}
	r, err := Run(context.Background(), nodes, 2, 4)
	if !errors.Is(err, cause) || !errors.Is(err, ErrFailed) || called || r.States["dependent"] != "blocked" || r.Results["independent"] != "valid" {
		t.Fatalf("report=%+v err=%v", r, err)
	}
}
func TestInvalidAndBackpressureBeforeDispatch(t *testing.T) {
	calls := 0
	job := func(context.Context, map[string]string) (string, error) { calls++; return "", nil }
	invalid := []Node{{ID: "a", Depends: []string{"a"}, Run: job}}
	if _, err := Run(context.Background(), invalid, 2, 2); !errors.Is(err, ErrGraph) || calls != 0 {
		t.Fatal("invalid graph executed")
	}
	roots := []Node{{ID: "a", Run: job}, {ID: "b", Run: job}}
	r, err := Run(context.Background(), roots, 1, 1)
	if !errors.Is(err, ErrQueue) || calls != 0 || r.States["a"] != "canceled" {
		t.Fatalf("queue=%+v err=%v", r, err)
	}
}
func TestCancellationDrainsStartedWorkAndRejectsLateText(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := make(chan struct{})
	done := make(chan struct{})
	worker := func(ctx context.Context, _ map[string]string) (string, error) {
		close(start)
		<-ctx.Done()
		close(done)
		return "late", nil
	}
	results := make(chan Report, 1)
	go func() {
		r, err := Run(ctx, []Node{{ID: "a", Run: worker}}, 1, 2)
		if !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
		results <- r
	}()
	<-start
	cancel()
	select {
	case r := <-results:
		if len(r.Results) != 0 || r.States["a"] != "canceled" {
			t.Fatal("late output accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled scheduler blocked")
	}
	<-done
}
