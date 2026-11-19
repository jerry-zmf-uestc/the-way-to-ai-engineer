package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func open(t *testing.T, dir string) Store {
	t.Helper()
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return Store{Root: r}
}
func id() Identity { return Identity{"t", "u", "s", "r"} }

var actions = []string{"read-A", "read-B"}

func TestCrashWorker(t *testing.T) {
	if os.Getenv("CHECKPOINT_WORKER") != "1" {
		t.Skip("helper process")
	}
	root, err := os.OpenRoot(os.Getenv("CHECKPOINT_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store := Store{Root: root, beforeRename: func() {
		if os.Getenv("CHECKPOINT_STAGE") == "before_rename" {
			os.Exit(93)
		}
	}}
	_, err = Run(context.Background(), store, id(), actions, 8, true, func(_ context.Context, action string) (string, error) {
		f, e := root.OpenFile("calls.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			return "", e
		}
		_, e = f.WriteString(action + "\n")
		f.Close()
		return "fixed:" + action, e
	}, func(stage string) {
		if stage == os.Getenv("CHECKPOINT_STAGE") {
			os.Exit(93)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestHardExitBeforeAndAfterResultCommit(t *testing.T) {
	for _, stage := range []string{"after_execute", "after_commit"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashWorker$")
			cmd.Env = append(os.Environ(), "CHECKPOINT_WORKER=1", "CHECKPOINT_DIR="+dir, "CHECKPOINT_STAGE="+stage)
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 93 {
				t.Fatalf("helper exit=%v", err)
			}
			// 已通过Wait确认子进程退出，才由测试操作者处置遗留锁；运行器不自动偷锁。
			if err := os.Remove(filepath.Join(dir, "writer.lock")); err != nil {
				t.Fatal(err)
			}
			store := open(t, dir)
			if stage == "after_execute" {
				s, err := Run(context.Background(), store, id(), actions, 8, false, func(context.Context, string) (string, error) { t.Fatal("unknown outcome executed"); return "", nil }, nil)
				if !errors.Is(err, ErrUnknown) || s.Cursor != 0 {
					t.Fatal("outcome ambiguity lost")
				}
			}
			s, err := Run(context.Background(), store, id(), actions, 8, true, func(_ context.Context, action string) (string, error) {
				f, e := store.Root.OpenFile("calls.log", os.O_APPEND|os.O_WRONLY, 0600)
				if e != nil {
					return "", e
				}
				_, e = f.WriteString(action + "\n")
				f.Close()
				return "fixed:" + action, e
			}, nil)
			if err != nil || s.Phase != "completed" || s.Cursor != 2 {
				t.Fatalf("state=%+v err=%v", s, err)
			}
			log, err := os.ReadFile(filepath.Join(dir, "calls.log"))
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if stage == "after_execute" {
				want = 2
			}
			if strings.Count(string(log), "read-A") != want || strings.Count(string(log), "read-B") != 1 {
				t.Fatalf("calls=%s", log)
			}
		})
	}
}
func TestIdentityLockSchemaAndBudget(t *testing.T) {
	s := open(t, t.TempDir())
	release, err := s.claim()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.claim(); !errors.Is(err, ErrBusy) {
		t.Fatal("duplicate claim")
	}
	release()
	state, err := Run(context.Background(), s, id(), actions, 1, true, func(context.Context, string) (string, error) { return "ok", nil }, nil)
	if !errors.Is(err, ErrBudget) || state.Cursor != 1 {
		t.Fatalf("budget=%+v %v", state, err)
	}
	foreign := id()
	foreign.Tenant = "other"
	if _, err := Run(context.Background(), s, foreign, actions, 8, true, func(context.Context, string) (string, error) { t.Fatal("foreign execute"); return "", nil }, nil); !errors.Is(err, ErrState) {
		t.Fatal("identity ignored")
	}
	state.Schema = 9
	if err := s.Save(state); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), s, id(), actions, 8, true, func(context.Context, string) (string, error) { return "", nil }, nil); !errors.Is(err, ErrState) {
		t.Fatal("unknown schema accepted")
	}
}
func TestCancellationLeavesUncertainWindow(t *testing.T) {
	s := open(t, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state, err := Run(ctx, s, id(), actions, 8, true, func(context.Context, string) (string, error) { cancel(); return "late", nil }, nil)
	if !errors.Is(err, context.Canceled) || state.Phase != "in_flight" || state.Cursor != 0 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	saved, e := s.Load()
	if e != nil || saved.Attempts != 1 || saved.Phase != "in_flight" {
		t.Fatal("uncertainty not durable")
	}
}

func TestReleaseFailureIsObservableAfterCommit(t *testing.T) {
	dir := t.TempDir()
	store := open(t, dir)
	state, err := Run(context.Background(), store, id(), actions[:1], 8, true, func(context.Context, string) (string, error) { return "fixed", nil }, func(stage string) {
		if stage == "after_commit" {
			store.Root.Close()
		}
	})
	if state.Phase != "completed" || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("cleanup failure hidden: %+v %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "writer.lock")); err != nil {
		t.Fatal("test did not leave lock")
	}
}
