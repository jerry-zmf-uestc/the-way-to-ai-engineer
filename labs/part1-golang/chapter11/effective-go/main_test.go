package main

import (
	"errors"
	"strconv"
	"testing"
)

func TestRetryPolicyNormalizedAttempts(t *testing.T) {
	tests := []struct {
		name string
		in   retryPolicy
		want int
	}{
		{name: "zero value uses one attempt", in: retryPolicy{}, want: 1},
		{name: "configured attempts are preserved", in: retryPolicy{Attempts: 3}, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.normalizedAttempts(); got != tt.want {
				t.Fatalf("normalizedAttempts() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseQuantity(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int
		wantErr error
	}{
		{name: "trims spaces", in: " 12 ", want: 12},
		{name: "empty input", in: " ", wantErr: errEmptyQuantity},
		{name: "invalid number", in: "many", wantErr: &strconv.NumError{}},
		{name: "non-positive number", in: "0", wantErr: errNonPositiveQuantity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQuantity(tt.in)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("parseQuantity(%q) returned unexpected error: %v", tt.in, err)
				}
				if got != tt.want {
					t.Fatalf("parseQuantity(%q) = %d, want %d", tt.in, got, tt.want)
				}
				return
			}

			if err == nil {
				t.Fatalf("parseQuantity(%q) returned nil error", tt.in)
			}
			var numErr *strconv.NumError
			if errors.As(tt.wantErr, &numErr) {
				if !errors.As(err, &numErr) {
					t.Fatalf("parseQuantity(%q) error = %v, want strconv.NumError", tt.in, err)
				}
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseQuantity(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestSummarizeOrders(t *testing.T) {
	tests := []struct {
		name   string
		orders []order
		want   string
	}{
		{name: "empty", want: "no orders"},
		{name: "single", orders: []order{{ID: "A-100", Quantity: 1}}, want: "1 order"},
		{name: "multiple", orders: []order{{ID: "A-100", Quantity: 1}, {ID: "B-200", Quantity: 2}}, want: "2 orders"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := summarizeOrders(tt.orders); got != tt.want {
				t.Fatalf("summarizeOrders() = %q, want %q", got, tt.want)
			}
		})
	}
}
