package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func costIdentity() RunIdentity { return RunIdentity{"demo", "reader", "run"} }
func costPrice() Price          { return Price{Model: "fixture", Version: "v1", Unit: "microcredit", Input: 1} }
func costRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestDurableCrashWorker(t *testing.T) {
	if os.Getenv("COST_DURABLE_WORKER") != "1" {
		t.Skip("helper process")
	}
	r := costRoot(t, os.Getenv("COST_DURABLE_DIR"))
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("COST_DURABLE_STAGE") == "after_reserve_attempt" {
		if _, err := d.ReserveAttempt("attempt-1", strings.Repeat("a", 64), costPrice(), Bound{Input: 50}); err != nil {
			t.Fatal(err)
		}
		os.Exit(93)
	}
	id, err := d.Reserve(costPrice(), Bound{Input: 50})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Begin(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("COST_DURABLE_STAGE") == "after_begin" {
		os.Exit(93)
	}
	f, err := r.OpenFile("calls.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.WriteString("read\n")
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("COST_DURABLE_STAGE") == "after_execute" {
		os.Exit(93)
	}
	if err := d.Settle(id, Usage{Input: 40}); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("COST_DURABLE_STAGE") == "after_settle" {
		os.Exit(93)
	}
	t.Fatal("unknown fault stage")
}

func TestDurableLedgerHardExitKeepsUncertaintyAndSettlement(t *testing.T) {
	for _, stage := range []string{"after_begin", "after_execute", "after_settle"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDurableCrashWorker$")
			cmd.Env = append(os.Environ(), "COST_DURABLE_WORKER=1", "COST_DURABLE_DIR="+dir, "COST_DURABLE_STAGE="+stage)
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 93 {
				t.Fatalf("worker: %v %s", err, output)
			}
			// Run waited for this owned child to exit before removing its stale lock.
			if err := os.Remove(filepath.Join(dir, "ledger.lock")); err != nil {
				t.Fatal(err)
			}
			r := costRoot(t, dir)
			d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			s, err := d.Snapshot()
			if err != nil || len(s.Entries) != 1 {
				t.Fatal(s, err)
			}
			wantCalls := 1
			if stage == "after_begin" {
				wantCalls = 0
			}
			log, logErr := os.ReadFile(filepath.Join(dir, "calls.log"))
			if (logErr != nil && !(wantCalls == 0 && errors.Is(logErr, os.ErrNotExist))) || strings.Count(string(log), "read\n") != wantCalls {
				t.Fatal(string(log), logErr)
			}
			if stage == "after_settle" {
				if s.Used != 40 || s.Held != 0 || s.Entries[0].State != "settled" {
					t.Fatal(s)
				}
			} else {
				if s.Used != 0 || s.Held != 50 || s.Entries[0].State != "unknown" {
					t.Fatal(s)
				}
				if err := d.Begin(context.Background(), 1); !errors.Is(err, ErrState) {
					t.Fatal("unknown call resent", err)
				}
				if _, err := d.Reserve(costPrice(), Bound{Input: 51}); !errors.Is(err, ErrBudget) {
					t.Fatal("unknown hold lost", err)
				}
			}
			known := Usage{Input: 40}
			if stage == "after_begin" {
				known = Usage{} // Controlled fixture confirms no simulated request ran.
			}
			if err := d.Settle(1, known); err != nil {
				t.Fatal(err)
			}
			if err := d.Settle(1, known); err != nil {
				t.Fatal(err)
			}
			s, err = d.Snapshot()
			if err != nil || s.Used != known.Input || s.Held != 0 {
				t.Fatal(s, err)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			again, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
			if err != nil {
				t.Fatal(err)
			}
			defer again.Close()
			if err := again.Settle(1, known); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDurableUnknownAndOverrunStayCommitted(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	id, err := d.Reserve(costPrice(), Bound{Input: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Begin(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := d.Unknown(id); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(id, Usage{Input: 110}); !errors.Is(err, ErrOverrun) {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s, err := d.Snapshot()
	if err != nil || s.Used != 110 || s.Held != 0 || !s.Frozen {
		t.Fatal(s, err)
	}
	if _, err := d.Reserve(costPrice(), Bound{}); !errors.Is(err, ErrBudget) {
		t.Fatal(err)
	}
}

func TestDurableLoadRejectsAmbiguousOrUnboundedState(t *testing.T) {
	for name, mutate := range map[string]func(string) string{
		"duplicate":   func(s string) string { return strings.TrimSuffix(s, "}") + `,"schema":1}` },
		"nested":      func(s string) string { return strings.Replace(s, `"Tenant":`, `"tenant":"ignored","Tenant":`, 1) },
		"unicode-key": func(s string) string { return strings.Replace(s, `"Schema"`, `"ſchema"`, 1) },
		"utf8":        func(s string) string { return strings.Replace(s, `"reader"`, "\"\xff\"", 1) },
		"extra":       func(s string) string { return s + `{}` },
		"oversize":    func(s string) string { return s + strings.Repeat(" ", 64<<10) },
		"deep":        func(string) string { return strings.Repeat("[", 10) + "0" + strings.Repeat("]", 10) },
	} {
		t.Run(name, func(t *testing.T) {
			r := costRoot(t, t.TempDir())
			d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := r.ReadFile("ledger.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.WriteFile("ledger.json", []byte(mutate(string(data))), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100); !errors.Is(err, ErrState) {
				t.Fatal("bad state resumed", err)
			}
			if _, err := r.Stat("ledger.lock"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed open retained new claim", err)
			}
		})
	}
}

func TestDurableConcurrentReservationsCommitWithinOneBudget(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	outcomes := make(chan error, 10)
	var workers sync.WaitGroup
	for range 10 {
		workers.Go(func() {
			<-start
			_, err := d.Reserve(costPrice(), Bound{Input: 30})
			outcomes <- err
		})
	}
	close(start)
	workers.Wait()
	close(outcomes)
	accepted := 0
	for err := range outcomes {
		if err == nil {
			accepted++
		} else if !errors.Is(err, ErrBudget) {
			t.Fatal("concurrent persistence failed", err)
		}
	}
	if accepted != 3 {
		t.Fatal("concurrency exceeded budget", accepted)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s, err := d.Snapshot()
	if err != nil || s.Held != 90 || len(s.Entries) != 3 {
		t.Fatal("restart lost committed reservations", s, err)
	}
}

func TestDurablePolicyCancellationAndExclusiveOwner(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	id, err := d.Reserve(costPrice(), Bound{Input: 50})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.Begin(ctx, id); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	for _, config := range []struct {
		id    RunIdentity
		limit int64
	}{{RunIdentity{"other", "reader", "run"}, 100}, {costIdentity(), 101}} {
		if _, err := OpenDurableLedger(r, config.id, "microcredit", config.limit); !errors.Is(err, ErrState) {
			t.Fatal(config, err)
		}
	}
	d, err = OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s, err := d.Snapshot()
	if err != nil || s.Held != 0 || s.Entries[0].State != "not_sent" {
		t.Fatal(s, err)
	}
}

func TestDurableWriteFailureStopsAdmissionAndKeepsOldState(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile("ledger.tmp", []byte("retained candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	if id, err := d.Reserve(costPrice(), Bound{Input: 50}); err == nil || id != 0 {
		t.Fatal("uncommitted reservation acknowledged", id, err)
	}
	if _, err := d.Snapshot(); !errors.Is(err, ErrState) {
		t.Fatal("unconfirmed memory exposed", err)
	}
	if _, err := d.Reserve(costPrice(), Bound{}); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	state, err := readCostState(r)
	if err != nil || state.Ledger.Held != 0 || len(state.Ledger.Entries) != 0 {
		t.Fatal(state, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100); err == nil {
		t.Fatal("residual candidate promoted")
	}
}
