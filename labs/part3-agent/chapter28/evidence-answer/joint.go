package main

import (
	"context"
	"fmt"
	"strings"
)

type CitationRef struct {
	EvidenceID string `json:"evidence_id"`
	Version    string `json:"version"`
	Quote      string `json:"quote"`
}
type JointClaim struct {
	Text      string        `json:"text"`
	Citations []CitationRef `json:"citations"`
}
type JointJudge func(context.Context, JointClaim, []Evidence) (Verdict, error)

// ValidateJoint consumes an application-authorized snapshot, not model-supplied evidence.
func ValidateJoint(ctx context.Context, claims []JointClaim, registry map[string]Evidence, judge JointJudge) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(claims) == 0 || len(claims) > 16 || len(registry) == 0 || len(registry) > 8 {
		return ErrDraft
	}
	snapshot := make([]JointClaim, len(claims))
	sets := make([][]Evidence, len(claims))
	bytes := 0
	for n, claim := range claims {
		if strings.TrimSpace(claim.Text) == "" || len(claim.Citations) == 0 || len(claim.Citations) > 8 {
			return ErrDraft
		}
		snapshot[n] = JointClaim{Text: claim.Text, Citations: append([]CitationRef(nil), claim.Citations...)}
		bytes += len(claim.Text)
		seen := map[string]bool{}
		for _, ref := range claim.Citations {
			bytes += len(ref.EvidenceID) + len(ref.Version) + len(ref.Quote)
			if bytes > 32<<10 || ref.EvidenceID == "" || ref.Version == "" || strings.TrimSpace(ref.Quote) == "" || seen[ref.EvidenceID] {
				return ErrDraft
			}
			seen[ref.EvidenceID] = true
			item, ok := registry[ref.EvidenceID]
			if !ok || item.ID != ref.EvidenceID || ref.Version != item.Version || !strings.Contains(item.Text, ref.Quote) {
				return ErrEvidence
			}
			sets[n] = append(sets[n], item)
		}
	}
	if judge == nil {
		return ErrReview
	}
	for n, claim := range snapshot {
		if err := ctx.Err(); err != nil {
			return err
		}
		claim.Citations = append([]CitationRef(nil), claim.Citations...)
		verdict, err := judge(ctx, claim, append([]Evidence(nil), sets[n]...))
		if err != nil {
			return fmt.Errorf("rag: joint judge: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		switch verdict {
		case Supported:
		case Unsupported:
			return ErrUnsupported
		default:
			return ErrReview
		}
	}
	return ctx.Err()
}
