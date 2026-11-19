package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAttemptReservationIsIdempotentAcrossReopen(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	id, err := d.ReserveAttempt("attempt-1", hash, costPrice(), Bound{Input: 50})
	if err != nil || id != 1 {
		t.Fatal(id, err)
	}
	if err := d.Begin(context.Background(), id); err != nil {
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
	again, err := d.ReserveAttempt("attempt-1", hash, costPrice(), Bound{Input: 50})
	if err != nil || again != id {
		t.Fatal(again, err)
	}
	s, err := d.Snapshot()
	if err != nil || s.Held != 50 || len(s.Entries) != 1 || s.Entries[0].State != "unknown" {
		t.Fatal(s, err)
	}
	if err := d.Settle(id, Usage{Input: 40}); err != nil {
		t.Fatal(err)
	}
	if again, err := d.ReserveAttempt("attempt-1", hash, costPrice(), Bound{Input: 50}); err != nil || again != id {
		t.Fatal(again, err)
	}
	if next, err := d.ReserveAttempt("attempt-2", hash, costPrice(), Bound{Input: 50}); err != nil || next != 2 {
		t.Fatal(next, err)
	}
	s, err = d.Snapshot()
	if err != nil || s.Used != 40 || s.Held != 50 || len(s.Entries) != 2 {
		t.Fatal(s, err)
	}
}

func TestAttemptConflictRejectsChangedRequestOrAccounting(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	hash := strings.Repeat("a", 64)
	if _, err := d.ReserveAttempt("attempt-1", hash, costPrice(), Bound{Input: 50}); err != nil {
		t.Fatal(err)
	}
	changedPrice := costPrice()
	changedPrice.Version = "v2"
	for _, request := range []struct {
		key, hash string
		price     Price
		bound     Bound
	}{
		{"attempt-1", strings.Repeat("b", 64), costPrice(), Bound{Input: 50}},
		{"attempt-1", hash, changedPrice, Bound{Input: 50}},
		{"attempt-1", hash, costPrice(), Bound{Input: 51}},
		{" ", hash, costPrice(), Bound{Input: 50}},
		{"attempt-2", strings.Repeat("A", 64), costPrice(), Bound{Input: 50}},
	} {
		if id, err := d.ReserveAttempt(request.key, request.hash, request.price, request.bound); !errors.Is(err, ErrState) || id != 0 {
			t.Fatal(request, id, err)
		}
	}
	s, err := d.Snapshot()
	if err != nil || s.Held != 50 || len(s.Entries) != 1 {
		t.Fatal(s, err)
	}
}

func TestAttemptLookupNeverCreatesReservation(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	hash := strings.Repeat("a", 64)
	if _, err := d.LookupAttempt("missing", hash, costPrice(), Bound{Input: 50}); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	s, err := d.Snapshot()
	if err != nil || s.Held != 0 || len(s.Entries) != 0 {
		t.Fatal(s, err)
	}
	id, err := d.ReserveAttempt("known", hash, costPrice(), Bound{Input: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := d.LookupAttempt("known", hash, costPrice(), Bound{Input: 50}); err != nil || got != id {
		t.Fatal(got, err)
	}
	if _, err := d.LookupAttempt("known", strings.Repeat("b", 64), costPrice(), Bound{Input: 50}); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	s, err = d.Snapshot()
	if err != nil || s.Held != 50 || len(s.Entries) != 1 {
		t.Fatal(s, err)
	}
}

func TestConcurrentPreparationUsesOneAttemptEntry(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	start := make(chan struct{})
	ids := make(chan int, 10)
	errs := make(chan error, 10)
	var workers sync.WaitGroup
	for range 10 {
		workers.Go(func() {
			<-start
			id, err := d.ReserveAttempt("attempt-1", strings.Repeat("a", 64), costPrice(), Bound{Input: 50})
			ids <- id
			errs <- err
		})
	}
	close(start)
	workers.Wait()
	close(ids)
	close(errs)
	for id := range ids {
		if id != 1 {
			t.Fatal(id)
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := d.Snapshot()
	if err != nil || s.Held != 50 || len(s.Entries) != 1 {
		t.Fatal(s, err)
	}
}

func TestAttemptBindingSurvivesHardExit(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDurableCrashWorker$")
	cmd.Env = append(os.Environ(), "COST_DURABLE_WORKER=1", "COST_DURABLE_DIR="+dir, "COST_DURABLE_STAGE=after_reserve_attempt")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 93 {
		t.Fatalf("worker %v %s", err, output)
	}
	if err := os.Remove(filepath.Join(dir, "ledger.lock")); err != nil {
		t.Fatal(err)
	}
	r := costRoot(t, dir)
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if id, err := d.ReserveAttempt("attempt-1", strings.Repeat("a", 64), costPrice(), Bound{Input: 50}); err != nil || id != 1 {
		t.Fatal(id, err)
	}
	s, err := d.Snapshot()
	if err != nil || s.Held != 50 || len(s.Entries) != 1 || s.Entries[0].State != "reserved" {
		t.Fatal(s, err)
	}
}

func TestLegacyStateKeepsUnboundEntriesAndRejectsInvalidBindings(t *testing.T) {
	r := costRoot(t, t.TempDir())
	d, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Reserve(costPrice(), Bound{Input: 20}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := readCostState(r)
	if err != nil {
		t.Fatal(err)
	}
	s.Schema = 1
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile("ledger.json", data, 0600); err != nil {
		t.Fatal(err)
	}
	d, err = OpenDurableLedger(r, costIdentity(), "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := d.ReserveAttempt("new-attempt", strings.Repeat("a", 64), costPrice(), Bound{Input: 20}); err != nil || id != 2 {
		t.Fatal(id, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = readCostState(r)
	if err != nil || s.Schema != 2 || len(s.Bindings) != 1 || s.Bindings[0].EntryID != 2 || s.Ledger.Held != 40 {
		t.Fatal(s, err)
	}
	for _, mutate := range []func(*CostState){
		func(s *CostState) { s.Schema = 1 },
		func(s *CostState) { s.Bindings = append(s.Bindings, s.Bindings[0]) },
		func(s *CostState) { s.Bindings[0].EntryID = 9 },
		func(s *CostState) { s.Bindings[0].RequestHash = "bad" },
	} {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var copy CostState
		if err := json.Unmarshal(data, &copy); err != nil {
			t.Fatal(err)
		}
		mutate(&copy)
		data, err = json.Marshal(copy)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.WriteFile("ledger.json", data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenDurableLedger(r, costIdentity(), "microcredit", 100); !errors.Is(err, ErrState) {
			t.Fatal(copy, err)
		}
	}
}
