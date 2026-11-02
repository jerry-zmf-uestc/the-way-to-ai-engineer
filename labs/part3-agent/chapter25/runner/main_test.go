package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func policy() Policy {
	return Policy{MaxModelCalls: 3, MaxToolCalls: 2, MaxTokens: 20, MaxOutputBytes: 128, MaxAttempts: 2}
}
func checkTerminal(t *testing.T, r Report, state State) {
	t.Helper()
	if r.State != state {
		t.Fatalf("state=%s want %s", r.State, state)
	}
	n := 0
	for i, e := range r.Events {
		if e.Seq != i+1 {
			t.Fatal("event order lost")
		}
		if e.Kind == "run_finished" {
			n++
			if e.State != state || i != len(r.Events)-1 {
				t.Fatal("terminal event inconsistent")
			}
		}
	}
	if n != 1 {
		t.Fatalf("terminal events=%d", n)
	}
}
func TestRetriesConsumeBudgetAndUsage(t *testing.T) {
	calls := 0
	job := Job{Name: "model", Kind: "model", RetrySafe: true, Execute: func(context.Context) (Result, error) {
		calls++
		if calls == 1 {
			return Result{Tokens: 2}, fmt.Errorf("provider: %w", ErrTemporary)
		}
		return Result{Text: "ok", Tokens: 3}, nil
	}}
	r, err := Run(context.Background(), policy(), []Job{job})
	if err != nil || calls != 2 || r.ModelCalls != 2 || r.Tokens != 5 || len(r.Outputs) != 1 || r.Outputs[0] != "ok" {
		t.Fatalf("report=%+v calls=%d err=%v", r, calls, err)
	}
	checkTerminal(t, r, Completed)
	p := policy()
	p.MaxModelCalls = 1
	calls = 0
	r, err = Run(context.Background(), p, []Job{job})
	if !errors.Is(err, ErrBudget) || calls != 1 || r.Tokens != 2 || len(r.Outputs) != 0 {
		t.Fatalf("budget report=%+v calls=%d err=%v", r, calls, err)
	}
	checkTerminal(t, r, Failed)
}
func TestRetryRequiresBothConditions(t *testing.T) {
	for _, c := range []struct {
		name  string
		safe  bool
		cause error
		want  int
	}{
		{"unsafe", false, ErrTemporary, 1}, {"permanent", true, ErrInvalid, 1}, {"temporary", true, ErrTemporary, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			calls := 0
			r, err := Run(context.Background(), policy(), []Job{{Name: "read", Kind: "tool", RetrySafe: c.safe, Execute: func(context.Context) (Result, error) { calls++; return Result{}, c.cause }}})
			if !errors.Is(err, c.cause) || calls != c.want || r.ToolCalls != c.want {
				t.Fatalf("calls=%d report=%+v err=%v", calls, r, err)
			}
			checkTerminal(t, r, Failed)
		})
	}
}
func TestCancellationRejectsLateResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := Run(ctx, policy(), []Job{{Name: "read", Kind: "tool", Execute: func(context.Context) (Result, error) { cancel(); return Result{Text: "late", Tokens: 2}, nil }}})
	if !errors.Is(err, context.Canceled) || len(r.Outputs) != 0 || r.Tokens != 2 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	checkTerminal(t, r, Canceled)
}
func TestCancellationDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := policy()
	p.Backoff = time.Hour
	started := make(chan struct{})
	done := make(chan struct{})
	var r Report
	var err error
	go func() {
		defer close(done)
		r, err = Run(ctx, p, []Job{{Name: "read", Kind: "tool", RetrySafe: true, Execute: func(context.Context) (Result, error) { close(started); return Result{}, ErrTemporary }}})
	}()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked")
	}
	if !errors.Is(err, context.Canceled) || r.ToolCalls != 1 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	checkTerminal(t, r, Canceled)
}
func TestWrappedDeadlineStopsWithoutRetry(t *testing.T) {
	r, err := Run(context.Background(), policy(), []Job{{Name: "read", Kind: "tool", RetrySafe: true, Execute: func(context.Context) (Result, error) {
		return Result{}, fmt.Errorf("child: %w", context.DeadlineExceeded)
	}}})
	if !errors.Is(err, context.DeadlineExceeded) || r.ToolCalls != 1 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	checkTerminal(t, r, Canceled)
}
func TestBudgetsPreventNextAction(t *testing.T) {
	for _, c := range []struct {
		name  string
		limit func(*Policy)
		first Result
	}{
		{"tool calls", func(p *Policy) { p.MaxToolCalls = 1 }, Result{}},
		{"tokens exhausted", func(p *Policy) { p.MaxTokens = 2 }, Result{Tokens: 2}},
		{"usage overshoot", func(p *Policy) { p.MaxTokens = 1 }, Result{Tokens: 2}},
		{"oversize output", func(p *Policy) { p.MaxOutputBytes = 1 }, Result{Text: "long"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := policy()
			c.limit(&p)
			calls := 0
			job := Job{Name: "read", Kind: "tool", Execute: func(context.Context) (Result, error) { calls++; return c.first, nil }}
			r, err := Run(context.Background(), p, []Job{job, job})
			if !errors.Is(err, ErrBudget) || calls != 1 {
				t.Fatalf("calls=%d report=%+v err=%v", calls, r, err)
			}
			checkTerminal(t, r, Failed)
		})
	}
}
func TestValidationAndInitialCancellation(t *testing.T) {
	calls := 0
	job := Job{Name: "read", Kind: "tool", Execute: func(context.Context) (Result, error) { calls++; return Result{}, nil }}
	p := policy()
	p.MaxAttempts = 0
	r, err := Run(context.Background(), p, []Job{job})
	if !errors.Is(err, ErrInvalid) || calls != 0 {
		t.Fatal("invalid policy executed")
	}
	checkTerminal(t, r, Failed)
	r, err = Run(context.Background(), policy(), []Job{{Name: "bad", Kind: "other"}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid job accepted")
	}
	checkTerminal(t, r, Failed)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err = Run(ctx, policy(), []Job{job})
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("canceled run executed")
	}
	checkTerminal(t, r, Canceled)
	r, err = Run(context.Background(), policy(), []Job{{Name: "bad usage", Kind: "model", Execute: func(context.Context) (Result, error) { return Result{Tokens: -1}, nil }}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("negative usage accepted")
	}
	checkTerminal(t, r, Failed)
}
func TestConcurrentRunsOwnTheirReports(t *testing.T) {
	for i := 0; i < 8; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			r, err := Run(context.Background(), policy(), []Job{{Name: "read", Kind: "tool", Execute: func(context.Context) (Result, error) { return Result{Text: "own"}, nil }}})
			if err != nil || r.ToolCalls != 1 {
				t.Fatalf("report=%+v err=%v", r, err)
			}
			checkTerminal(t, r, Completed)
		})
	}
}
