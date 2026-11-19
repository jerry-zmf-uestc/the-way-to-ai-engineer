package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func restoreFixture(t *testing.T) Snapshot {
	t.Helper()
	l, _ := NewLedger("microcredit", 100)
	p := Price{Model: "fixture", Version: "v1", Unit: "microcredit", Input: 1}
	a, _ := l.Reserve(p, Bound{Input: 20})
	if err := l.Begin(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(a, Usage{Input: 20}); err != nil {
		t.Fatal(err)
	}
	b, _ := l.Reserve(p, Bound{Input: 50})
	if err := l.Begin(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	return l.Snapshot()
}

func TestRestorePreservesUnknownReservationAndIdempotentSettlement(t *testing.T) {
	s := restoreFixture(t)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	l, err := RestoreLedger(decoded, "microcredit", 100)
	if err != nil {
		t.Fatal(err)
	}
	if l.Snapshot().Entries[1].State != "unknown" || s.Entries[1].State != "sent" {
		t.Fatal("restore changed input or replayed dispatch")
	}
	if err := l.Begin(context.Background(), 2); !errors.Is(err, ErrState) {
		t.Fatal("sent call dispatched again", err)
	}
	p := s.Entries[0].Price
	if _, err := l.Reserve(p, Bound{Input: 31}); !errors.Is(err, ErrBudget) {
		t.Fatal("restart dropped held cost", err)
	}
	decoded.Entries[1].Reserved = 0
	if l.Snapshot().Held != 50 {
		t.Fatal("shared mutable snapshot")
	}
	if err := l.Settle(2, Usage{Input: 40}); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(2, Usage{Input: 40}); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(2, Usage{Input: 41}); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	if got := l.Snapshot(); got.Used != 60 || got.Held != 0 {
		t.Fatal(got)
	}
	if _, err := l.Reserve(p, Bound{Input: 40}); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreRejectsAccountingAndPolicyMismatch(t *testing.T) {
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot) { s.Used = 0 }, func(s *Snapshot) { s.Held = 0 },
		func(s *Snapshot) { s.Frozen = true }, func(s *Snapshot) { s.Entries[1].ID = 1 },
		func(s *Snapshot) { s.Entries[1].State = "completed" },
		func(s *Snapshot) { s.Entries[1].Reserved = 0 },
		func(s *Snapshot) { s.Entries[1].Actual = 1 },
		func(s *Snapshot) { s.Entries[1].Usage.Input = 1 },
		func(s *Snapshot) { s.Entries[0].Actual = 19 },
		func(s *Snapshot) { s.Entries[0].Usage.Input = -1 },
		func(s *Snapshot) { s.Entries[0].State = "overrun" },
		func(s *Snapshot) { s.Entries[0].Price.Unit = "other" },
		func(s *Snapshot) { s.Entries[0].Price.Input = 2 },
		func(s *Snapshot) { s.Entries[1].Bound.Input = 1_000_001 },
		func(s *Snapshot) { s.Entries = append(s.Entries, make([]Entry, 63)...) },
	} {
		s := restoreFixture(t)
		mutate(&s)
		if l, err := RestoreLedger(s, "microcredit", 100); !errors.Is(err, ErrState) || l != nil {
			t.Fatal("invalid snapshot accepted", s, err)
		}
	}
	for _, policy := range []struct {
		unit  string
		limit int64
	}{{"other", 100}, {"microcredit", 101}} {
		if l, err := RestoreLedger(restoreFixture(t), policy.unit, policy.limit); !errors.Is(err, ErrState) || l != nil {
			t.Fatal(policy, err)
		}
	}
}

func TestRestorePreservesOverrunFreezeAndUnsentCancellation(t *testing.T) {
	p := Price{Model: "fixture", Version: "v1", Unit: "microcredit", Input: 1}
	l, _ := NewLedger("microcredit", 100)
	a, _ := l.Reserve(p, Bound{Input: 20})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Begin(ctx, a); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	b, _ := l.Reserve(p, Bound{Input: 10})
	if err := l.Begin(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(b, Usage{Input: 110}); !errors.Is(err, ErrOverrun) {
		t.Fatal(err)
	}
	restored, err := RestoreLedger(l.Snapshot(), "microcredit", 100)
	if err != nil || !restored.Snapshot().Frozen || restored.Snapshot().Used != 110 || restored.Snapshot().Held != 0 {
		t.Fatal(restored, err)
	}
	if _, err := restored.Reserve(p, Bound{}); !errors.Is(err, ErrBudget) {
		t.Fatal("restart thawed overrun", err)
	}
	if err := restored.Settle(b, Usage{Input: 110}); !errors.Is(err, ErrOverrun) {
		t.Fatal(err)
	}
}
