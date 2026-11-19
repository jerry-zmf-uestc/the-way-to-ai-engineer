package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDownstreamContextFailureIsNotGenericReaderFailure(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			s, p, _ := fixture(t, func(context.Context, Action) (string, error) {
				return "late private payload", fmt.Errorf("synthetic-private-error: %w", cause)
			})
			r := approve(t, s, p, request(t, s, p))
			final, err := s.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint)
			wantState, wantCode := "canceled", "canceled"
			if errors.Is(cause, context.DeadlineExceeded) {
				wantState, wantCode = "failed", "deadline_exceeded"
			}
			if !errors.Is(err, cause) || strings.Contains(err.Error(), "synthetic-private-error") || final.Output != "" || final.State != wantState || final.Failure != wantCode {
				t.Fatal(final, err)
			}
			saved, err := s.load()
			if err != nil || saved.State != wantState || saved.Failure != wantCode || saved.Attempts != 1 || saved.Output != "" {
				t.Fatal("terminal not persisted", saved, err)
			}
		})
	}
}
