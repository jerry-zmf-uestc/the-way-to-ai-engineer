package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
)

var (
	ErrInvalid = errors.New("invalid_cost_contract")
	ErrState   = errors.New("invalid_call_state")
	ErrBudget  = errors.New("cost_budget_exhausted")
	ErrOverrun = errors.New("actual_usage_exceeded_reservation")
)

type Price struct {
	Model, Version, Unit                 string
	Input, CacheRead, CacheWrite, Output int64
}
type Usage struct{ Input, CacheRead, CacheWrite, Output int64 }
type Bound struct{ Input, Output int64 }

func (p Price) valid() bool {
	if p.Model == "" || p.Version == "" || p.Unit == "" || len(p.Model) > 128 || len(p.Version) > 128 || len(p.Unit) > 32 {
		return false
	}
	for _, v := range []int64{p.Input, p.CacheRead, p.CacheWrite, p.Output} {
		if v < 0 || v > 1_000_000 {
			return false
		}
	}
	return true
}
func (u Usage) valid() bool {
	for _, v := range []int64{u.Input, u.CacheRead, u.CacheWrite, u.Output} {
		if v < 0 || v > 1_000_000 {
			return false
		}
	}
	return u.Input+u.CacheRead+u.CacheWrite <= 1_000_000
}
func (u Usage) Cost(p Price) (int64, error) {
	if !p.valid() || !u.valid() {
		return 0, ErrInvalid
	}
	// Bounded integer ranges ensure every product and total fit int64.
	return u.Input*p.Input + u.CacheRead*p.CacheRead + u.CacheWrite*p.CacheWrite + u.Output*p.Output, nil
}

type Entry struct {
	ID               int
	State            string
	Price            Price
	Bound            Bound
	Reserved, Actual int64
	Usage            Usage
}
type Snapshot struct {
	Unit              string
	Limit, Used, Held int64
	Frozen            bool
	Entries           []Entry
}
type Ledger struct {
	mu                sync.Mutex
	unit              string
	limit, used, held int64
	frozen            bool
	entries           map[int]Entry
	serial            int
}

func NewLedger(unit string, limit int64) (*Ledger, error) {
	if unit == "" || len(unit) > 32 || limit <= 0 || limit > 1_000_000_000_000 {
		return nil, ErrInvalid
	}
	return &Ledger{unit: unit, limit: limit, entries: map[int]Entry{}}, nil
}
func (l *Ledger) Reserve(p Price, b Bound) (int, error) {
	if !p.valid() || p.Unit != l.unit || b.Input < 0 || b.Input > 1_000_000 || b.Output < 0 || b.Output > 1_000_000 {
		return 0, ErrInvalid
	}
	amount := b.Input*max(p.Input, p.CacheRead, p.CacheWrite) + b.Output*p.Output
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.frozen || len(l.entries) >= 64 || amount > l.limit-l.used-l.held {
		return 0, ErrBudget
	}
	l.serial++
	l.entries[l.serial] = Entry{ID: l.serial, State: "reserved", Price: p, Bound: b, Reserved: amount}
	l.held += amount
	return l.serial, nil
}

// Begin is the dispatch boundary; IDs are local to one Ledger and never provided by the model.
func (l *Ledger) Begin(ctx context.Context, id int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[id]
	if !ok || e.State != "reserved" {
		return ErrState
	}
	if err := ctx.Err(); err != nil {
		l.held -= e.Reserved
		e.State = "not_sent"
		l.entries[id] = e
		return err
	}
	e.State = "sent"
	l.entries[id] = e
	return nil
}
func (l *Ledger) Unknown(id int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[id]
	if !ok || (e.State != "sent" && e.State != "unknown") {
		return ErrState
	}
	e.State = "unknown"
	l.entries[id] = e
	// Request failure/cancel after dispatch is not proof of zero consumption.
	return nil
}
func (l *Ledger) Settle(id int, usage Usage) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[id]
	if !ok {
		return ErrState
	}
	if e.State == "settled" || e.State == "overrun" {
		if usage != e.Usage {
			return ErrState
		}
		if e.State == "overrun" {
			return ErrOverrun
		}
		return nil
	}
	if e.State != "sent" && e.State != "unknown" {
		return ErrState
	}
	amount, err := usage.Cost(e.Price)
	if err != nil {
		return err
	}
	l.held -= e.Reserved
	l.used += amount
	e.Usage = usage
	e.Actual = amount
	e.State = "settled"
	if amount > e.Reserved || usage.Input+usage.CacheRead+usage.CacheWrite > e.Bound.Input || usage.Output > e.Bound.Output {
		l.frozen = true
		e.State = "overrun"
		l.entries[id] = e
		return ErrOverrun
	}
	l.entries[id] = e
	return nil
}
func (l *Ledger) Snapshot() Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := Snapshot{Unit: l.unit, Limit: l.limit, Used: l.used, Held: l.held, Frozen: l.frozen}
	for id := 1; id <= l.serial; id++ {
		s.Entries = append(s.Entries, l.entries[id])
	}
	return s
}

