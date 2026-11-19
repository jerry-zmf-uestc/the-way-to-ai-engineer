package main

import (
	"slices"
	"testing"
)

func TestSum(t *testing.T) {
	t.Run("ints", func(t *testing.T) {
		got := sum([]int{2, 4, 6})
		if got != 12 {
			t.Fatalf("sum ints = %d, want 12", got)
		}
	})

	t.Run("floats", func(t *testing.T) {
		got := sum([]float64{1.5, 2.5, 3})
		if got != 7 {
			t.Fatalf("sum floats = %.1f, want 7.0", got)
		}
	})
}

func TestMapSlice(t *testing.T) {
	got := mapSlice([]string{"go", "generic"}, func(value string) int {
		return len(value)
	})

	if !slices.Equal(got, []int{2, 7}) {
		t.Fatalf("mapSlice() = %v, want [2 7]", got)
	}
}

func TestFirst(t *testing.T) {
	t.Run("returns first value", func(t *testing.T) {
		got, ok := first([]string{"cache", "database"})
		if !ok {
			t.Fatal("first() ok = false, want true")
		}
		if got != "cache" {
			t.Fatalf("first() = %q, want cache", got)
		}
	})

	t.Run("reports empty slice", func(t *testing.T) {
		got, ok := first([]int{})
		if ok {
			t.Fatalf("first() = (%d, true), want empty result", got)
		}
		if got != 0 {
			t.Fatalf("first() zero value = %d, want 0", got)
		}
	})
}

func TestSet(t *testing.T) {
	services := newSet("cache", "database", "cache")

	if !services.contains("cache") {
		t.Fatal("set contains cache = false, want true")
	}
	if services.contains("queue") {
		t.Fatal("set contains queue = true, want false")
	}
	if got := services.len(); got != 2 {
		t.Fatalf("set len = %d, want 2", got)
	}
}
