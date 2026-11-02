package main

import (
	"reflect"
	"slices"
	"testing"
)

func TestCountWords(t *testing.T) {
	got := countWords([]string{"go", "learn", "go", ""})
	want := map[string]int{"go": 2, "learn": 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("countWords() = %v, want %v", got, want)
	}
}

func TestLookup(t *testing.T) {
	counts := map[string]int{"go": 2}
	if got, ok := lookup(counts, "go"); !ok || got != 2 {
		t.Fatalf("lookup existing = (%d, %v), want (2, true)", got, ok)
	}
	if got, ok := lookup(counts, "missing"); ok || got != 0 {
		t.Fatalf("lookup missing = (%d, %v), want (0, false)", got, ok)
	}
}

func TestUniqueWords(t *testing.T) {
	got := uniqueWords([]string{"go", "learn", "go", "", "test"})
	want := []string{"go", "learn", "test"}
	if !slices.Equal(got, want) {
		t.Fatalf("uniqueWords() = %v, want %v", got, want)
	}
}
