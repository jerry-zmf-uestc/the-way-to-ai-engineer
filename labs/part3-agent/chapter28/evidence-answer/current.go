package main

import (
	"context"
	"fmt"
)

// CurrentEvidenceReader is supplied by the trusted application and enforces its read policy.
type CurrentEvidenceReader func(context.Context, string, string) (Evidence, error)

func checkCurrentEvidence(ctx context.Context, tenant string, items []Evidence, read CurrentEvidenceReader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if read == nil || tenant == "" || len(items) > 8 {
		return ErrEvidence
	}
	seen := map[string]bool{}
	for _, item := range items {
		if item.ID == "" || item.Tenant != tenant || seen[item.ID] {
			return ErrEvidence
		}
		seen[item.ID] = true
	}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := read(ctx, tenant, item.ID)
		if err != nil {
			return fmt.Errorf("rag: current evidence: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current != item {
			return ErrEvidence
		}
	}
	return ctx.Err()
}

func RunCurrentJoint(ctx context.Context, tenant, question string, items []Evidence, read CurrentEvidenceReader, model Model, judge JointJudge) (JointAnswer, error) {
	if err := ctx.Err(); err != nil {
		return JointAnswer{}, err
	}
	if len(items) > 8 {
		return JointAnswer{}, ErrEvidence
	}
	snapshot := append([]Evidence(nil), items...)
	if err := checkCurrentEvidence(ctx, tenant, snapshot, read); err != nil {
		return JointAnswer{}, err
	}
	answer, err := RunJoint(ctx, tenant, question, snapshot, model, judge)
	if err != nil {
		return JointAnswer{}, err
	}
	if err := checkCurrentEvidence(ctx, tenant, snapshot, read); err != nil {
		return JointAnswer{}, err
	}
	return answer, nil
}
