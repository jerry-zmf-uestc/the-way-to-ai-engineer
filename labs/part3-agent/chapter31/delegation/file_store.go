package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

var (
	ErrHandoffBusy    = errors.New("handoff: writer already claimed")
	ErrHandoffUnknown = errors.New("handoff: previous execution not confirmed")
	ErrHandoffFailed  = errors.New("handoff: previous execution failed")
)

type HandoffFileRecord struct {
	Schema       int
	Phase        string
	Stable       HandoffCheckpoint
	Used         int
	InitialUsed  int
	Attempts     []HandoffToolAttempt
	InitialTurns int
	RoleResults  []HandoffRoleResult
	Generation   int
	Recoveries   []HandoffReadonlyRecovery
}
type HandoffReadonlyRecovery struct {
	Generation int
	Actor      string
	Revision   uint64
	Used       int
}
type HandoffRecoveryApproval struct {
	Version, Actor            string
	Readonly, PreviousStopped bool
}
type HandoffRoleResult struct {
	Turn   int
	Result Result
}
type HandoffToolAttempt struct {
	Sequence           int
	Generation         int
	Actor, Tool        string
	Revision           uint64
	Outcome, ErrorCode string
	Raw                Fact
}
type HandoffStore struct {
	root    *os.Root
	fresh   func() (*Handoff, error)
	version string
}

func readonlyHandoffFactory() (*Handoff, error) {
	d := Dispatcher{Roles: roles(), ParentTools: tools(), Ledger: &Ledger{Limit: 2}, MaxDepth: 2}
	task := Task{ID: "report", Parent: "request", Scope: Scope{"demo", "reader"}, Goal: "read timeout and retry"}
	return NewHandoff(task, d, "timeout-reader", map[string][]string{"timeout-reader": {"retry-reader"}}, 2, 4)
}

