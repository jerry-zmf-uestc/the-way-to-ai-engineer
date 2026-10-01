package main

import (
	"errors"
	"slices"
	"testing"
)

func TestParseInts(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := parseInts([]string{"1", "2"})
		if err != nil {
			t.Fatalf("parseInts() error = %v", err)
		}
		if !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("parseInts() = %v, want [1 2]", got)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := parseInts(nil); !errors.Is(err, errNoInput) {
			t.Fatalf("parseInts(nil) error = %v, want %v", err, errNoInput)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := parseInts([]string{"x"}); err == nil {
			t.Fatal("parseInts() error = nil, want error")
		}
	})
}

func TestSumAvg(t *testing.T) {
	sum, avg, err := sumAvg([]int{1, 2, 3})
	if err != nil {
		t.Fatalf("sumAvg() error = %v", err)
	}
	if sum != 6 || avg != 2 {
		t.Fatalf("sumAvg() = (%d, %.2f), want (6, 2.00)", sum, avg)
	}

	if _, _, err := sumAvg(nil); !errors.Is(err, errEmptyNumbers) {
		t.Fatalf("sumAvg(nil) error = %v, want %v", err, errEmptyNumbers)
	}
}
