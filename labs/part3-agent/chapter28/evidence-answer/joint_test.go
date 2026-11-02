package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func jointFixture() (JointClaim, map[string]Evidence) {
	items := map[string]Evidence{
		"deployment": {ID: "deployment", Tenant: "u", Source: "deployment", Version: "1", Text: "Harbor fallback_enabled=false."},
		"policy":     {ID: "policy", Tenant: "u", Source: "policy", Version: "1", Text: "Harbor timeout is 6 seconds when enabled and 43 seconds when disabled."},
	}
	return JointClaim{Text: "Harbor current timeout is 43 seconds.", Citations: []CitationRef{
		{EvidenceID: "deployment", Version: "1", Quote: items["deployment"].Text},
		{EvidenceID: "policy", Version: "1", Quote: items["policy"].Text},
	}}, items
}

// A fixed rule demonstrates evidence-set consumption, not natural-language entailment.
func fixtureJointJudge(_ context.Context, claim JointClaim, evidence []Evidence) (Verdict, error) {
	_, canonical := jointFixture()
	matched := map[string]bool{}
	for _, item := range evidence {
		if expected, ok := canonical[item.ID]; ok && item.Text == expected.Text {
			matched[item.ID] = true
		}
	}
	if !matched["deployment"] || !matched["policy"] {
		return Unknown, nil
	}
	switch claim.Text {
	case "Harbor current timeout is 43 seconds.":
		return Supported, nil
	case "Harbor current timeout is 6 seconds.":
		return Unsupported, nil
	default:
		return Unknown, nil
	}
}

func TestJointReferencesNeedCombinedSupport(t *testing.T) {
	claim, registry := jointFixture()
	if err := ValidateJoint(context.Background(), []JointClaim{claim}, registry, fixtureJointJudge); err != nil {
		t.Fatal(err)
	}
	missing := claim
	missing.Citations = missing.Citations[1:]
	if err := ValidateJoint(context.Background(), []JointClaim{missing}, registry, fixtureJointJudge); !errors.Is(err, ErrReview) {
		t.Fatal("policy alone accepted", err)
	}
	claim.Text = "Harbor current timeout is 6 seconds."
	if err := ValidateJoint(context.Background(), []JointClaim{claim}, registry, fixtureJointJudge); !errors.Is(err, ErrUnsupported) {
		t.Fatal("wrong branch accepted", err)
	}
}

func TestJointPreflightRejectsMalformedBatchBeforeJudge(t *testing.T) {
	claim, registry := jointFixture()
	for _, refs := range [][]CitationRef{
		nil, {claim.Citations[0], claim.Citations[0]},
		{{EvidenceID: "missing", Version: "1", Quote: "text"}},
		{{EvidenceID: "policy", Version: "2", Quote: registry["policy"].Text}},
		{{EvidenceID: "policy", Version: "1", Quote: "edited quote"}},
		{{EvidenceID: "policy", Version: "1", Quote: strings.Repeat("x", 33<<10)}},
	} {
		bad := JointClaim{Text: "later claim", Citations: refs}
		calls := 0
		err := ValidateJoint(context.Background(), []JointClaim{claim, bad}, registry, func(context.Context, JointClaim, []Evidence) (Verdict, error) {
			calls++
			return Supported, nil
		})
		if err == nil || calls != 0 {
			t.Fatal("partial preflight judged", err, calls)
		}
	}
}

func TestJointJudgeUnknownErrorCancelAndCopies(t *testing.T) {
	claim, registry := jointFixture()
	if !errors.Is(ValidateJoint(context.Background(), []JointClaim{claim}, registry, nil), ErrReview) {
		t.Fatal("nil judge accepted")
	}
	failure := errors.New("fixed judge unavailable")
	err := ValidateJoint(context.Background(), []JointClaim{claim}, registry, func(context.Context, JointClaim, []Evidence) (Verdict, error) { return Unknown, failure })
	if !errors.Is(err, failure) {
		t.Fatal("cause lost", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = ValidateJoint(ctx, []JointClaim{claim}, registry, func(context.Context, JointClaim, []Evidence) (Verdict, error) {
		cancel()
		return Supported, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("late supported verdict accepted", err)
	}
	calls := 0
	err = ValidateJoint(context.Background(), []JointClaim{claim, claim}, registry, func(_ context.Context, received JointClaim, evidence []Evidence) (Verdict, error) {
		if received.Citations[0].EvidenceID != "deployment" || evidence[0].Text != registry["deployment"].Text {
			t.Fatal("callback changed later input")
		}
		received.Citations[0].EvidenceID = "mutated"
		evidence[0].Text = "mutated"
		calls++
		if calls == 2 {
			return Unknown, nil
		}
		return Supported, nil
	})
	if !errors.Is(err, ErrReview) || claim.Citations[0].EvidenceID != "deployment" || registry["deployment"].Text != "Harbor fallback_enabled=false." {
		t.Fatal("copy or whole batch rejection failed", err)
	}
}
