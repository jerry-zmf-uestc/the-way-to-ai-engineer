package main

import (
	"context"
	"errors"
	"testing"
)

func TestExactSourceJudgeDoesNotBecomeSemanticJudge(t *testing.T) {
	raw := `{"claims":[{"text":"Timeout is 30 seconds.","evidence_id":"e","version":"1","quote":"30 seconds."}]}`
	answer, err := Run(context.Background(), "u", "q", evidence(), scripted(raw), ExactSourceJudge)
	if err != nil || len(answer.Claims) != 1 {
		t.Fatal(answer, err)
	}
	for _, claim := range []string{"Timeout is 60 seconds.", "Requests time out after 30 seconds."} {
		verdict, err := ExactSourceJudge(context.Background(), Claim{Text: claim}, evidence()[0])
		if err != nil || verdict != Unknown {
			t.Fatal("rule judged semantics", claim, verdict, err)
		}
	}
	raw = `{"claims":[{"text":"Timeout is 30 seconds.","evidence_id":"missing","version":"1","quote":"30 seconds."}]}`
	if _, err := Run(context.Background(), "u", "q", evidence(), scripted(raw), ExactSourceJudge); !errors.Is(err, ErrEvidence) {
		t.Fatal("judge bypassed citation", err)
	}
}
