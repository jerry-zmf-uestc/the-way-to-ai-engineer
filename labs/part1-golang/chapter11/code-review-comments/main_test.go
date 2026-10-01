package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestMissingRequiredChecks(t *testing.T) {
	tests := []struct {
		name   string
		checks []string
		want   []string
	}{
		{name: "all present", checks: []string{"gofmt", "tests", "error-handling"}},
		{name: "trims and lowercases", checks: []string{" GoFmt ", "TESTS", "error-handling"}},
		{name: "reports missing checks", checks: []string{"gofmt"}, want: []string{"tests", "error-handling"}},
		{name: "ignores empty checks", checks: []string{" ", "tests"}, want: []string{"gofmt", "error-handling"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingRequiredChecks(tt.checks)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("missingRequiredChecks(%q) = %q, want %q", tt.checks, got, tt.want)
			}
		})
	}
}

func TestSummarizeReview(t *testing.T) {
	tests := []struct {
		name    string
		input   reviewInput
		want    string
		wantErr error
	}{
		{
			name: "valid review",
			input: reviewInput{
				Author:       "Lin",
				ChangeID:     "CL-2048",
				LinesChanged: 42,
				Checks:       []string{"gofmt", "tests", "error-handling"},
			},
			want: "Lin reviewed CL-2048: 42 lines checked",
		},
		{
			name:    "missing author",
			input:   reviewInput{ChangeID: "CL-2048", Checks: []string{"gofmt"}},
			wantErr: errMissingAuthor,
		},
		{
			name:    "missing change ID",
			input:   reviewInput{Author: "Lin", Checks: []string{"gofmt"}},
			wantErr: errMissingChangeID,
		},
		{
			name:    "missing checks",
			input:   reviewInput{Author: "Lin", ChangeID: "CL-2048"},
			wantErr: errMissingChecks,
		},
		{
			name:    "missing required check",
			input:   reviewInput{Author: "Lin", ChangeID: "CL-2048", Checks: []string{"gofmt", "tests"}},
			wantErr: errors.New("missing required checks"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := summarizeReview(context.Background(), tt.input)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("summarizeReview(%+v) returned unexpected error: %v", tt.input, err)
				}
				if got != tt.want {
					t.Fatalf("summarizeReview(%+v) = %q, want %q", tt.input, got, tt.want)
				}
				return
			}

			if err == nil {
				t.Fatalf("summarizeReview(%+v) returned nil error, want %v", tt.input, tt.wantErr)
			}
			if errors.Is(tt.wantErr, errMissingAuthor) ||
				errors.Is(tt.wantErr, errMissingChangeID) ||
				errors.Is(tt.wantErr, errMissingChecks) {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("summarizeReview(%+v) error = %v, want %v", tt.input, err, tt.wantErr)
				}
				return
			}
			if got := err.Error(); got != "missing required checks: error-handling" {
				t.Fatalf("summarizeReview(%+v) error = %q, want missing required checks", tt.input, got)
			}
		})
	}
}

func TestSummarizeReviewCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := summarizeReview(ctx, reviewInput{
		Author:   "Lin",
		ChangeID: "CL-2048",
		Checks:   []string{"gofmt", "tests", "error-handling"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("summarizeReview(canceled context) error = %v, want context.Canceled", err)
	}
}
