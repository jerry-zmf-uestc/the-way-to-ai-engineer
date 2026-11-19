package main

import (
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

func validAttempt(key, requestHash string) bool {
	decoded, err := hex.DecodeString(requestHash)
	return strings.TrimSpace(key) != "" && len(key) <= 128 && utf8.ValidString(key) && err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == requestHash
}

func validateBindings(bindings []AttemptBinding, s Snapshot) error {
	if len(bindings) > 64 {
		return ErrState
	}
	keys := map[string]bool{}
	ids := map[int]bool{}
	for _, b := range bindings {
		if !validAttempt(b.Key, b.RequestHash) || keys[b.Key] || ids[b.EntryID] || b.EntryID < 1 || b.EntryID > len(s.Entries) || s.Entries[b.EntryID-1].ID != b.EntryID {
			return ErrState
		}
		keys[b.Key], ids[b.EntryID] = true, true
	}
	return nil
}

func (d *DurableLedger) LookupAttempt(key, requestHash string, p Price, bound Bound) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failed || d.closed || !validAttempt(key, requestHash) {
		return 0, ErrState
	}
	for _, binding := range d.bindings {
		if binding.Key == key {
			entry := d.ledger.Snapshot().Entries[binding.EntryID-1]
			if binding.RequestHash != requestHash || entry.Price != p || entry.Bound != bound {
				return 0, ErrState
			}
			return entry.ID, nil
		}
	}
	return 0, ErrState
}

// ReserveAttempt binds one physical attempt; retrying an execution needs a new key.
func (d *DurableLedger) ReserveAttempt(key, requestHash string, p Price, bound Bound) (int, error) {
	if !validAttempt(key, requestHash) {
		return 0, ErrState
	}
	var id int
	err := d.update(func(l *Ledger) error {
		for _, binding := range d.bindings {
			if binding.Key == key {
				entry := l.Snapshot().Entries[binding.EntryID-1]
				if binding.RequestHash != requestHash || entry.Price != p || entry.Bound != bound {
					return ErrState
				}
				id = entry.ID
				return nil
			}
		}
		var err error
		id, err = l.Reserve(p, bound)
		if err == nil {
			d.bindings = append(d.bindings, AttemptBinding{Key: key, RequestHash: requestHash, EntryID: id})
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}
