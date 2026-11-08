package main

import (
	"slices"
	"testing"
)

func TestInstanceDown(t *testing.T) {
	got, err := exercise()
	want := []observation{{"A", "A", 1, 0}, {"A", "", 0, 1}, {"B", "B", 1, 0}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}
