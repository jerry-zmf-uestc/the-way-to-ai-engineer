package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

var ErrJointTool = errors.New("checkpoint: joint tool failed")

type JointResponse struct {
	Text       string
	InputUsage *int64
	Failure    string
}
type JointAttempt struct {
	Action, Key, Hash, Phase string
	FeeID                    int
	Text                     string
	UsageKnown               bool
	Input                    int64
	Failure                  string
}
type JointState struct {
	Schema              int
	Identity            Identity
	PlanHash            string
	MaxAttempts, Cursor int
	Results             []Result
	Calls               []JointAttempt
}

func jointHash(plan, action string) string {
	data, _ := json.Marshal([]string{plan, action, "fixture-read-v1"})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
func jointFailure(kind string) error {
	switch kind {
	case "":
		return nil
	case "canceled":
		return context.Canceled
	case "deadline":
		return context.DeadlineExceeded
	case "overrun":
		return ErrBudget
	default:
		return ErrJointTool
	}
}
func validateJoint(s JointState, id Identity, actions []string, limit int) error {
	if s.Schema != 1 || s.Identity != id || s.PlanHash != fingerprint(actions) || s.MaxAttempts != limit || limit < 1 || limit > 32 || len(s.Calls) > limit || s.Cursor < 0 || s.Cursor > len(actions) || len(s.Results) != s.Cursor {
		return ErrState
	}
	position := 0
	seen := map[int]bool{}
	for n, c := range s.Calls {
		if position >= len(actions) || c.Action != actions[position] || c.Key != fmt.Sprintf("call-%d-attempt-%d", position, n+1) || c.Hash != jointHash(s.PlanHash, c.Action) || c.FeeID < 0 || c.FeeID > 64 || c.FeeID != 0 && seen[c.FeeID] {
			return ErrState
		}
		if c.FeeID != 0 {
			seen[c.FeeID] = true
		}
		switch c.Phase {
		case "prepared", "unknown":
			if c.Text != "" || c.UsageKnown || c.Input != 0 || c.Failure != "" || c.Phase == "prepared" && n != len(s.Calls)-1 || c.Phase == "unknown" && c.FeeID == 0 {
				return ErrState
			}
		case "received", "done":
			if c.FeeID == 0 || !utf8.ValidString(c.Text) || len(c.Text) > 4096 || c.Input < 0 || c.Input > 1_000_000 || !c.UsageKnown && c.Input != 0 || c.Failure != "" && c.Failure != "tool_failed" && c.Failure != "canceled" && c.Failure != "deadline" && c.Failure != "overrun" {
				return ErrState
			}
			if c.Phase == "received" && n != len(s.Calls)-1 {
				return ErrState
			}
			if c.Phase == "done" && c.Failure == "" {
				if position >= len(s.Results) || s.Results[position] != (Result{c.Action, c.Text}) {
					return ErrState
				}
				position++
			} else if c.Phase == "done" && c.Failure != "tool_failed" && n != len(s.Calls)-1 {
				return ErrState
			}
		default:
			return ErrState
		}
	}
	if position != s.Cursor {
		return ErrState
	}
	return nil
}
func (store Store) loadJoint() (JointState, error) {
	f, err := store.Root.Open("joint.json")
	if err != nil {
		return JointState{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil {
		return JointState{}, err
	}
	if len(data) > 64<<10 || !utf8.Valid(data) || uniqueStateJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return JointState{}, ErrState
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s JointState
	if dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF {
		return JointState{}, ErrState
	}
	return s, nil
}
func (store Store) saveJoint(s JointState) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return store.saveBytes("joint.json", "joint.tmp", data)
}

func auditJointFees(ctx context.Context, fees FeeCLI, s JointState) error {
	hasBinding := false
	for _, call := range s.Calls {
		hasBinding = hasBinding || call.FeeID != 0
	}
	if !hasBinding {
		return nil
	}
	reply, err := fees.Invoke(ctx, FeeRequest{Action: "snapshot"})
	if err != nil {
		return err
	}
	for _, call := range s.Calls {
		if call.FeeID == 0 {
			continue
		}
		if call.FeeID > len(reply.Snapshot.Entries) {
			return ErrState
		}
		binding, lookupErr := fees.Invoke(ctx, FeeRequest{Action: "lookup", Key: call.Key, Hash: call.Hash})
		if lookupErr != nil {
			return errors.Join(ErrState, lookupErr)
		}
		if binding.EntryID != call.FeeID {
			return ErrState
		}
		entry := reply.Snapshot.Entries[call.FeeID-1]
		if call.Phase == "received" || call.Phase == "done" || call.Phase == "unknown" {
			if entry.State == "reserved" || entry.State == "not_sent" {
				return ErrState
			}
		}
		if call.UsageKnown && (entry.State == "settled" || entry.State == "overrun") && entry.Usage.Input != call.Input {
			return ErrState
		}
		if call.Phase == "done" && call.UsageKnown {
			want := "settled"
			if call.Input > 50 {
				want = "overrun"
			}
			if entry.State != want {
				return ErrState
			}
		}
	}
	return nil
}

func RunJoint(ctx context.Context, store Store, id Identity, actions []string, limit int, replayReadonly bool, fees FeeCLI, read func(context.Context, string) JointResponse, hook func(string)) (state JointState, err error) {
	if store.Root == nil || fees.Run != id || read == nil || len(actions) < 1 || len(actions) > 16 || limit < 1 || limit > 32 {
		return state, ErrState
	}
	for _, value := range []string{id.Tenant, id.User, id.Session, id.Run} {
		if value == "" || len(value) > 128 || !utf8.ValidString(value) {
			return state, ErrState
		}
	}
	info, statErr := store.Root.Stat(".")
	if statErr != nil || info.Mode().Perm() != 0700 {
		return state, errors.Join(ErrState, statErr)
	}
	seen := map[string]bool{}
	for _, action := range actions {
		if action == "" || len(action) > 128 || !utf8.ValidString(action) || seen[action] {
			return state, ErrState
		}
		seen[action] = true
	}
	if err := ctx.Err(); err != nil {
		return state, err
	}
	release, err := store.claim()
	if err != nil {
		return state, err
	}
	defer func() { err = errors.Join(err, release()) }()
	state, err = store.loadJoint()
	if errors.Is(err, os.ErrNotExist) {
		state = JointState{Schema: 1, Identity: id, PlanHash: fingerprint(actions), MaxAttempts: limit}
		err = store.saveJoint(state)
	}
	if err != nil {
		return state, err
	}
	if err := validateJoint(state, id, actions, limit); err != nil {
		return state, err
	}
	if err := auditJointFees(ctx, fees, state); err != nil {
		return state, err
	}
	stage := func(name string) {
		if hook != nil {
			hook(name)
		}
	}
	for state.Cursor < len(actions) {
		if err := ctx.Err(); err != nil {
			return state, err
		}
		create := len(state.Calls) == 0
		if !create {
			last := state.Calls[len(state.Calls)-1]
			if last.Phase == "done" {
				if last.Failure != "" && !(last.Failure == "tool_failed" && replayReadonly) {
					return state, jointFailure(last.Failure)
				}
				create = true
			} else if last.Phase == "unknown" {
				if !replayReadonly {
					return state, ErrUnknown
				}
				create = true
			}
		}
		if create {
			if len(state.Calls) >= limit {
				return state, ErrBudget
			}
			action := actions[state.Cursor]
			state.Calls = append(state.Calls, JointAttempt{Action: action, Key: fmt.Sprintf("call-%d-attempt-%d", state.Cursor, len(state.Calls)+1), Hash: jointHash(state.PlanHash, action), Phase: "prepared"})
			if err := store.saveJoint(state); err != nil {
				return state, err
			}
			stage("after_intent")
		}
		call := &state.Calls[len(state.Calls)-1]
		if call.Phase == "prepared" {
			reply, feeErr := fees.Invoke(ctx, FeeRequest{Action: "prepare", Key: call.Key, Hash: call.Hash})
			if feeErr != nil {
				return state, feeErr
			}
			stage("after_fee_prepare")
			if call.FeeID != 0 && call.FeeID != reply.EntryID {
				return state, ErrState
			}
			call.FeeID = reply.EntryID
			if err := store.saveJoint(state); err != nil {
				return state, err
			}
			stage("after_binding")
			entry := reply.Snapshot.Entries[call.FeeID-1]
			if entry.State != "reserved" {
				call.Phase = "unknown"
				if err := store.saveJoint(state); err != nil {
					return state, err
				}
				if !replayReadonly {
					return state, ErrUnknown
				}
				continue
			}
			if _, err := fees.Invoke(ctx, FeeRequest{Action: "begin", EntryID: call.FeeID}); err != nil {
				return state, err
			}
			stage("after_fee_begin")
			if err := ctx.Err(); err != nil {
				return state, err
			}
			response := read(ctx, call.Action)
			stage("after_execute")
			if !utf8.ValidString(response.Text) || len(response.Text) > 4096 || response.Failure != "" && response.Failure != "tool_failed" && response.Failure != "canceled" && response.Failure != "deadline" || response.InputUsage != nil && (*response.InputUsage < 0 || *response.InputUsage > 1_000_000) {
				return state, ErrState
			}
			call.Text, call.Failure, call.Phase = response.Text, response.Failure, "received"
			if response.InputUsage != nil {
				call.UsageKnown = true
				call.Input = *response.InputUsage
			}
			if stop := ctx.Err(); stop != nil {
				call.Failure = "canceled"
				if errors.Is(stop, context.DeadlineExceeded) {
					call.Failure = "deadline"
				}
			}
			if err := store.saveJoint(state); err != nil {
				return state, err
			}
			stage("after_receipt")
			if err := ctx.Err(); err != nil {
				return state, err
			}
		}
		request := FeeRequest{Action: "unknown", EntryID: call.FeeID}
		if call.UsageKnown {
			request.Action = "settle"
			request.InputUsage = &call.Input
		}
		reply, feeErr := fees.Invoke(ctx, request)
		if feeErr != nil {
			if !errors.Is(feeErr, ErrFeeOperation) || reply.Error != "overrun" {
				return state, feeErr
			}
			call.Failure = "overrun"
		}
		stage("after_fee_settle")
		call.Phase = "done"
		if call.Failure == "" {
			state.Results = append(state.Results, Result{Action: call.Action, Text: call.Text})
			state.Cursor++
		}
		if err := store.saveJoint(state); err != nil {
			return state, err
		}
		stage("after_result_commit")
		if call.Failure != "" && !(call.Failure == "tool_failed" && replayReadonly) {
			return state, jointFailure(call.Failure)
		}
	}
	return state, nil
}
