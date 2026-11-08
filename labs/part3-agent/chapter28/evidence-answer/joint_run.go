package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type JointDraft struct {
	Claims    []JointClaim `json:"claims"`
	Abstained bool         `json:"abstained,omitempty"`
	Reason    string       `json:"reason,omitempty"`
}
type JointAnswer struct {
	Claims    []JointClaim
	Abstained bool
	Reason    string
}

func ParseJoint(raw string) (JointDraft, error) {
	if len(raw) > 32<<10 || !utf8.ValidString(raw) || uniqueFields(json.NewDecoder(strings.NewReader(raw)), 0) != nil {
		return JointDraft{}, ErrDraft
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var draft JointDraft
	if err := decoder.Decode(&draft); err != nil {
		return JointDraft{}, fmt.Errorf("rag: joint draft: %w: %w", ErrDraft, err)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return JointDraft{}, ErrDraft
	}
	if draft.Abstained {
		if len(draft.Claims) != 0 || draft.Reason != "insufficient_evidence" {
			return JointDraft{}, ErrDraft
		}
		return draft, nil
	}
	if draft.Reason != "" || len(draft.Claims) == 0 || len(draft.Claims) > 16 {
		return JointDraft{}, ErrDraft
	}
	for _, claim := range draft.Claims {
		if strings.TrimSpace(claim.Text) == "" || len(claim.Citations) == 0 || len(claim.Citations) > 8 {
			return JointDraft{}, ErrDraft
		}
		seen := map[string]bool{}
		for _, ref := range claim.Citations {
			if ref.EvidenceID == "" || ref.Version == "" || strings.TrimSpace(ref.Quote) == "" || seen[ref.EvidenceID] {
				return JointDraft{}, ErrDraft
			}
			seen[ref.EvidenceID] = true
		}
	}
	return draft, nil
}

func RunJoint(ctx context.Context, tenant, question string, items []Evidence, model Model, judge JointJudge) (JointAnswer, error) {
	if err := ctx.Err(); err != nil {
		return JointAnswer{}, err
	}
	if tenant == "" || strings.TrimSpace(question) == "" || len(question) > 4096 || len(items) > 8 {
		return JointAnswer{}, ErrEvidence
	}
	if len(items) == 0 {
		return JointAnswer{Abstained: true, Reason: "no_evidence"}, nil
	}
	registry, reason, err := prepareEvidence(tenant, items)
	if err != nil {
		return JointAnswer{}, err
	}
	if reason != "" {
		return JointAnswer{Abstained: true, Reason: reason}, nil
	}
	if model == nil {
		return JointAnswer{}, ErrDraft
	}
	raw, err := model(ctx, question, append([]Evidence(nil), items...))
	if err != nil {
		return JointAnswer{}, fmt.Errorf("rag: joint model: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return JointAnswer{}, err
	}
	draft, err := ParseJoint(raw)
	if err != nil {
		return JointAnswer{}, err
	}
	if draft.Abstained {
		if err := ctx.Err(); err != nil {
			return JointAnswer{}, err
		}
		return JointAnswer{Abstained: true, Reason: "model_insufficient_evidence"}, nil
	}
	if err := ValidateJoint(ctx, draft.Claims, registry, judge); err != nil {
		return JointAnswer{}, err
	}
	claims := make([]JointClaim, len(draft.Claims))
	for n, claim := range draft.Claims {
		claims[n] = JointClaim{Text: claim.Text, Citations: append([]CitationRef(nil), claim.Citations...)}
	}
	if err := ctx.Err(); err != nil {
		return JointAnswer{}, err
	}
	return JointAnswer{Claims: claims}, nil
}
