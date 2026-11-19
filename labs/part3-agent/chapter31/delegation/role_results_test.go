package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func missingRoleFactory() (*Handoff, error) {
	h, err := readonlyHandoffFactory()
	if err == nil {
		h.dispatcher.Roles["timeout-reader"] = Role{Name: "timeout-reader", Tools: []string{"read-timeout"}, Run: func(_ context.Context, task Task, _ *Toolbox) (Result, error) {
			return Result{TaskID: task.ID, Parent: task.Parent, Actor: "timeout-reader", Scope: task.Scope, Missing: []string{"timeout"}}, nil
		}}
	}
	return h, err
}

func TestRoleResultsKeepMissingAfterReopenAndCompletion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	s, err := OpenHandoffStore(dir, missingRoleFactory, "missing-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"execute", "transfer", "execute", "complete"} {
		to := ""
		if operation == "transfer" {
			to = "retry-reader"
		}
		r, err = s.Apply(context.Background(), operation, storeRequest(r, to))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenHandoffStore(dir, missingRoleFactory, "missing-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Schema != 5 || shown.Stable.State.Status != "completed" || shown.Used != 1 || len(shown.RoleResults) != 2 || len(shown.RoleResults[0].Result.Missing) != 1 || shown.RoleResults[0].Result.Missing[0] != "timeout" || len(shown.RoleResults[1].Result.Facts) != 1 {
		t.Fatal("missing erased by restart or completed", shown, err)
	}
}

func TestRoleResultsRejectBadIdentityOrPayload(t *testing.T) {
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
	r, err = s.Apply(context.Background(), "execute", storeRequest(r, ""))
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(r)
	for _, change := range []func(*HandoffFileRecord){
		func(r *HandoffFileRecord) { r.RoleResults[0].Result.Parent = "other" },
		func(r *HandoffFileRecord) { r.RoleResults[0].Result.Actor = "unknown" },
		func(r *HandoffFileRecord) { r.RoleResults[0].Turn = 2 },
		func(r *HandoffFileRecord) { r.RoleResults[0].Result.Scope.User = "other" },
		func(r *HandoffFileRecord) { r.RoleResults[0].Result.Facts[0].Value = "wrong" },
		func(r *HandoffFileRecord) { r.RoleResults[0].Result.Missing = []string{strings.Repeat("x", 257)} },
		func(r *HandoffFileRecord) { r.RoleResults = nil },
	} {
		var bad HandoffFileRecord
		if json.Unmarshal(original, &bad) != nil {
			t.Fatal("invalid produced record")
		}
		change(&bad)
		data, _ := json.Marshal(bad)
		if err := os.WriteFile(filepath.Join(dir, "record.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Apply(context.Background(), "show", HandoffRequest{}); !errors.Is(err, ErrHandoff) {
			t.Fatal("bad archived result accepted", err)
		}
	}
}

func TestSchema2MigrationDoesNotInventRoleHistory(t *testing.T) {
	s, err := OpenHandoffStore(filepath.Join(t.TempDir(), "private"), readonlyHandoffFactory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Apply(context.Background(), "execute", storeRequest(r, ""))
	if err != nil {
		t.Fatal(err)
	}
	r.Schema, r.InitialTurns, r.RoleResults = 2, 0, nil
	for n := range r.Attempts {
		r.Attempts[n].Outcome, r.Attempts[n].ErrorCode, r.Attempts[n].Raw = "", "", Fact{}
	}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	r, err = s.Apply(context.Background(), "transfer", storeRequest(r, "retry-reader"))
	if err != nil || r.Schema != 5 || r.InitialTurns != 1 || len(r.RoleResults) != 0 || r.Used != 1 {
		t.Fatal("old history fabricated or counts reset", r, err)
	}
	r, err = s.Apply(context.Background(), "execute", storeRequest(r, ""))
	if err != nil || len(r.RoleResults) != 1 || r.RoleResults[0].Turn != 2 || r.Used != 2 {
		t.Fatal(r, err)
	}
}
