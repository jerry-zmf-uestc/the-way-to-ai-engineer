package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestHardExitBeforeRenamePreservesCommitAndBlocksTempReuse(t *testing.T) {
	dir := t.TempDir()
	store := open(t, dir)
	release, err := store.claim()
	if err != nil {
		t.Fatal(err)
	}
	original := State{Schema: 2, Identity: id(), PlanHash: fingerprint(actions), Phase: "pending", MaxAttempts: 8}
	if err = store.Save(original); err != nil {
		t.Fatal(err)
	}
	if err = release(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestCrashWorker$")
	command.Env = append(os.Environ(), "CHECKPOINT_WORKER=1", "CHECKPOINT_DIR="+dir, "CHECKPOINT_STAGE=before_rename")
	var exit *exec.ExitError
	if err = command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 93 {
		t.Fatal("not confirmed terminated", err)
	}
	committed, err := store.Load()
	if err != nil || committed.Cursor != 0 || committed.Attempts != 0 || committed.Phase != "pending" {
		t.Fatal("temporary state promoted", committed, err)
	}
	if err = validate(committed, id(), actions); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Root.Stat("checkpoint.tmp"); err != nil {
		t.Fatal("expected leftover temporary", err)
	}
	if _, err = store.Root.Stat("calls.log"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("tool called before commit", err)
	}
	// The child is now known dead; cleanup is an explicit operator action, not a lease timeout.
	if err = store.Root.Remove("writer.lock"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	read := func(context.Context, string) (string, error) { calls++; return "fixed", nil }
	if _, err = Run(context.Background(), store, id(), actions, 8, true, read, nil); !errors.Is(err, os.ErrExist) || calls != 0 {
		t.Fatal("stale temp silently reused", calls, err)
	}
	if err = store.Root.Remove("checkpoint.tmp"); err != nil {
		t.Fatal(err)
	}
	state, err := Run(context.Background(), store, id(), actions, 8, true, read, nil)
	if err != nil || calls != 2 || state.Cursor != 2 || state.Phase != "completed" {
		t.Fatal(state, calls, err)
	}
}
