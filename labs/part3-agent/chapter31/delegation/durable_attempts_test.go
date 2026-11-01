package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDurableReservationMustConfirmBeforeTool(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	calls := 0
	factory := func() (*Handoff, error) {
		h, err := readonlyHandoffFactory()
		if err == nil {
			h.dispatcher.ParentTools["read-timeout"] = func(context.Context, Scope) (Fact, error) {
				calls++
				return Fact{"timeout", "30", "guide@v1:timeout"}, nil
			}
			h.dispatcher.Roles["timeout-reader"] = Role{Name: "timeout-reader", Tools: []string{"read-timeout"}, Run: func(ctx context.Context, task Task, box *Toolbox) (Result, error) {
				// Simulate storage refusal after the execute intent but before budget confirmation.
				if err := os.WriteFile(filepath.Join(dir, ".record.tmp"), []byte("occupied"), 0600); err != nil {
					return Result{}, err
				}
				_, err := box.Read(ctx, "read-timeout")
				return Result{}, err
			}}
		}
		return h, err
	}
	s, err := OpenHandoffStore(dir, factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); err == nil || calls != 0 {
		t.Fatal("unconfirmed reservation invoked tool", err, calls)
	}
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Phase != "executing" || shown.Used != 0 {
		t.Fatal("failed save pretended committed", shown, err)
	}
}

func TestLegacyStableKeepsUnattributedConsumption(t *testing.T) {
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
	r.Schema, r.InitialUsed, r.Attempts = 1, 0, nil
	r.InitialTurns, r.RoleResults = 0, nil
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	upgraded, err := s.Apply(context.Background(), "transfer", storeRequest(r, "retry-reader"))
	if err != nil || upgraded.Schema != 5 || upgraded.InitialUsed != 1 || upgraded.Used != 1 || len(upgraded.Attempts) != 0 {
		t.Fatal("legacy consumption reset or history fabricated", upgraded, err)
	}
	upgraded, err = s.Apply(context.Background(), "execute", storeRequest(upgraded, ""))
	if err != nil || upgraded.Used != 2 || len(upgraded.Attempts) != 1 || upgraded.Attempts[0].Sequence != 2 {
		t.Fatal(upgraded, err)
	}
	upgraded.Attempts[0].Sequence = 1
	data, _ := json.Marshal(upgraded)
	if err := os.WriteFile(filepath.Join(dir, "record.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), "show", HandoffRequest{}); !errors.Is(err, ErrHandoff) {
		t.Fatal("wrong attempt identity accepted", err)
	}
}
