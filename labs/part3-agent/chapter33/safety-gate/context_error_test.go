package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestParentDeadlineHasDistinctSafeAuditCategory(t *testing.T) {
	called := 0
	g, _, _ := fixture(t, func(context.Context, Document) (Result, error) {
		called++
		return Result{ID: "guide", Text: "late"}, nil
	})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if result, err := g.Execute(ctx, Proposal{"read_document", `{"document_id":"guide"}`}); !errors.Is(err, context.DeadlineExceeded) || result.Text != "" || called != 0 {
		t.Fatal(result, err, called)
	}
	audit := g.Audit()
	if len(audit) != 1 || audit[0].Reason != "deadline_exceeded" || audit[0].Bytes != 0 {
		t.Fatal(audit)
	}
}
