package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDraftRejectsInvalidUTF8BeforeJudgment(t *testing.T) {
	for _, field := range []string{"text", "quote", "evidence_id"} {
		t.Run(field, func(t *testing.T) {
			raw := strings.Replace(valid, `"`+field+`":"`, `"`+field+`":"`+string([]byte{0xff}), 1)
			if draft, err := Parse(raw); !errors.Is(err, ErrDraft) || len(draft.Claims) != 0 {
				t.Fatalf("invalid bytes accepted: draft=%+v err=%v", draft, err)
			}
			calls := 0
			judge := func(context.Context, Claim, Evidence) (Verdict, error) { calls++; return Supported, nil }
			answer, err := Run(context.Background(), "u", "q", evidence(), scripted(raw), judge)
			if !errors.Is(err, ErrDraft) || len(answer.Claims) != 0 || calls != 0 {
				t.Fatalf("answer=%+v err=%v judge_calls=%d", answer, err, calls)
			}
		})
	}
	raw := strings.Replace(valid, `"text":"`, `"text":"`+"�", 1)
	if _, err := Parse(raw); err != nil {
		t.Fatalf("valid Unicode rejected: %v", err)
	}
}
