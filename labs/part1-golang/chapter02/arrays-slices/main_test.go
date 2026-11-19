package main

import (
	"slices"
	"testing"
)

func TestAppendCopy(t *testing.T) {
	base := []int{1, 2, 3}
	got := appendCopy(base, 4, 5)
	want := []int{1, 2, 3, 4, 5}

	if !slices.Equal(got, want) {
		t.Fatalf("appendCopy() = %v, want %v", got, want)
	}
	if !slices.Equal(base, []int{1, 2, 3}) {
		t.Fatalf("appendCopy mutated base: %v", base)
	}
}

func TestWindow(t *testing.T) {
	values := []int{1, 2, 3, 4}
	if got, want := window(values, 1, 3), []int{2, 3}; !slices.Equal(got, want) {
		t.Fatalf("window() = %v, want %v", got, want)
	}
	if got := window(values, 2, 2); len(got) != 0 {
		t.Fatalf("empty window = %v, want empty slice", got)
	}
	if got := window(values, 3, 1); got != nil {
		t.Fatalf("invalid window = %v, want nil", got)
	}
}
