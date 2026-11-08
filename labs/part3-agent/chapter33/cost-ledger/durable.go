package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"unicode/utf8"
)

var ErrBusy = errors.New("cost_ledger_already_claimed")

type RunIdentity struct{ Tenant, User, Run string }
type AttemptBinding struct {
	Key, RequestHash string
	EntryID          int
}
type CostState struct {
	Schema   int
	Identity RunIdentity
	Ledger   Snapshot
	Bindings []AttemptBinding `json:",omitempty"`
}
type DurableLedger struct {
	mu       sync.Mutex
	root     *os.Root
	identity RunIdentity
	ledger   *Ledger
	bindings []AttemptBinding
	failed   bool
	closed   bool
	closeErr error
}

func OpenDurableLedger(root *os.Root, id RunIdentity, unit string, limit int64) (_ *DurableLedger, err error) {
	if root == nil {
		return nil, ErrState
	}
	for _, value := range []string{id.Tenant, id.User, id.Run} {
		if value == "" || len(value) > 128 || !utf8.ValidString(value) {
			return nil, ErrState
		}
	}
	info, err := root.Stat(".")
	if err != nil || info.Mode().Perm() != 0700 {
		return nil, errors.Join(ErrState, err)
	}
	f, err := root.OpenFile("ledger.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil, ErrBusy
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, root.Remove("ledger.lock"))
		}
	}()
	_, writeErr := fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	if err = errors.Join(writeErr, f.Close()); err != nil {
		return nil, err
	}
	d := &DurableLedger{root: root, identity: id}
	state, err := readCostState(root)
	if errors.Is(err, os.ErrNotExist) {
		d.ledger, err = NewLedger(unit, limit)
	} else if err == nil {
		if (state.Schema != 1 && state.Schema != 2) || (state.Schema == 1 && len(state.Bindings) != 0) || state.Identity != id {
			return nil, ErrState
		}
		d.ledger, err = RestoreLedger(state.Ledger, unit, limit)
		if err == nil {
			if err = validateBindings(state.Bindings, state.Ledger); err != nil {
				return nil, err
			}
			d.bindings = append([]AttemptBinding(nil), state.Bindings...)
		}
	}
	if err != nil {
		return nil, err
	}
	if err = d.save(); err != nil {
		return nil, err
	}
	return d, nil
}

func readCostState(root *os.Root) (CostState, error) {
	f, err := root.Open("ledger.json")
	if err != nil {
		return CostState{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return CostState{}, errors.Join(ErrState, err)
	}
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil {
		return CostState{}, err
	}
	if len(data) > 64<<10 || !utf8.Valid(data) || uniqueCostJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return CostState{}, ErrState
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var state CostState
	if dec.Decode(&state) != nil {
		return CostState{}, ErrState
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return CostState{}, ErrState
	}
	return state, nil
}

func (d *DurableLedger) save() (err error) {
	state := CostState{Schema: 2, Identity: d.identity, Ledger: d.ledger.Snapshot(), Bindings: d.bindings}
	if _, err := RestoreLedger(state.Ledger, state.Ledger.Unit, state.Ledger.Limit); err != nil {
		return err
	}
	if err := validateBindings(state.Bindings, state.Ledger); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil || len(data) > 64<<10 {
		return errors.Join(ErrState, err)
	}
	f, err := d.root.OpenFile("ledger.tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := d.root.Remove("ledger.tmp"); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	if err = errors.Join(writeErr, syncErr, f.Close()); err != nil {
		return err
	}
	if err = d.root.Rename("ledger.tmp", "ledger.json"); err != nil {
		return err
	}
	dir, err := d.root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

func (d *DurableLedger) update(change func(*Ledger) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failed || d.closed {
		return ErrState
	}
	changeErr := change(d.ledger)
	if err := d.save(); err != nil {
		d.failed = true
		return errors.Join(changeErr, err)
	}
	return changeErr
}

func (d *DurableLedger) Reserve(p Price, bound Bound) (int, error) {
	var id int
	err := d.update(func(l *Ledger) error {
		var err error
		id, err = l.Reserve(p, bound)
		return err
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}
func (d *DurableLedger) Begin(ctx context.Context, id int) error {
	return d.update(func(l *Ledger) error { return l.Begin(ctx, id) })
}
func (d *DurableLedger) Unknown(id int) error {
	return d.update(func(l *Ledger) error { return l.Unknown(id) })
}
func (d *DurableLedger) Settle(id int, usage Usage) error {
	return d.update(func(l *Ledger) error { return l.Settle(id, usage) })
}
func (d *DurableLedger) Snapshot() (Snapshot, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failed || d.closed {
		return Snapshot{}, ErrState
	}
	return d.ledger.Snapshot(), nil
}
func (d *DurableLedger) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		d.closeErr = d.root.Remove("ledger.lock")
	}
	return d.closeErr
}
