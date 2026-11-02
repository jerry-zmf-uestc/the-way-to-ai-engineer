package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontmatterDoesNotCoerceScalarOrMetadataTypes(t *testing.T) {
	base := "---\nname: test-skill\ndescription: Read evidence.\nmetadata:\n  version: v1\n---\nbody"
	for _, change := range [][2]string{
		{"name: test-skill", "name: 123"},
		{"description: Read evidence.", "description: 123"},
		{"description: Read evidence.", "description: true"},
		{"description: Read evidence.", "description: null"},
		{"  version: v1", "  version: 1"},
		{"  version: v1", "  version: v1\n  author: false"},
		{"  version: v1", "  version: v1\n  123: author"},
		{"description: Read evidence.", "description: Read evidence.\nlicense: 123"},
		{"description: Read evidence.", "description: Read evidence.\nallowed-tools: false"},
	} {
		raw := strings.Replace(base, change[0], change[1], 1)
		if _, _, err := header([]byte(raw)); !errors.Is(err, ErrSkill) {
			t.Errorf("wrong YAML type accepted: %s: %v", change[1], err)
		}
	}
	valid := "---\nname: '123'\ndescription: |\n  Read evidence.\n  Keep sources.\nlicense: '123'\nallowed-tools: 'false'\nmetadata:\n  version: '1'\n  author: 'false'\n---\nbody"
	if meta, _, err := header([]byte(valid)); err != nil || meta.Name != "123" || meta.Metadata["version"] != "1" {
		t.Fatal("quoted strings or multiline description rejected", meta, err)
	}
}

func TestYAMLTypesAreCheckedDuringDiscoveryAndActivation(t *testing.T) {
	l, dir := loader(t, 1024)
	path := filepath.Join(dir, "incident-diagnosis", "SKILL.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "description:") {
			lines[i] = "description: 123"
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Load(context.Background(), "incident-diagnosis"); !errors.Is(err, ErrSkill) || l.Used != 0 {
		t.Fatal("changed header loaded or partially charged", err, l.Used)
	}
	if next, err := New(l.root, 1024, map[string]bool{"incident-diagnosis": true}, nil); !errors.Is(err, ErrSkill) || next != nil {
		t.Fatal("discovery accepted wrong scalar type", next, err)
	}
}
