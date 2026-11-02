package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestJointRunPublishesOnlyWholeSupportedDraft(t *testing.T) {
	claim, registry := jointFixture()
	items := []Evidence{registry["deployment"], registry["policy"]}
	raw, _ := json.Marshal(JointDraft{Claims: []JointClaim{claim}})
	calls := 0
	model := func(_ context.Context, _ string, given []Evidence) (string, error) {
		calls++
		given[0].Text = "changed only in model view"
		return string(raw), nil
	}
	answer, err := RunJoint(context.Background(), "u", "current timeout?", items, model, fixtureJointJudge)
	if err != nil || len(answer.Claims) != 1 || len(answer.Claims[0].Citations) != 2 || calls != 1 || items[0].Text != registry["deployment"].Text {
		t.Fatal(answer, err, calls)
	}
	bad := claim
	bad.Text = "Harbor current timeout is 6 seconds."
	raw, _ = json.Marshal(JointDraft{Claims: []JointClaim{claim, bad}})
	answer, err = RunJoint(context.Background(), "u", "current timeout?", items, model, fixtureJointJudge)
	if !errors.Is(err, ErrUnsupported) || len(answer.Claims) != 0 || answer.Abstained {
		t.Fatal("earlier claim published", answer, err)
	}
}

func TestJointParserRejectsAmbiguousMixedAndUnboundedDrafts(t *testing.T) {
	for _, raw := range []string{
		`{"claims":[],"Claims":[]}`, `{"abstained":false,"Abstained":true,"reason":"insufficient_evidence"}`,
		`{"abstained":true,"reason":"insufficient_evidence","claims":[{"text":"x","citations":[]}]}`,
		`{"abstained":true,"reason":"new_reason"}`, `{"abstained":true,"reason":"insufficient_evidence","extra":1}`,
		`{"claims":[{"text":"x","citations":[{"evidence_id":"e","Evidence_ID":"e","version":"1","quote":"x"}]}]}`,
		`{"claims":[{"text":"x","citations":[{"evidence_id":"e","version":"1","quote":"x"},{"evidence_id":"e","version":"1","quote":"x"}]}]}`,
		`{"claims":[]}{}`, strings.Repeat("x", (32<<10)+1), string([]byte{0xff}),
	} {
		got, err := ParseJoint(raw)
		if !errors.Is(err, ErrDraft) || len(got.Claims) != 0 || got.Abstained {
			t.Fatal("invalid draft produced output", err)
		}
	}
	if _, err := Parse(`{"abstained":false,"Abstained":true,"reason":"insufficient_evidence"}`); !errors.Is(err, ErrDraft) {
		t.Fatal("old parser allowed alias override", err)
	}
	draft, err := ParseJoint(`{"abstained":true,"reason":"insufficient_evidence"}`)
	if err != nil || !draft.Abstained {
		t.Fatal(draft, err)
	}
}

func TestJointRunInputRefusalAndCancellationBoundaries(t *testing.T) {
	_, registry := jointFixture()
	items := []Evidence{registry["deployment"], registry["policy"]}
	calls := 0
	model := func(context.Context, string, []Evidence) (string, error) {
		calls++
		return `{"abstained":true,"reason":"insufficient_evidence"}`, nil
	}
	answer, err := RunJoint(context.Background(), "u", "q", nil, model, nil)
	if err != nil || answer.Reason != "no_evidence" || calls != 0 {
		t.Fatal(answer, err, calls)
	}
	foreign := append([]Evidence(nil), items...)
	foreign[1].Tenant = "other"
	if _, err := RunJoint(context.Background(), "u", "q", foreign, model, nil); !errors.Is(err, ErrEvidence) || calls != 0 {
		t.Fatal(err, calls)
	}
	conflict := append([]Evidence(nil), items...)
	conflict[0].FactKey, conflict[0].FactValue = "fixture", "a"
	conflict[1].FactKey, conflict[1].FactValue = "fixture", "b"
	answer, err = RunJoint(context.Background(), "u", "q", conflict, model, nil)
	if err != nil || answer.Reason != "conflicting_evidence" || calls != 0 {
		t.Fatal(answer, err, calls)
	}
	answer, err = RunJoint(context.Background(), "u", "q", items, model, func(context.Context, JointClaim, []Evidence) (Verdict, error) {
		t.Fatal("refusal judged")
		return Unknown, nil
	})
	if err != nil || answer.Reason != "model_insufficient_evidence" || calls != 1 {
		t.Fatal(answer, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	answer, err = RunJoint(ctx, "u", "q", items, func(context.Context, string, []Evidence) (string, error) {
		cancel()
		return `{"abstained":true,"reason":"insufficient_evidence"}`, nil
	}, nil)
	if !errors.Is(err, context.Canceled) || answer.Abstained || len(answer.Claims) != 0 {
		t.Fatal("late refusal released", answer, err)
	}
}

func TestJointRunUnknownAndModelCauseDoNotRelease(t *testing.T) {
	claim, registry := jointFixture()
	items := []Evidence{registry["deployment"], registry["policy"]}
	raw, _ := json.Marshal(JointDraft{Claims: []JointClaim{claim}})
	answer, err := RunJoint(context.Background(), "u", "q", items, scripted(string(raw)), nil)
	if !errors.Is(err, ErrReview) || len(answer.Claims) != 0 {
		t.Fatal(answer, err)
	}
	cause := errors.New("fixed unavailable model")
	answer, err = RunJoint(context.Background(), "u", "q", items, func(context.Context, string, []Evidence) (string, error) { return "", cause }, nil)
	if !errors.Is(err, cause) || len(answer.Claims) != 0 {
		t.Fatal(answer, err)
	}
}
