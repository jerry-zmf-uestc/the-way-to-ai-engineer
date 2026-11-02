package main

import "testing"

func TestLabelScore(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{score: -1, want: "invalid"},
		{score: 59, want: "retry"},
		{score: 60, want: "pass"},
		{score: 90, want: "excellent"},
		{score: 101, want: "invalid"},
	}

	for _, tt := range tests {
		if got := labelScore(tt.score); got != tt.want {
			t.Fatalf("labelScore(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestSumEven(t *testing.T) {
	if got := sumEven(10); got != 30 {
		t.Fatalf("sumEven(10) = %d, want 30", got)
	}
}
