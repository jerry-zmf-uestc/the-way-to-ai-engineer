package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceDigestUsesBytesNotHexLetterCase(t *testing.T) {
	dir := t.TempDir()
	if err := fixture(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "incident-diagnosis", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := hash([]byte("Use only approved readonly tools.\n"))
	if err = os.WriteFile(path, []byte(strings.Replace(string(data), expected, strings.ToUpper(expected), 1)), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	l, err := New(root, 1024, map[string]bool{"incident-diagnosis": true}, map[string]bool{"read-doc": true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.Load(context.Background(), "incident-diagnosis"); err != nil {
		t.Fatal(err)
	}
	text, err := l.ReadReference(context.Background(), "incident-diagnosis", "references/guide.md")
	if err != nil || text != "Use only approved readonly tools.\n" {
		t.Fatal(text, err)
	}
	// Fresh loader must still reject changed bytes; old cache is intentionally a snapshot.
	if err = os.WriteFile(filepath.Join(dir, "incident-diagnosis", "references", "guide.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	fresh, err := New(root, 1024, map[string]bool{"incident-diagnosis": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fresh.Load(context.Background(), "incident-diagnosis"); err != nil {
		t.Fatal(err)
	}
	if _, err = fresh.ReadReference(context.Background(), "incident-diagnosis", "references/guide.md"); !errors.Is(err, ErrSkill) {
		t.Fatal("changed content accepted", err)
	}
}
