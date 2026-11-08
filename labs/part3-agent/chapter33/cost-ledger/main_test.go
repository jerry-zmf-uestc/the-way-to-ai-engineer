package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

var price = Price{Model: "fixture", Version: "v1", Unit: "microcredit", Input: 2, CacheRead: 1, CacheWrite: 3, Output: 4}

func ledger(t *testing.T, limit int64) *Ledger {
	t.Helper()
	l, err := NewLedger("microcredit", limit)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func sent(t *testing.T, l *Ledger, b Bound) int {
	t.Helper()
	id, err := l.Reserve(price, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Begin(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	return id
}
func TestReservationPriceAndSettlement(t *testing.T) {
	l := ledger(t, 1000)
	p := price
	id, err := l.Reserve(p, Bound{100, 20})
	if err != nil {
		t.Fatal(err)
	}
	p.Input = 999
	if s := l.Snapshot(); s.Held != 380 {
		t.Fatal(s)
	}
	if err = l.Begin(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	usage := Usage{Input: 20, CacheRead: 60, Output: 10}
	if err = l.Settle(id, usage); err != nil {
		t.Fatal(err)
	}
	if err = l.Settle(id, usage); err != nil {
		t.Fatal(err)
	}
	if err = l.Settle(id, Usage{Input: 21}); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	s := l.Snapshot()
	if s.Used != 140 || s.Held != 0 || s.Entries[0].Price.Input != 2 {
		t.Fatal(s)
	}
	s.Entries[0].Actual = 0
	if l.Snapshot().Used != 140 {
		t.Fatal("snapshot mutation")
	}
}
func TestCancellationAndUnknownReconciliation(t *testing.T) {
	l := ledger(t, 1000)
	id, _ := l.Reserve(price, Bound{100, 20})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Begin(ctx, id); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s := l.Snapshot(); s.Held != 0 || s.Used != 0 || s.Entries[0].State != "not_sent" {
		t.Fatal(s)
	}
	if err := l.Settle(id, Usage{Input: 1}); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	id = sent(t, l, Bound{100, 20})
	if err := l.Unknown(id); err != nil {
		t.Fatal(err)
	}
	if s := l.Snapshot(); s.Held != 380 || s.Used != 0 || s.Entries[1].State != "unknown" {
		t.Fatal(s)
	}
	if err := l.Settle(id, Usage{Input: 50, Output: 10}); err != nil {
		t.Fatal(err)
	}
	if s := l.Snapshot(); s.Held != 0 || s.Used != 140 {
		t.Fatal(s)
	}
}
func TestOverrunRecordsActualAndFreezes(t *testing.T) {
	l := ledger(t, 30)
	id := sent(t, l, Bound{1, 1})
	if err := l.Settle(id, Usage{Input: 10, Output: 10}); !errors.Is(err, ErrOverrun) {
		t.Fatal(err)
	}
	s := l.Snapshot()
	if !s.Frozen || s.Used != 60 || s.Held != 0 {
		t.Fatal(s)
	}
	if _, err := l.Reserve(price, Bound{}); !errors.Is(err, ErrBudget) {
		t.Fatal(err)
	}
	if err := l.Settle(id, Usage{Input: 10, Output: 10}); !errors.Is(err, ErrOverrun) || l.Snapshot().Used != 60 {
		t.Fatal(err)
	}
}
func TestConcurrentAdmissionIncludesOutstandingHolds(t *testing.T) {
	l := ledger(t, 30)
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := l.Reserve(price, Bound{Input: 1, Output: 1}); err == nil {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	s := l.Snapshot()
	if admitted.Load() != 4 || s.Held != 28 {
		t.Fatal(admitted.Load(), s)
	}
}
func TestInvalidUsageDoesNotBecomeFreeAndIntegerBounds(t *testing.T) {
	l := ledger(t, 1000)
	id := sent(t, l, Bound{1, 1})
	if err := l.Settle(id, Usage{Input: -1}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if s := l.Snapshot(); s.Held != 7 || s.Entries[0].State != "sent" {
		t.Fatal(s)
	}
	huge := price
	huge.Input = 1_000_001
	if _, err := l.Reserve(huge, Bound{1, 1}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	maximum := Price{Model: "max", Version: "v1", Unit: "microcredit", Input: 1_000_000, CacheRead: 1_000_000, CacheWrite: 1_000_000, Output: 1_000_000}
	cost, err := (Usage{Input: 1_000_000, Output: 1_000_000}).Cost(maximum)
	if err != nil || cost != 2_000_000_000_000 {
		t.Fatal(cost, err)
	}
	wrong := price
	wrong.Unit = "other"
	if _, err := l.Reserve(wrong, Bound{1, 1}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
func TestReportIncludesFailedTasksAndUnknownCoverage(t *testing.T) {
	rows := []TaskRecord{{CaseID: "a", Config: "baseline", Unit: "microcredit", Success: true, CostKnown: true, Cost: 20}, {CaseID: "b", Config: "baseline", Unit: "microcredit", Success: false, CostKnown: true, Cost: 80}}
	r, err := Summarize(rows)
	if err != nil || !r.CostPerSuccessKnown || r.CostPerSuccess != 100 {
		t.Fatal(r, err)
	}
	rows[1].CostKnown = false
	rows[1].Cost = 0
	r, err = Summarize(rows)
	if err != nil || r.Known != 1 || r.CostPerSuccessKnown {
		t.Fatal(r, err)
	}
	rows[0].Success = false
	r, err = Summarize(rows)
	if err != nil || r.CostPerSuccessKnown {
		t.Fatal(r, err)
	}
	rows[1].CaseID = "a"
	if _, err = Summarize(rows); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestComparisonsUseSameCasesQualityAndCompleteCosts(t *testing.T) {
	baseline := []TaskRecord{{CaseID: "a", Config: "base", Unit: "microcredit", Success: true, CostKnown: true, Cost: 60}, {CaseID: "b", Config: "base", Unit: "microcredit", Success: false, CostKnown: true, Cost: 60}}
	candidate := []TaskRecord{{CaseID: "a", Config: "new", Unit: "microcredit", Success: true, CostKnown: true, Cost: 90}, {CaseID: "b", Config: "new", Unit: "microcredit", Success: true, CostKnown: true, Cost: 90}}
	ok, _, err := Compare(baseline, candidate, 1)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	candidate[1].Success = false
	ok, reason, err := Compare(baseline, candidate, 1)
	if err != nil || ok || reason != "quality_floor" {
		t.Fatal(ok, reason, err)
	}
	candidate[1].Success = true
	candidate[1].CostKnown = false
	candidate[1].Cost = 0
	ok, _, err = Compare(baseline, candidate, 1)
	if err != nil || ok {
		t.Fatal(ok, err)
	}
	candidate[1].CaseID = "other"
	if _, _, err = Compare(baseline, candidate, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
