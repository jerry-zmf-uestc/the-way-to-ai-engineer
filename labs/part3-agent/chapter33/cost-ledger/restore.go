package main

import "unicode/utf8"

// RestoreLedger consumes a snapshot already bound to the Run by trusted storage.
// It validates accounting, not provenance, and never dispatches requests.
func RestoreLedger(s Snapshot, unit string, limit int64) (*Ledger, error) {
	l, err := NewLedger(unit, limit)
	if err != nil {
		return nil, err
	}
	if s.Unit != unit || s.Limit != limit || !utf8.ValidString(unit) || len(s.Entries) > 64 {
		return nil, ErrState
	}
	for n, e := range s.Entries {
		if e.ID != n+1 || !e.Price.valid() || e.Price.Unit != unit || !utf8.ValidString(e.Price.Model) || !utf8.ValidString(e.Price.Version) || e.Bound.Input < 0 || e.Bound.Input > 1_000_000 || e.Bound.Output < 0 || e.Bound.Output > 1_000_000 {
			return nil, ErrState
		}
		reserved := e.Bound.Input*max(e.Price.Input, e.Price.CacheRead, e.Price.CacheWrite) + e.Bound.Output*e.Price.Output
		if e.Reserved != reserved {
			return nil, ErrState
		}
		switch e.State {
		case "reserved", "sent", "unknown", "not_sent":
			if e.Actual != 0 || e.Usage != (Usage{}) {
				return nil, ErrState
			}
			if e.State != "not_sent" {
				l.held += reserved
			}
			if e.State == "sent" {
				e.State = "unknown"
			}
		case "settled", "overrun":
			actual, err := e.Usage.Cost(e.Price)
			if err != nil || actual != e.Actual {
				return nil, ErrState
			}
			overrun := actual > reserved || e.Usage.Input+e.Usage.CacheRead+e.Usage.CacheWrite > e.Bound.Input || e.Usage.Output > e.Bound.Output
			if (e.State == "overrun") != overrun {
				return nil, ErrState
			}
			l.used += actual
			l.frozen = l.frozen || overrun
		default:
			return nil, ErrState
		}
		l.entries[e.ID] = e
		l.serial = e.ID
	}
	if l.used != s.Used || l.held != s.Held || l.frozen != s.Frozen || (!l.frozen && l.used+l.held > limit) {
		return nil, ErrState
	}
	return l, nil
}
