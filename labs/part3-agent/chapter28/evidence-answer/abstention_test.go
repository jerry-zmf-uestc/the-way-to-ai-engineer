package main

import (
	"context"
	"errors"
	"testing"
)

const refusal = `{"abstained":true,"reason":"insufficient_evidence","claims":[]}`

func TestAbstentionIsControlledResultNotSupportProof(t *testing.T) {
	judgeCalls := 0
	judge := func(context.Context, Claim, Evidence) (Verdict, error) { judgeCalls++; return Supported, nil }
	a, err := Run(context.Background(), "u", "unrelated question", evidence(), scripted(refusal), judge)
	if err != nil || !a.Abstained || a.Reason != "model_insufficient_evidence" || len(a.Claims) != 0 || judgeCalls != 0 {
		t.Fatal(a, err, judgeCalls)
	}
	// Same refusal on an answerable question is valid protocol, but a task-Eval failure.
	a, err = Run(context.Background(), "u", "What is the timeout?", evidence(), scripted(refusal), judge)
	if err != nil || !a.Abstained {
		t.Fatal(a, err)
	}
	if err = Validate(context.Background(), Draft{Abstained: true, Reason: "insufficient_evidence"}, nil, judge); !errors.Is(err, ErrDraft) {
		t.Fatal("refusal treated as supported claims", err)
	}
}
func TestAbstentionUnionRejectsMixedUnknownAndLate(t *testing.T) {
	for _, raw := range []string{
		`{"claims":[],"reason":"insufficient_evidence"}`,
		`{"abstained":true,"reason":"anything","claims":[]}`,
		`{"abstained":true,"reason":"insufficient_evidence","claims":[{"text":"Timeout is 30 seconds.","evidence_id":"e","version":"1","quote":"Timeout is 30 seconds."}]}`,
		`{"abstained":true,"reason":"insufficient_evidence","message":"arbitrary text"}`,
		`{"abstained":true,"abstained":false,"reason":"insufficient_evidence"}`,
	} {
		if _, err := Parse(raw); !errors.Is(err, ErrDraft) {
			t.Fatal("bad refusal", raw, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := func(context.Context, string, []Evidence) (string, error) { cancel(); return refusal, nil }
	if a, err := Run(ctx, "u", "q", evidence(), model, nil); !errors.Is(err, context.Canceled) || a.Abstained {
		t.Fatal("late refusal delivered", a, err)
	}
	calls := 0
	model = func(context.Context, string, []Evidence) (string, error) { calls++; return refusal, nil }
	if _, err := Run(context.Background(), "foreign", "q", evidence(), model, nil); !errors.Is(err, ErrEvidence) || calls != 0 {
		t.Fatal("refusal bypassed scope", err)
	}
	if _, err := Run(context.Background(), "u", "q", evidence(), scripted("not json"), nil); !errors.Is(err, ErrDraft) {
		t.Fatal("format error converted to refusal", err)
	}
}
