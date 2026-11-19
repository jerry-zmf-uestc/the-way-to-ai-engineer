package main

import "context"

// ExactSourceJudge proves only full original-text identity, not semantic entailment.
// Validate must already have checked citation ID/version/quote ownership.
func ExactSourceJudge(ctx context.Context, claim Claim, evidence Evidence) (Verdict, error) {
	if err := ctx.Err(); err != nil {
		return Unknown, err
	}
	if claim.Text == evidence.Text {
		return Supported, nil
	}
	return Unknown, nil
}