type TaskRecord struct {
	CaseID, Config, Unit string
	Success, CostKnown   bool
	Cost                 int64
}
type Report struct {
	Unit                    string
	Tasks, Successes, Known int
	KnownCost               int64
	CostPerSuccessKnown     bool
	CostPerSuccess          float64
}

// Summarize consumes task-level independent verdicts; failure/retry charges must already be aggregated.
func Summarize(rows []TaskRecord) (Report, error) {
	if len(rows) == 0 || len(rows) > 100 {
		return Report{}, ErrInvalid
	}
	r := Report{Unit: rows[0].Unit}
	seen := map[string]bool{}
	config := rows[0].Config
	for _, row := range rows {
		if row.CaseID == "" || seen[row.CaseID] || row.Config == "" || row.Config != config || row.Unit == "" || row.Unit != r.Unit || row.Cost < 0 || row.Cost > 128_000_000_000_000 || (!row.CostKnown && row.Cost != 0) {
			return Report{}, ErrInvalid
		}
		seen[row.CaseID] = true
		r.Tasks++
		if row.Success {
			r.Successes++
		}
		if row.CostKnown {
			r.Known++
			r.KnownCost += row.Cost
		}
	}
	r.CostPerSuccessKnown = r.Known == r.Tasks && r.Successes > 0
	if r.CostPerSuccessKnown {
		r.CostPerSuccess = float64(r.KnownCost) / float64(r.Successes)
	}
	return r, nil
}

// Compare checks the same case set before applying illustrative quality/cost criteria.
// It does not measure latency uncertainty or authorize a production rollout.
func Compare(baseline, candidate []TaskRecord, minSuccessRate float64) (bool, string, error) {
	if math.IsNaN(minSuccessRate) || minSuccessRate < 0 || minSuccessRate > 1 {
		return false, "", ErrInvalid
	}
	b, err := Summarize(baseline)
	if err != nil {
		return false, "", err
	}
	c, err := Summarize(candidate)
	if err != nil {
		return false, "", err
	}
	if b.Unit != c.Unit || len(baseline) != len(candidate) || baseline[0].Config == candidate[0].Config {
		return false, "", ErrInvalid
	}
	ids := map[string]bool{}
	for _, row := range baseline {
		ids[row.CaseID] = true
	}
	for _, row := range candidate {
		if !ids[row.CaseID] {
			return false, "", ErrInvalid
		}
	}
	if float64(c.Successes)/float64(c.Tasks) < minSuccessRate {
		return false, "quality_floor", nil
	}
	if !b.CostPerSuccessKnown || !c.CostPerSuccessKnown {
		return false, "incomplete_cost_or_no_success", nil
	}
	if c.CostPerSuccess >= b.CostPerSuccess {
		return false, "no_cost_gain", nil
	}
	return true, "fixture_candidate", nil
}

func main() {
	if handled, code := RunLedgerCLI(os.Args[1:], os.Stdout); handled {
		if code != 0 {
			os.Exit(code)
		}
		return
	}
	p := Price{Model: "fixture-fast", Version: "teaching-v1", Unit: "microcredit", Input: 2, CacheRead: 1, CacheWrite: 3, Output: 4}
	l, _ := NewLedger("microcredit", 1000)
	id, _ := l.Reserve(p, Bound{Input: 100, Output: 20})
	_ = l.Begin(context.Background(), id)
	if err := l.Settle(id, Usage{Input: 20, CacheRead: 60, Output: 10}); err != nil {
		panic(err)
	}
	s := l.Snapshot()
	fmt.Printf("used=%d held=%d unit=%s price=%s\n", s.Used, s.Held, s.Unit, s.Entries[0].Price.Version)
	cheap, _ := Summarize([]TaskRecord{{CaseID: "a", Config: "cheap", Unit: p.Unit, Success: true, CostKnown: true, Cost: 60}, {CaseID: "b", Config: "cheap", Unit: p.Unit, Success: false, CostKnown: true, Cost: 60}})
	strong, _ := Summarize([]TaskRecord{{CaseID: "a", Config: "strong", Unit: p.Unit, Success: true, CostKnown: true, Cost: 90}, {CaseID: "b", Config: "strong", Unit: p.Unit, Success: true, CostKnown: true, Cost: 90}})
	fmt.Printf("fixture_cost_per_success cheap=%.0f strong=%.0f\n", cheap.CostPerSuccess, strong.CostPerSuccess)
}
