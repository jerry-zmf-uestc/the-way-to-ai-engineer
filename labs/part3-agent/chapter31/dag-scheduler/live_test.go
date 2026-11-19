package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestLiveAdmissionGivesNewTenantNextEligibleTurn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	input := make(chan ReadyJob)
	send := func(job ReadyJob) {
		select {
		case input <- job:
		case <-ctx.Done():
			t.Fatal("submission did not finish")
		}
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	type result struct {
		report ResourceReport
		err    error
	}
	done := make(chan result, 1)
	go func() { r, e := RunReadyStream(ctx, input, map[string]int{"db": 1}, 1, 3, 4); done <- result{r, e} }()
	worker := func(context.Context) (string, error) { return "read", nil }
	send(ReadyJob{ID: "a1", Tenant: "a", Needs: []string{"db"}, Run: func(ctx context.Context) (string, error) {
		close(entered)
		select {
		case <-release:
			return "first", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}})
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("first worker missing")
	}
	send(ReadyJob{ID: "a2", Tenant: "a", Needs: []string{"db"}, Run: worker})
	send(ReadyJob{ID: "a3", Tenant: "a", Needs: []string{"db"}, Run: worker})
	send(ReadyJob{ID: "b1", Tenant: "b", Needs: []string{"db"}, Run: worker})
	close(input)
	close(release)
	select {
	case r := <-done:
		if r.err != nil || !reflect.DeepEqual(r.report.Dispatch, []string{"a1", "b1", "a2", "a3"}) || r.report.Free["db"] != 1 || len(r.report.Results) != 4 {
			t.Fatalf("report=%+v err=%v", r.report, r.err)
		}
	case <-ctx.Done():
		t.Fatal("stream did not finish")
	}
}

func TestLiveAdmissionFailureCancelsAndDrains(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	input := make(chan ReadyJob)
	send := func(job ReadyJob) {
		select {
		case input <- job:
		case <-ctx.Done():
			t.Fatal("submission did not finish")
		}
	}
	entered := make(chan struct{})
	exited := make(chan struct{})
	type result struct {
		report ResourceReport
		err    error
	}
	done := make(chan result, 1)
	go func() { r, e := RunReadyStream(ctx, input, map[string]int{"db": 1}, 1, 1, 4); done <- result{r, e} }()
	job := func(id string) ReadyJob {
		return ReadyJob{ID: id, Tenant: "a", Needs: []string{"db"}, Run: func(context.Context) (string, error) { return "read", nil }}
	}
	first := job("a1")
	first.Run = func(ctx context.Context) (string, error) {
		close(entered)
		<-ctx.Done()
		close(exited)
		return "late", nil
	}
	send(first)
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("worker missing")
	}
	send(job("a2"))
	send(job("a3"))
	close(input)
	select {
	case r := <-done:
		if !errors.Is(r.err, ErrQueue) || len(r.report.Results) != 0 || r.report.States["a1"] != "canceled" || r.report.States["a2"] != "canceled" || r.report.Free["db"] != 1 {
			t.Fatalf("report=%+v err=%v", r.report, r.err)
		}
		select {
		case <-exited:
		default:
			t.Fatal("returned before exit")
		}
	case <-ctx.Done():
		t.Fatal("drain missing")
	}
}

func TestLiveClosedInputAndFailureRelease(t *testing.T) {
	input := make(chan ReadyJob, 2)
	cause := errors.New("read failed")
	input <- ReadyJob{ID: "bad", Tenant: "a", Needs: []string{"db"}, Run: func(context.Context) (string, error) { return "partial", cause }}
	input <- ReadyJob{ID: "good", Tenant: "b", Needs: []string{"db"}, Run: func(context.Context) (string, error) { return "read", nil }}
	close(input)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, err := RunReadyStream(ctx, input, map[string]int{"db": 1}, 1, 2, 2)
	if !errors.Is(err, ErrFailed) || !errors.Is(err, cause) || r.Results["good"] != "read" || r.States["bad"] != "failed" || r.Free["db"] != 1 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	empty := make(chan ReadyJob)
	r, err = RunReadyStream(stopped, empty, map[string]int{"db": 1}, 1, 1, 1)
	if !errors.Is(err, context.Canceled) || len(r.Dispatch) != 0 || r.Free["db"] != 1 {
		t.Fatal("pre-cancel dispatched work")
	}
}

func TestLiveRejectsMalformedDuplicateAndTotalBudget(t *testing.T) {
	for _, mode := range []string{"unknown-resource", "duplicate-id", "total-budget"} {
		t.Run(mode, func(t *testing.T) {
			input := make(chan ReadyJob, 2)
			calls := 0
			run := func(context.Context) (string, error) { calls++; return "read", nil }
			first := ReadyJob{ID: "a", Tenant: "a", Needs: []string{"db"}, Run: run}
			maxTasks := 2
			if mode == "unknown-resource" {
				first.Needs = []string{"missing"}
			}
			input <- first
			if mode != "unknown-resource" {
				second := first
				if mode == "total-budget" {
					second.ID = "b"
					maxTasks = 1
				}
				input <- second
			}
			close(input)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			report, err := RunReadyStream(ctx, input, map[string]int{"db": 1}, 1, 2, maxTasks)
			want := ErrGraph
			if mode == "total-budget" {
				want = ErrQueue
			}
			if !errors.Is(err, want) || calls > 1 || report.Free["db"] != 1 {
				t.Fatalf("calls=%d report=%+v err=%v", calls, report, err)
			}
			if mode == "unknown-resource" && calls != 0 {
				t.Fatal("invalid task executed")
			}
		})
	}
}
