package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func recoveryApproval(actor string) HandoffRecoveryApproval {
	return HandoffRecoveryApproval{"readonly-demo-v1", actor, true, true}
}

func TestReadonlyRecoveryPreservesCrashedAttemptsAndControlGeneration(t *testing.T) {
	for _, afterReturn := range []bool{false, true} {
		t.Run(map[bool]string{false: "reserved", true: "returned"}[afterReturn], func(t *testing.T) {
			t.Setenv("HANDOFF_STORE_LIMIT", "3")
			if afterReturn {
				t.Setenv("HANDOFF_STORE_CRASH_AFTER_RETURN", "1")
			}
			factory := func() (*Handoff, error) {
				h, err := readonlyHandoffFactory()
				if err == nil {
					h.dispatcher.Ledger.Limit = 3
				}
				return h, err
			}
			s, err := OpenHandoffStore(filepath.Join(t.TempDir(), "private"), factory, "readonly-demo-v1")
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r, err := s.Apply(context.Background(), "init", HandoffRequest{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = storeChild(t, s.root.Name(), "execute", storeRequest(r, ""), !afterReturn)
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 42 {
				t.Fatal("child did not reach fixed crash", err)
			}
			approval := recoveryApproval(r.Stable.State.Owner)
			if _, err = s.RecoverReadonly(context.Background(), storeRequest(r, ""), 0, approval); !errors.Is(err, ErrHandoffBusy) {
				t.Fatal("recovery removed surviving claim", err)
			}
			// Wait above confirmed exit; remove only this test's private stale claim.
			if err = s.root.Remove("writer.lock"); err != nil {
				t.Fatal(err)
			}
			r, err = s.Apply(context.Background(), "show", HandoffRequest{})
			if err != nil || r.Phase != "executing" || r.Used != 1 || len(r.Attempts) != 1 {
				t.Fatal(r, err)
			}
			old := r.Attempts[0]
			denied := approval
			denied.PreviousStopped = false
			if _, err = s.RecoverReadonly(context.Background(), storeRequest(r, ""), 0, denied); !errors.Is(err, ErrDenied) {
				t.Fatal("implicit recovery accepted", err)
			}
			r, err = s.RecoverReadonly(context.Background(), storeRequest(r, ""), 0, approval)
			if err != nil || r.Phase != "stable" || r.Generation != 1 || r.Used != 1 || r.Stable.Used != 1 || r.Attempts[0] != old || len(r.RoleResults) != 0 {
				t.Fatal("recovery invented completion or reset usage", r, err)
			}
			if _, err = s.Apply(context.Background(), "execute", storeRequest(r, "")); !errors.Is(err, ErrHandoff) {
				t.Fatal("legacy request reused after recovery", err)
			}
			if _, err = s.RecoverReadonly(context.Background(), storeRequest(r, ""), 0, approval); !errors.Is(err, ErrHandoff) {
				t.Fatal("duplicate recovery accepted", err)
			}
			for _, op := range []struct{ operation, to string }{{"execute", ""}, {"transfer", "retry-reader"}, {"execute", ""}, {"complete", ""}} {
				r, err = s.ApplyVersioned(context.Background(), op.operation, storeRequest(r, op.to), 1)
				if err != nil {
					t.Fatal(op, err)
				}
			}
			if r.Stable.State.Status != "completed" || r.Used != 3 || len(r.Attempts) != 3 || len(r.RoleResults) != 2 || r.Attempts[0] != old || r.Attempts[1].Generation != 1 || r.Attempts[2].Generation != 1 {
				t.Fatal("bounded recovery result incomplete", r)
			}
			facts := map[string]string{}
			for _, fact := range r.Stable.State.Facts {
				facts[fact.Key] = fact.Value
			}
			if len(facts) != 2 || facts["timeout"] != "30" || facts["retry"] != "2" || len(r.RoleResults[0].Result.Missing) != 0 || len(r.RoleResults[1].Result.Missing) != 0 {
				t.Fatal("fixed report goal not covered", facts, r.RoleResults)
			}
			shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
			if err != nil || shown.Generation != 1 || shown.Used != 3 {
				t.Fatal("persisted recovery lost", shown, err)
			}
		})
	}
}

func TestReadonlyRecoveryRejectsBudgetPolicyAndForgedHistory(t *testing.T) {
	s, err := OpenHandoffStore(filepath.Join(t.TempDir(), "private"), readonlyHandoffFactory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	r.Phase = "executing"
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	request := storeRequest(r, "")
	wrong := request
	wrong.Scope.User = "other"
	if _, err = s.RecoverReadonly(context.Background(), wrong, 0, recoveryApproval(request.From)); !errors.Is(err, ErrHandoff) {
		t.Fatal("wrong scope recovered", err)
	}
	wrong = request
	wrong.Revision++
	if _, err = s.RecoverReadonly(context.Background(), wrong, 0, recoveryApproval(request.From)); !errors.Is(err, ErrHandoff) {
		t.Fatal("wrong revision recovered", err)
	}
	for _, approval := range []HandoffRecoveryApproval{{"other-version", request.From, true, true}, {"readonly-demo-v1", request.From, false, true}, {"readonly-demo-v1", "retry-reader", true, true}} {
		if _, err := s.RecoverReadonly(context.Background(), request, 0, approval); !errors.Is(err, ErrDenied) {
			t.Fatal("wrong approval accepted", err)
		}
	}
	r.Used = 2
	r.Attempts = []HandoffToolAttempt{{Sequence: 1, Actor: request.From, Tool: "read-timeout", Revision: 2, Outcome: "reserved"}, {Sequence: 2, Actor: request.From, Tool: "read-timeout", Revision: 2, Outcome: "reserved"}}
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecoverReadonly(context.Background(), request, 0, recoveryApproval(request.From)); !errors.Is(err, ErrBudget) {
		t.Fatal("spent quota recovered", err)
	}
	r.Schema, r.Generation = 4, 1
	r.Recoveries = []HandoffReadonlyRecovery{{1, request.From, request.Revision, 2}}
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), "show", HandoffRequest{}); !errors.Is(err, ErrHandoff) {
		t.Fatal("legacy schema accepted forged recovery", err)
	}
	if _, err = os.Stat(filepath.Join(s.root.Name(), "writer.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed operation leaked claim", err)
	}
}

func TestReadonlyRecoveryBoundsRepeatedPreToolCrashesAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := false
	factory := func() (*Handoff, error) {
		if stop {
			cancel()
		}
		return readonlyHandoffFactory()
	}
	s, err := OpenHandoffStore(filepath.Join(t.TempDir(), "private"), factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	r.Phase = "executing"
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	stop = true
	if _, err = s.RecoverReadonly(ctx, storeRequest(r, ""), 0, recoveryApproval(r.Stable.State.Owner)); !errors.Is(err, context.Canceled) {
		t.Fatal("late cancellation committed recovery", err)
	}
	stop = false
	r, err = s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || r.Generation != 0 || r.Phase != "executing" {
		t.Fatal("canceled operation changed record", r, err)
	}
	for generation := 0; generation < 4; generation++ {
		r.Phase = "executing" // Simulate interruption before the tool budget hook.
		if err = s.save(r); err != nil {
			t.Fatal(err)
		}
		r, err = s.RecoverReadonly(context.Background(), storeRequest(r, ""), generation, recoveryApproval(r.Stable.State.Owner))
		if err != nil || r.Generation != generation+1 || r.Used != 0 || r.Stable.State.Turns != 0 {
			t.Fatal("recovery counter invented execution", r, err)
		}
	}
	r.Phase = "executing"
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecoverReadonly(context.Background(), storeRequest(r, ""), 4, recoveryApproval(r.Stable.State.Owner)); !errors.Is(err, ErrBudget) {
		t.Fatal("pre-tool crashes bypassed role budget", err)
	}
}
