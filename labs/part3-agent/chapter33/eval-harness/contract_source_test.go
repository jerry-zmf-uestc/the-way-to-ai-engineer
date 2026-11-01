package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Frozen datasets keep their original bytes; the book manifest records source moves.
func assertContractSource(t *testing.T, root, source string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, source)); err == nil {
		return
	}
	raw, err := os.ReadFile(filepath.Join(root, "book", "chapters.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pages map[string]struct {
		Path          string   `json:"path"`
		LegacySources []string `json:"legacySources"`
	}
	if err := json.Unmarshal(raw, &pages); err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		if slices.Contains(page.LegacySources, source) {
			if _, err := os.Stat(filepath.Join(root, page.Path)); err != nil {
				t.Fatal("moved contract source missing", source, err)
			}
			return
		}
	}
	t.Fatal("contract source has no current file or declared move", source)
}
