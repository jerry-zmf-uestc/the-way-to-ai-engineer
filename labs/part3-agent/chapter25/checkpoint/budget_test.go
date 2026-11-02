package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReopenedRunKeepsOriginalAttemptLimit(t *testing.T) {
	dir := t.TempDir()
	store := open(t, dir)
	state, err := Run(context.Background(), store, id(), actions, 2, true, func(context.Context, string) (string, error) {
		return "", ErrUnknown
	}, nil)
	if !errors.Is(err, ErrUnknown) || state.Attempts != 1 || state.MaxAttempts != 2 || state.Phase != "in_flight" {
		t.Fatal(state, err)
	}
	if err := store.Root.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := open(t, dir)
	before, err := os.ReadFile(filepath.Join(dir, "checkpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{1, 8} {
		_, err := Run(context.Background(), reopened, id(), actions, limit, true, func(context.Context, string) (string, error) {
			t.Fatal("changed budget executed")
			return "", nil
		}, nil)
		after, readErr := os.ReadFile(filepath.Join(dir, "checkpoint.json"))
		if !errors.Is(err, ErrState) || readErr != nil || !bytes.Equal(before, after) {
			t.Fatal("mismatch changed persistent state", limit, err, readErr)
		}
	}
	calls := 0
	state, err = Run(context.Background(), reopened, id(), actions, 2, true, func(_ context.Context, action string) (string, error) {
		calls++
		if action != actions[0] {
			t.Fatal("advanced beyond remaining budget", action)
		}
		return "fixed", nil
	}, nil)
	if !errors.Is(err, ErrBudget) || calls != 1 || state.Attempts != 2 || state.Cursor != 1 || state.MaxAttempts != 2 {
		t.Fatal(state, err, calls)
	}
	_, err = Run(context.Background(), reopened, id(), actions, 2, true, func(context.Context, string) (string, error) {
		t.Fatal("exhausted restart got a new budget")
		return "", nil
	}, nil)
	if !errors.Is(err, ErrBudget) {
		t.Fatal(err)
	}
}

func TestLegacyAndInvalidAttemptLimitsDoNotResume(t *testing.T) {
	for _, mutate := range []func(*State){
		func(s *State) { s.Schema = 1; s.MaxAttempts = 0 },
		func(s *State) { s.MaxAttempts = 0 },
		func(s *State) { s.MaxAttempts = 33 },
		func(s *State) { s.Attempts = 3 },
	} {
		store := open(t, t.TempDir())
		state := State{Schema: 2, Identity: id(), PlanHash: fingerprint(actions), Phase: "pending", MaxAttempts: 2}
		mutate(&state)
		if err := store.Save(state); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err != nil {
			t.Fatal("raw state inspection failed", err)
		}
		if _, err := Run(context.Background(), store, id(), actions, 2, true, func(context.Context, string) (string, error) {
			t.Fatal("unbound or invalid budget executed")
			return "", nil
		}, nil); !errors.Is(err, ErrState) {
			t.Fatal(state, err)
		}
	}
}
