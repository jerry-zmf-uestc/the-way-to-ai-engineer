package tasks

import (
	"context"
	"testing"
	"time"
)

func TestAlreadyStoppedBeforeWorkerEntry(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline", "update"} {
		t.Run(mode, func(t *testing.T) {
			calls := []Task{}
			m := manager(t, 1, func(_ context.Context, task Task) (string, error) {
				calls = append(calls, task)
				return task.Input, nil
			})
			deadline := time.Now().Add(time.Second)
			if mode == "deadline" {
				deadline = time.Now().Add(-time.Second)
			}
			life, stop := context.WithDeadline(context.Background(), deadline)
			defer stop()
			ctx, cancel := context.WithCancel(life)
			snapshot := Task{ID: "entry", Scope: alice, Input: "old", Revision: 1, State: "running"}
			r := &record{task: snapshot, cancel: cancel, life: life, stop: stop, done: make(chan struct{})}
			if mode == "cancel" {
				r.task.State = "canceling"
				r.task.Revision++
				cancel()
			} else if mode == "update" {
				r.task.State = "updating"
				r.task.Revision++
				r.task.Input = "new"
				cancel()
			}
			// Establish the same ownership bookkeeping as Start, then test the entry boundary directly.
			m.records[snapshot.ID] = r
			m.active = 1
			m.wg.Add(1)
			m.run(ctx, r, snapshot)
			final := wait(t, m, snapshot)
			if mode == "update" {
				if len(calls) != 1 || calls[0].Revision != 2 || final.Attempt != 1 || final.Output != "new" || final.State != "completed" {
					t.Fatal("invalidated old attempt entered worker", calls, final)
				}
			} else if len(calls) != 0 || final.Attempt != 0 || final.Output != "" {
				t.Fatal("already stopped task entered worker", calls, final)
			}
			if mode == "deadline" && (final.State != "failed" || final.Failure != "deadline_exceeded") {
				t.Fatal("deadline category lost", final)
			}
			if mode == "cancel" && final.State != "canceled" {
				t.Fatal("cancel category lost", final)
			}
			if m.active != 0 {
				t.Fatal("slot not released")
			}
		})
	}
}
