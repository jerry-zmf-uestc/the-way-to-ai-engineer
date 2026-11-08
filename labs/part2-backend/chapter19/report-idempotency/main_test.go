package main

import "testing"

func TestResponseLostAfterCommit(t *testing.T) {
	for _, tc := range []struct {
		reuse bool
		want  observation
	}{
		{false, observation{true, "export-1", "export-2", 2}},
		{true, observation{true, "export-1", "export-1", 1}},
	} {
		got, err := exercise(tc.reuse)
		if err != nil || got != tc.want {
			t.Fatalf("reuse=%t: got %+v, %v; want %+v", tc.reuse, got, err, tc.want)
		}
	}
}

func TestSameKeyDifferentInput(t *testing.T) {
	s := repository{jobs: make(map[string]record)}
	original, _, _ := s.submit("key", "csv")
	got, created, matched := s.submit("key", "json")
	if got != original || created || matched || len(s.jobs) != 1 {
		t.Fatalf("conflicting input changed the original: %+v", s.jobs)
	}
}
