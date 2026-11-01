package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func readyJobs() []ReadyJob {
	jobs := []ReadyJob{}
	for _, id := range []string{"a1", "a2", "a3", "b1", "b2"} {
		jobs = append(jobs, ReadyJob{ID: id, Tenant: id[:1], Needs: []string{"db"}, Run: func(context.Context) (string, error) { return "read", nil }})
	}
	return jobs
}
func TestResourceRoundRobinAndSameFIFOInput(t *testing.T) {
	jobs := readyJobs()
	capacity := map[string]int{"db": 1}
	fifo, err := RunReady(context.Background(), jobs, capacity, 2, 3, false)
	if err != nil {
		t.Fatal(err)
	}
	rr, err := RunReady(context.Background(), jobs, capacity, 2, 3, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fifo.Dispatch, []string{"a1", "a2", "a3", "b1", "b2"}) || !reflect.DeepEqual(rr.Dispatch, []string{"a1", "b1", "a2", "b2", "a3"}) {
		t.Fatal(fifo.Dispatch, rr.Dispatch)
	}
	if len(rr.Results) != 5 || rr.Peak != 1 || rr.Free["db"] != 1 || !reflect.DeepEqual(fifo.Results, rr.Results) {
		t.Fatal("capacity/results diverged", rr)
	}
	if rejected, err := RunReady(context.Background(), jobs, capacity, 2, 2, true); !errors.Is(err, ErrQueue) || len(rejected.States) != 0 {
		t.Fatal("tenant admission unbounded", err)
	}
}
func TestResourceReservationAllOrNone(t *testing.T) {
	pool := resourcePool{free: map[string]int{"db": 1, "cpu": 0}, held: map[string][]string{}}
	if pool.take(ReadyJob{ID: "x", Needs: []string{"db", "cpu"}}) || pool.free["db"] != 1 || len(pool.held) != 0 {
		t.Fatal("partial resource held")
	}
	pool.free["cpu"] = 1
	if !pool.take(ReadyJob{ID: "x", Needs: []string{"db", "cpu"}}) {
		t.Fatal("eligible denied")
	}
	pool.release("x")
	pool.release("x")
	if pool.free["db"] != 1 || pool.free["cpu"] != 1 {
		t.Fatal("release leak/double release")
	}
}

func TestResourcePoliciesTreatBlockedTenantHeadDifferently(t *testing.T) {
	for _, rr := range []bool{false, true} {
		t.Run(map[bool]string{false: "eligible scan", true: "tenant head round robin"}[rr], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			started := make(chan string, 4)
			job := func(id, tenant, resource string) ReadyJob {
				return ReadyJob{ID: id, Tenant: tenant, Needs: []string{resource}, Run: func(ctx context.Context) (string, error) {
					started <- id
					<-ctx.Done()
					return "late", nil
				}}
			}
			jobs := []ReadyJob{job("a-hold", "a", "db"), job("a-wait", "a", "db"), job("a-small", "a", "cpu"), job("b-small", "b", "cpu")}
			done := make(chan struct{})
			var out ResourceReport
			var err error
			go func() { defer close(done); out, err = RunReady(ctx, jobs, map[string]int{"db": 1, "cpu": 1}, 2, 3, rr) }()
			t.Cleanup(func() { cancel(); <-done })
			observed := map[string]bool{}
			for n := 0; n < 2; n++ {
				select {
				case id := <-started:
					observed[id] = true
				case <-ctx.Done():
					t.Fatal("second resource dispatch never started")
				}
			}
			want := []string{"a-hold", "a-small"}
			if rr {
				want = []string{"a-hold", "b-small"}
			}
			if !observed[want[0]] || !observed[want[1]] {
				t.Fatal("queue-head policy changed", observed, want)
			}
			cancel()
			<-done
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(out.Dispatch, want) || out.Free["db"] != 1 || out.Free["cpu"] != 1 || len(out.Results) != 0 {
				t.Fatal(out, err)
			}
		})
	}
}
func TestResourceCancelDrainAndFailureRelease(t *testing.T) {
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobs := []ReadyJob{{ID: "active", Tenant: "a", Needs: []string{"db", "cpu"}, Run: func(ctx context.Context) (string, error) { close(started); <-ctx.Done(); return "late", nil }}, {ID: "queued", Tenant: "b", Needs: []string{"db"}, Run: func(context.Context) (string, error) { t.Error("queued started after cancel"); return "", nil }}}
	type result struct {
		out ResourceReport
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := RunReady(ctx, jobs, map[string]int{"db": 1, "cpu": 1}, 2, 2, true)
		done <- result{out, err}
	}()
	<-started
	cancel()
	r := <-done
	if !errors.Is(r.err, context.Canceled) || len(r.out.Results) != 0 || r.out.States["active"] != "canceled" || r.out.States["queued"] != "canceled" || r.out.Free["db"] != 1 || r.out.Free["cpu"] != 1 {
		t.Fatal(r)
	}
	cause := errors.New("read failed")
	jobs[0].Run = func(context.Context) (string, error) { return "residual", cause }
	jobs[1].Run = func(context.Context) (string, error) { return "valid", nil }
	out, err := RunReady(context.Background(), jobs, map[string]int{"db": 1, "cpu": 1}, 2, 2, true)
	if !errors.Is(err, cause) || out.States["active"] != "failed" || out.Results["queued"] != "valid" || out.Free["db"] != 1 || out.Free["cpu"] != 1 {
		t.Fatal(out, err)
	}
}
func TestAcyclicTasksCanHaveResourceCycle(t *testing.T) {
	// Independent tasks, but each holds one resource and waits for the other's.
	db, cpu := make(chan struct{}, 1), make(chan struct{}, 1)
	db <- struct{}{}
	cpu <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	acquired, waiting, done := make(chan struct{}, 2), make(chan struct{}, 2), make(chan error, 2)
	proceed := make(chan struct{})
	worker := func(own, other chan struct{}) {
		<-own
		defer func() { own <- struct{}{} }()
		acquired <- struct{}{}
		<-proceed
		waiting <- struct{}{}
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-other:
			err = ctx.Err()
			other <- struct{}{}
		}
		done <- err
	}
	go worker(db, cpu)
	go worker(cpu, db)
	<-acquired
	<-acquired
	close(proceed)
	<-waiting
	<-waiting
	select {
	case <-done:
		t.Fatal("resource cycle unexpectedly completed")
	default:
	}
	cancel()
	for n := 0; n < 2; n++ {
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	// Receiving both resources also synchronizes deferred release after done sends.
	<-db
	<-cpu
}