func OpenHandoffStore(dir string, fresh func() (*Handoff, error), version string) (*HandoffStore, error) {
	if !filepath.IsAbs(dir) || fresh == nil {
		return nil, ErrHandoff
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, ErrHandoff
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &HandoffStore{root, fresh, version}, nil
}
func (s *HandoffStore) Close() error { return s.root.Close() }

func (s *HandoffStore) load() (HandoffFileRecord, error) {
	info, err := s.root.Lstat("record.json")
	if err != nil {
		return HandoffFileRecord{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 64<<10 {
		return HandoffFileRecord{}, ErrHandoff
	}
	f, err := s.root.Open("record.json")
	if err != nil {
		return HandoffFileRecord{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(data) > 64<<10 || !utf8.Valid(data) || uniqueHandoffJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return HandoffFileRecord{}, ErrHandoff
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var record HandoffFileRecord
	if dec.Decode(&record) != nil || dec.Decode(new(any)) != io.EOF || record.Schema < 1 || record.Schema > 5 || record.Phase != "stable" && record.Phase != "executing" && record.Phase != "failed" {
		return HandoffFileRecord{}, ErrHandoff
	}
	return record, nil
}

func (s *HandoffStore) save(record HandoffFileRecord) error {
	data, err := json.Marshal(record)
	if err != nil || len(data) > 64<<10 {
		return ErrHandoff
	}
	f, err := s.root.OpenFile(".record.tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove(".record.tmp")
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = s.root.Rename(".record.tmp", "record.json"); err != nil {
		return err
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *HandoffStore) Apply(ctx context.Context, operation string, request HandoffRequest) (record HandoffFileRecord, err error) {
	return s.apply(ctx, operation, request, 0, nil)
}

func (s *HandoffStore) ApplyVersioned(ctx context.Context, operation string, request HandoffRequest, generation int) (HandoffFileRecord, error) {
	return s.apply(ctx, operation, request, generation, nil)
}

func (s *HandoffStore) RecoverReadonly(ctx context.Context, request HandoffRequest, generation int, approval HandoffRecoveryApproval) (HandoffFileRecord, error) {
	return s.apply(ctx, "recover-readonly", request, generation, &approval)
}

func (s *HandoffStore) apply(ctx context.Context, operation string, request HandoffRequest, generation int, approval *HandoffRecoveryApproval) (record HandoffFileRecord, err error) {
	if ctx.Err() != nil {
		return record, ctx.Err()
	}
	if generation < 0 || generation > 32 || operation != "init" && operation != "show" && operation != "execute" && operation != "transfer" && operation != "complete" && operation != "recover-readonly" {
		return record, ErrHandoff
	}
	if operation == "recover-readonly" && (approval == nil || !approval.Readonly || !approval.PreviousStopped || approval.Version != s.version || approval.Actor != request.From || request.To != "") {
		return record, ErrDenied
	}
	if operation != "show" {
		lock, claimErr := s.root.OpenFile("writer.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if errors.Is(claimErr, os.ErrExist) {
			return record, ErrHandoffBusy
		}
		if claimErr != nil {
			return record, claimErr
		}
		if err = lock.Close(); err != nil {
			s.root.Remove("writer.lock")
			return record, err
		}
		defer func() { err = errors.Join(err, s.root.Remove("writer.lock")) }()
	}
	h, err := s.fresh()
	if err != nil {
		return record, err
	}
	if h == nil || h.dispatcher.Ledger == nil || h.dispatcher.Ledger.Used != 0 || h.dispatcher.Ledger.beforeTake != nil || h.dispatcher.Ledger.afterReturn != nil {
		return record, ErrHandoff
	}
	if operation == "init" {
		if generation != 0 {
			return record, ErrHandoff
		}
		if _, err := s.root.Lstat("record.json"); !errors.Is(err, os.ErrNotExist) {
			return record, ErrHandoff
		}
		data, err := CheckpointHandoff(h, s.version)
		if err != nil || json.Unmarshal(data, &record.Stable) != nil {
			return HandoffFileRecord{}, ErrHandoff
		}
		record.Schema, record.Phase, record.Used = 5, "stable", record.Stable.Used
		record.InitialUsed = record.Stable.Used
		record.InitialTurns = record.Stable.State.Turns
		return record, s.save(record)
	}
	record, err = s.load()
	if err != nil {
		return HandoffFileRecord{}, err
	}
	fingerprint, err := handoffFingerprint(h, h.initialOwner, s.version)
	if err != nil || record.Stable.Fingerprint != fingerprint || record.Stable.State.ID != h.task.ID || record.Stable.State.Scope != h.task.Scope || record.Stable.Used < 0 || record.Used < record.Stable.Used || record.Used > h.dispatcher.Ledger.Limit || record.Phase == "stable" && record.Used != record.Stable.Used {
		return HandoffFileRecord{}, ErrHandoff
	}
	if record.Schema == 1 {
		if record.InitialUsed != 0 || len(record.Attempts) != 0 {
			return HandoffFileRecord{}, ErrHandoff
		}
	} else if !validHandoffAttempts(record, h) {
		return HandoffFileRecord{}, ErrHandoff
	}
	if record.Schema < 3 {
		if record.InitialTurns != 0 || len(record.RoleResults) != 0 {
			return HandoffFileRecord{}, ErrHandoff
		}
	} else if !validHandoffResults(record, h) {
		return HandoffFileRecord{}, ErrHandoff
	}
	if !validHandoffRecoveries(record, h) {
		return HandoffFileRecord{}, ErrHandoff
	}
	data, _ := json.Marshal(record.Stable)
	edges := map[string][]string{}
	for from, targets := range h.edges {
		for to := range targets {
			edges[from] = append(edges[from], to)
		}
	}
	h, err = RestoreHandoff(data, h.task, h.dispatcher, h.initialOwner, edges, h.maxHops, h.maxTurns, s.version)
	if err != nil {
		return HandoffFileRecord{}, err
	}
	if operation == "show" {
		return record, nil
	}
	if ctx.Err() != nil {
		return record, ctx.Err()
	}
	if generation != record.Generation {
		return record, ErrHandoff
	}
	if operation == "recover-readonly" {
		if record.Schema < 4 {
			return record, ErrHandoffUnknown
		}
		if record.Phase != "executing" || !h.authorized(request) {
			return record, ErrHandoff
		}
		if record.Used >= h.dispatcher.Ledger.Limit || record.Generation+record.Stable.State.Turns >= h.maxTurns {
			return record, ErrBudget
		}
		record.Schema, record.Phase = 5, "stable"
		record.Generation++
		record.Recoveries = append(record.Recoveries, HandoffReadonlyRecovery{record.Generation, request.From, request.Revision, record.Used})
		record.Stable.Used = record.Used
		return record, s.save(record)
	}
	if record.Phase == "failed" {
		return record, ErrHandoffFailed
	}
	if record.Phase != "stable" {
		return record, ErrHandoffUnknown
	}
	if !h.authorized(request) {
		return HandoffFileRecord{}, ErrHandoff
	}
	if record.Schema == 1 {
		record.InitialUsed = record.Used
	}
	if record.Schema < 3 {
		record.InitialTurns = record.Stable.State.Turns
	}
	record.Schema = 5
	var roleResult Result
	switch operation {
	case "execute":
		if record.Generation+record.Stable.State.Turns >= h.maxTurns {
			return record, ErrBudget
		}
		record.Phase = "executing"
		if err = s.save(record); err != nil {
			return record, err
		}
		h.dispatcher.Ledger.beforeTake = func(tool string, next int) error {
			if len(record.Attempts) >= 128 || next != record.Used+1 {
				return ErrBudget
			}
			record.Attempts = append(record.Attempts, HandoffToolAttempt{Sequence: next, Generation: record.Generation, Actor: request.From, Tool: tool, Revision: request.Revision + 1, Outcome: "reserved"})
			record.Used = next
			return s.save(record)
		}
		h.dispatcher.Ledger.afterReturn = func(sequence int, tool string, fact Fact, cause error) error {
			index := sequence - record.InitialUsed - 1
			if index < 0 || index >= len(record.Attempts) || record.Attempts[index].Tool != tool || record.Attempts[index].Outcome != "reserved" {
				return ErrHandoff
			}
			entry := &record.Attempts[index]
			if !boundedRawFact(fact) || cause == nil && !checkpointFact(fact) {
				entry.Outcome, entry.ErrorCode = "rejected", "invalid_payload"
				return errors.Join(ErrResult, s.save(record))
			}
			entry.Raw = fact
			if cause == nil {
				entry.Outcome = "returned"
			} else {
				entry.Outcome, entry.ErrorCode = "error", "tool_error"
				if errors.Is(cause, context.Canceled) {
					entry.ErrorCode = "canceled"
				}
				if errors.Is(cause, context.DeadlineExceeded) {
					entry.ErrorCode = "deadline_exceeded"
				}
			}
			return s.save(record)
		}
		roleResult, err = h.Execute(ctx, request)
		h.dispatcher.Ledger.beforeTake = nil
		h.dispatcher.Ledger.afterReturn = nil
	case "transfer":
		err = h.Transfer(ctx, request)
	case "complete":
		err = h.Complete(ctx, request)
	}
	if err != nil {
		if operation == "execute" {
			record.Phase, record.Used = "failed", max(record.Used, h.dispatcher.Ledger.Used)
			err = errors.Join(err, s.save(record))
		}
		return record, err
	}
	data, err = CheckpointHandoff(h, s.version)
	if err != nil || json.Unmarshal(data, &record.Stable) != nil {
		return record, ErrHandoff
	}
	if operation == "execute" {
		roleResult.Facts = append([]Fact(nil), roleResult.Facts...)
		roleResult.Missing = append([]string(nil), roleResult.Missing...)
		record.RoleResults = append(record.RoleResults, HandoffRoleResult{record.Stable.State.Turns, roleResult})
	}
	if !validHandoffResults(record, h) {
		return record, ErrHandoff
	}
	record.Phase, record.Used = "stable", record.Stable.Used
	return record, s.save(record)
}

func validHandoffRecoveries(record HandoffFileRecord, h *Handoff) bool {
	if record.Generation < 0 || record.Generation > 32 || record.Generation != len(record.Recoveries) || record.Generation+record.Stable.State.Turns > h.maxTurns || record.Schema < 5 && record.Generation != 0 {
		return false
	}
	used, revision := record.InitialUsed, uint64(1)
	for n, recovery := range record.Recoveries {
		if _, ok := h.dispatcher.Roles[recovery.Actor]; !ok || recovery.Generation != n+1 || recovery.Revision < revision || recovery.Revision > record.Stable.State.Revision || recovery.Used < used || recovery.Used > record.Used {
			return false
		}
		used, revision = recovery.Used, recovery.Revision
	}
	return true
}

func validHandoffResults(record HandoffFileRecord, h *Handoff) bool {
	if record.InitialTurns < 0 || record.InitialTurns > record.Stable.State.Turns || len(record.RoleResults) > 32 || record.Stable.State.Turns-record.InitialTurns != len(record.RoleResults) {
		return false
	}
	known := map[string]string{}
	for _, fact := range record.Stable.State.Facts {
		known[fact.Key] = fact.Value
	}
	for n, entry := range record.RoleResults {
		result := entry.Result
		role, ok := h.dispatcher.Roles[result.Actor]
		if entry.Turn != record.InitialTurns+n+1 || !ok || role.Name != result.Actor || result.TaskID != h.task.ID || result.Parent != h.task.Parent || result.Scope != h.task.Scope || len(result.Facts) > 8 || len(result.Missing) > 8 {
			return false
		}
		for _, fact := range result.Facts {
			value, found := known[fact.Key]
			if !checkpointFact(fact) || !found || value != fact.Value {
				return false
			}
		}
		for _, missing := range result.Missing {
			if missing == "" || len(missing) > 256 || !utf8.ValidString(missing) {
				return false
			}
		}
	}
	return true
}

func validHandoffAttempts(record HandoffFileRecord, h *Handoff) bool {
	if record.InitialUsed < 0 || record.InitialUsed > record.Used || len(record.Attempts) > 128 || record.Used-record.InitialUsed != len(record.Attempts) || len(record.Recoveries) > 32 {
		return false
	}
	maxRevision := record.Stable.State.Revision
	if record.Phase != "stable" {
		maxRevision++
	}
	for _, recovery := range record.Recoveries {
		if recovery.Revision == ^uint64(0) {
			return false
		}
		maxRevision = max(maxRevision, recovery.Revision+1)
	}
	for n, attempt := range record.Attempts {
		generation := 0
		for _, recovery := range record.Recoveries {
			if recovery.Used < attempt.Sequence {
				generation++
			}
		}
		if attempt.Generation != generation || record.Schema < 5 && attempt.Generation != 0 {
			return false
		}
		role, ok := h.dispatcher.Roles[attempt.Actor]
		if !ok || attempt.Sequence != record.InitialUsed+n+1 || attempt.Revision < 2 || attempt.Revision > maxRevision {
			return false
		}
		allowed := false
		for _, tool := range role.Tools {
			if tool == attempt.Tool && h.dispatcher.ParentTools[tool] != nil {
				allowed = true
			}
		}
		if !allowed {
			return false
		}
		if !validHandoffOutcome(attempt) || record.Schema < 4 && attempt.Outcome != "" {
			return false
		}
	}
	return true
}

func boundedRawFact(f Fact) bool {
	return len(f.Key) <= 128 && len(f.Source) <= 256 && len(f.Value) <= 4096 && utf8.ValidString(f.Key) && utf8.ValidString(f.Source) && utf8.ValidString(f.Value)
}

func validHandoffOutcome(a HandoffToolAttempt) bool {
	switch a.Outcome {
	case "", "reserved":
		return a.Raw == (Fact{}) && a.ErrorCode == ""
	case "returned":
		return checkpointFact(a.Raw) && a.ErrorCode == ""
	case "error":
		return boundedRawFact(a.Raw) && (a.ErrorCode == "tool_error" || a.ErrorCode == "canceled" || a.ErrorCode == "deadline_exceeded")
	case "rejected":
		return a.Raw == (Fact{}) && a.ErrorCode == "invalid_payload"
	}
	return false
}
