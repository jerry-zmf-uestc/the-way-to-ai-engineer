package main

import "testing"

func TestSummary(t *testing.T) {
	tests := []struct {
		name    string
		version int
		stable  bool
		want    string
	}{
		{name: "stable", version: 1, stable: true, want: "Go example v1 is stable"},
		{name: "experimental", version: 2, stable: false, want: "Go example v2 is experimental"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summary(tt.version, tt.stable); got != tt.want {
				t.Fatalf("summary(%d, %v) = %q, want %q", tt.version, tt.stable, got, tt.want)
			}
		})
	}
}
