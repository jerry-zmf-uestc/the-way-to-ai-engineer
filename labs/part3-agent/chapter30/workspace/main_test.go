package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, scope Scope, now func() time.Time) (*Workspace, *os.Root) {
	t.Helper()
	base, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { base.Close() })
	w, err := New(base, scope, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w, base
}
func TestPaginationAndTaskIsolation(t *testing.T) {
	w, base := fixture(t, Scope{"t", "u", "task-a"}, time.Now)
	a, err := w.Publish(context.Background(), "a", "甲\n乙\n丙\n", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	page, err := w.Read(context.Background(), a.ID, 0, 2)
	if err != nil || page.Text != "甲\n乙\n" || page.NextLine != 2 || page.Complete {
		t.Fatalf("page=%+v %v", page, err)
	}
	page, err = w.Read(context.Background(), a.ID, page.NextLine, 2)
	if err != nil || !page.Complete || page.Text != "丙\n" {
		t.Fatal("page continuation")
	}
	other, err := New(base, Scope{"t", "u", "task-b"}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.Read(context.Background(), "a", 0, 1); !errors.Is(err, ErrDenied) {
		t.Fatal("cross-task read")
	}
	for _, id := range []string{"../a", "/a", "unknown"} {
		if _, err := w.Read(context.Background(), id, 0, 1); !errors.Is(err, ErrDenied) {
			t.Fatal("arbitrary path accepted")
		}
	}
}
func TestExpiryQuotaAndIDTombstone(t *testing.T) {
	now := time.Now()
	w, _ := fixture(t, Scope{"t", "u", "task"}, func() time.Time { return now })
	w.Publish(context.Background(), "a", "one", time.Second)
	now = now.Add(time.Second)
	if _, err := w.Read(context.Background(), "a", 0, 1); !errors.Is(err, ErrDenied) || len(w.List()) != 0 {
		t.Fatal("expired artifact exposed")
	}
	if err := w.Cleanup(); err != nil || w.Used != 0 {
		t.Fatal("cleanup quota")
	}
	if _, err := w.Publish(context.Background(), "a", "new", time.Hour); !errors.Is(err, ErrDenied) {
		t.Fatal("old ID reused")
	}
	if _, err := w.Publish(context.Background(), "big", strings.Repeat("x", (32<<10)+1), time.Hour); !errors.Is(err, ErrQuota) {
		t.Fatal("size cap")
	}
}
func TestTamperAndSymlinkEscape(t *testing.T) {
	w, _ := fixture(t, Scope{"t", "u", "task"}, time.Now)
	w.Publish(context.Background(), "a", "original", time.Hour)
	a := w.artifacts["a"]
	f, err := w.root.OpenFile(a.path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("changed")
	f.Close()
	if _, err := w.Read(context.Background(), "a", 0, 1); !errors.Is(err, ErrChanged) {
		t.Fatal("tampering ignored")
	}
	outside := filepath.Join(t.TempDir(), "private.txt")
	os.WriteFile(outside, []byte("outside"), 0600)
	w.root.Remove(a.path)
	if err := w.root.Symlink(outside, a.path); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Read(context.Background(), "a", 0, 1); err == nil {
		t.Fatal("symlink escape")
	}
}
func TestCancellationAndInvalidScope(t *testing.T) {
	w, base := fixture(t, Scope{"t", "u", "task"}, time.Now)
	if _, err := New(base, Scope{"../t", "u", "task"}, time.Now); !errors.Is(err, ErrDenied) {
		t.Fatal("invalid scope")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.Publish(ctx, "a", "text", time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled publish")
	}
}
