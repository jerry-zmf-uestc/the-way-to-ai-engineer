package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func storeRequest(record HandoffFileRecord, to string) HandoffRequest {
	s := record.Stable.State
	return HandoffRequest{ID: s.ID, Scope: s.Scope, From: s.Owner, To: to, Revision: s.Revision}
}

func TestHandoffStoreClaimCoversExecution(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	factory := func() (*Handoff, error) {
		h, err := readonlyHandoffFactory()
		if err == nil {
			h.dispatcher.ParentTools["read-timeout"] = func(ctx context.Context, _ Scope) (Fact, error) {
				if calls.Add(1) == 1 {
					close(started)
				}
				select {
				case <-release:
					return Fact{"timeout", "30", "guide@v1:timeout"}, nil
				case <-ctx.Done():
					return Fact{}, ctx.Err()
				}
			}
		}
		return h, err
	}
	a, err := OpenHandoffStore(dir, factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := OpenHandoffStore(dir, factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	r, err := a.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := a.Apply(ctx, "execute", storeRequest(r, "")); done <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("tool not entered")
	}
	if _, err := b.Apply(ctx, "execute", storeRequest(r, "")); !errors.Is(err, ErrHandoffBusy) || calls.Load() != 1 {
		t.Fatal("second instance executed concurrently", err)
	}
	if _, err := b.RecoverReadonly(ctx, storeRequest(r, ""), 0, recoveryApproval(r.Stable.State.Owner)); !errors.Is(err, ErrHandoffBusy) || calls.Load() != 1 {
		t.Fatal("recovery bypassed alive worker claim", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	shown, err := b.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Used != 1 || shown.Stable.State.Revision != 3 {
		t.Fatal(shown, err)
	}
}

func TestHandoffStoreProcessHelper(t *testing.T) {
	if os.Getenv("HANDOFF_STORE_HELPER") != "1" {
		return
	}
	factory := readonlyHandoffFactory
	if os.Getenv("HANDOFF_STORE_CRASH_AFTER_RETURN") == "1" {
		factory = func() (*Handoff, error) {
			h, err := readonlyHandoffFactory()
			if err == nil {
				h.dispatcher.Roles["timeout-reader"] = Role{Name: "timeout-reader", Tools: []string{"read-timeout"}, Run: func(ctx context.Context, task Task, box *Toolbox) (Result, error) {
					if _, err := box.Read(ctx, "read-timeout"); err != nil {
						return Result{}, err
					}
					os.Exit(42)
					return Result{}, nil
				}}
			}
			return h, err
		}
	}
	if os.Getenv("HANDOFF_STORE_CRASH") == "1" {
		factory = func() (*Handoff, error) {
			h, err := readonlyHandoffFactory()
			if err == nil {
				h.dispatcher.ParentTools["read-timeout"] = func(context.Context, Scope) (Fact, error) {
					data, readErr := os.ReadFile(filepath.Join(os.Getenv("HANDOFF_STORE_DIR"), "record.json"))
					var record HandoffFileRecord
					if readErr != nil || json.Unmarshal(data, &record) != nil || record.Schema != 5 || record.Used != 1 || len(record.Attempts) != 1 || record.Attempts[0].Sequence != 1 || record.Attempts[0].Tool != "read-timeout" {
						os.Exit(43)
					}
					os.Exit(42)
					return Fact{}, nil
				}
			}
			return h, err
		}
	}
	if os.Getenv("HANDOFF_STORE_LIMIT") == "3" {
		original := factory
		factory = func() (*Handoff, error) {
			h, err := original()
			if err == nil {
				h.dispatcher.Ledger.Limit = 3
			}
			return h, err
		}
	}
	s, err := OpenHandoffStore(os.Getenv("HANDOFF_STORE_DIR"), factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var request HandoffRequest
	if json.Unmarshal([]byte(os.Getenv("HANDOFF_STORE_REQUEST")), &request) != nil {
		t.Fatal("request shape")
	}
	record, err := s.Apply(context.Background(), os.Getenv("HANDOFF_STORE_OP"), request)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(record)
	t.Logf("handoff_record=%s", data)
}

func storeChild(t *testing.T, dir, operation string, request HandoffRequest, crash bool) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	data, _ := json.Marshal(request)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHandoffStoreProcessHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "HANDOFF_STORE_HELPER=1", "HANDOFF_STORE_DIR="+dir, "HANDOFF_STORE_OP="+operation, "HANDOFF_STORE_REQUEST="+string(data))
	if crash {
		cmd.Env = append(cmd.Env, "HANDOFF_STORE_CRASH=1")
	}
	return cmd.CombinedOutput()
}

func TestHandoffStoreResumesAcrossProcesses(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	s, err := OpenHandoffStore(dir, readonlyHandoffFactory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if output, err := storeChild(t, dir, "init", HandoffRequest{}, false); err != nil {
		t.Fatal(string(output), err)
	}
	r, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	old := storeRequest(r, "")
	for _, operation := range []string{"execute", "transfer", "execute", "complete"} {
		to := ""
		if operation == "transfer" {
			to = "retry-reader"
		}
		if output, err := storeChild(t, dir, operation, storeRequest(r, to), false); err != nil {
			t.Fatal(operation, string(output), err)
		}
		r, err = s.Apply(context.Background(), "show", HandoffRequest{})
		if err != nil {
			t.Fatal(err)
		}
	}
	if r.Phase != "stable" || r.Stable.State.Status != "completed" || r.Used != 2 || len(r.Stable.State.Facts) != 2 || r.Stable.State.Revision != 7 || len(r.Attempts) != 2 || r.Attempts[0].Actor != "timeout-reader" || r.Attempts[1].Actor != "retry-reader" {
		t.Fatal("role/task/budget not preserved", r)
	}
	if _, err := s.Apply(context.Background(), "execute", old); !errors.Is(err, ErrHandoff) {
		t.Fatal("old request executed after process handoff", err)
	}
}

func TestHandoffStoreHardExitKeepsUnknown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	s, err := OpenHandoffStore(dir, readonlyHandoffFactory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = storeChild(t, dir, "execute", storeRequest(r, ""), true)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatal("child did not hard-exit in tool", err)
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); !errors.Is(err, ErrHandoffBusy) {
		t.Fatal("hard-exit claim disappeared", err)
	}
	// The test has waited for its own child to terminate; no public automatic clear exists.
	if err := os.Remove(filepath.Join(dir, "writer.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); !errors.Is(err, ErrHandoffUnknown) {
		t.Fatal("unknown execution became a fresh attempt", err)
	}
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Phase != "executing" || shown.Used != 1 || len(shown.Attempts) != 1 {
		t.Fatal(shown, err)
	}
}

func TestHandoffStoreKnownFailureDoesNotRefreshBudget(t *testing.T) {
	s, err := OpenHandoffStore(filepath.Join(t.TempDir(), "private"), readonlyHandoffFactory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		r, err = s.Apply(context.Background(), "execute", storeRequest(r, ""))
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); !errors.Is(err, ErrBudget) {
		t.Fatal("exhausted budget resumed", err)
	}
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Phase != "failed" || shown.Used != 2 {
		t.Fatal("known failure or spent budget lost", shown, err)
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); !errors.Is(err, ErrHandoffFailed) {
		t.Fatal("known failure became unknown or executable", err)
	}
}
