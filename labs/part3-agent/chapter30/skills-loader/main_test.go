package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loader(t *testing.T, budget int) (*Loader, string) {
	t.Helper()
	dir := t.TempDir()
	if err := fixture(dir); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	l, err := New(root, budget, map[string]bool{"incident-diagnosis": true}, map[string]bool{"read-doc": true})
	if err != nil {
		t.Fatal(err)
	}
	return l, dir
}
func TestProgressiveLoadingAndPermissionIsolation(t *testing.T) {
	l, _ := loader(t, 1024)
	if len(l.loaded) != 0 || l.Used != 0 || len(l.Catalog()) != 2 {
		t.Fatal("eager instruction load")
	}
	a, err := l.Load(context.Background(), "incident-diagnosis")
	if err != nil || a.Text == "" {
		t.Fatal(err)
	}
	used := l.Used
	b, err := l.Load(context.Background(), a.Name)
	if err != nil || a != b || l.Used != used {
		t.Fatal("duplicate loading")
	}
	if !errors.Is(l.AuthorizeTool("run-command"), ErrDenied) || l.AuthorizeTool("read-doc") != nil {
		t.Fatal("skill changed tools")
	}
	if _, err := l.Load(context.Background(), "report-format"); !errors.Is(err, ErrDenied) {
		t.Fatal("activation widened")
	}
}
func TestBudgetVersionAndPathRejection(t *testing.T) {
	l, dir := loader(t, 4)
	if _, err := l.Load(context.Background(), "incident-diagnosis"); !errors.Is(err, ErrBudget) || l.Used != 0 {
		t.Fatal("budget partial commit")
	}
	for _, name := range []string{"../report-format", "missing"} {
		if _, err := l.Load(context.Background(), name); !errors.Is(err, ErrDenied) {
			t.Fatal("unregistered name accepted")
		}
	}
	path := filepath.Join(dir, "incident-diagnosis", "SKILL.md")
	data, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(data), "version: v1", "version: v2", 1)), 0600)
	if _, err := l.Load(context.Background(), "incident-diagnosis"); !errors.Is(err, ErrSkill) {
		t.Fatal("catalog version changed silently")
	}
}
func TestSymlinkEscapeAndCancellation(t *testing.T) {
	l, dir := loader(t, 1024)
	path := filepath.Join(dir, "incident-diagnosis", "SKILL.md")
	outside := filepath.Join(t.TempDir(), "outside.md")
	os.WriteFile(outside, []byte("outside"), 0600)
	os.Remove(path)
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Load(context.Background(), "incident-diagnosis"); err == nil {
		t.Fatal("outside path read")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Load(ctx, "incident-diagnosis"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancel ignored")
	}
}

func TestSelectionAndReferenceBudget(t *testing.T) {
	l, _ := loader(t, 1024)
	names := l.Select("timeout")
	if len(names) != 1 || names[0] != "incident-diagnosis" {
		t.Fatal("metadata selection")
	}
	if _, err := l.ReadReference(context.Background(), names[0], "references/guide.md"); !errors.Is(err, ErrDenied) {
		t.Fatal("reference loaded before activation")
	}
	if _, err := l.Load(context.Background(), names[0]); err != nil {
		t.Fatal(err)
	}
	before := l.Used
	text, err := l.ReadReference(context.Background(), names[0], "references/guide.md")
	if err != nil || text == "" || l.Used <= before {
		t.Fatal("reference not budgeted")
	}
	used := l.Used
	if _, err := l.ReadReference(context.Background(), names[0], "references/guide.md"); err != nil || l.Used != used {
		t.Fatal("reference duplication")
	}
	if _, err := l.ReadReference(context.Background(), names[0], "references/../../report-format/SKILL.md"); !errors.Is(err, ErrDenied) {
		t.Fatal("reference traversal")
	}
}

func TestReferenceContentVersionBinding(t *testing.T) {
	l, dir := loader(t, 1024)
	if _, err := l.Load(context.Background(), "incident-diagnosis"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "incident-diagnosis", "references", "guide.md"), []byte("changed without version"), 0600)
	if _, err := l.ReadReference(context.Background(), "incident-diagnosis", "references/guide.md"); !errors.Is(err, ErrSkill) {
		t.Fatal("mixed resource version")
	}
}
