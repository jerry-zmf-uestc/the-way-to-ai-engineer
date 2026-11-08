package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrAcceptance       = errors.New("eval: invalid acceptance contract")
	ErrAcceptanceReview = errors.New("eval: acceptance needs review")
	ErrAcceptanceBudget = errors.New("eval: acceptance candidate budget exhausted")
)

type AcceptancePolicy struct {
	Version       string
	Criteria      []string
	MaxCandidates int
}
type CriterionDecision struct{ ID, Status, Evidence, Gap string }
type AcceptanceCandidate struct {
	Attempt               int
	Text, Digest, Binding string
}
type AcceptanceVerdict struct {
	Attempt         int
	Digest, Binding string
	Checks          []CriterionDecision
}
type AcceptanceAttempt struct {
	Candidate AcceptanceCandidate
	Checks    []CriterionDecision
}
type AcceptanceReport struct {
	PolicyVersion, Binding, Status, Output string
	GenerationCalls, ReviewCalls           int
	Attempts                               []AcceptanceAttempt
}
type CandidateGenerator func(context.Context, TaskInput, []CriterionDecision) (string, error)
type CandidateReviewer func(context.Context, AcceptanceCandidate) (AcceptanceVerdict, error)

func acceptanceText(text string, limit int) bool {
	return strings.TrimSpace(text) != "" && len(text) <= limit && utf8.ValidString(text)
}

func checkAcceptanceVerdict(candidate AcceptanceCandidate, required map[string]bool, verdict AcceptanceVerdict) ([]CriterionDecision, bool, error) {
	if verdict.Binding != candidate.Binding || verdict.Attempt != candidate.Attempt || verdict.Digest != candidate.Digest || len(verdict.Checks) != len(required) {
		return nil, false, ErrAcceptance
	}
	seen := map[string]bool{}
	feedback := []CriterionDecision{}
	unknown := false
	for _, check := range verdict.Checks {
		if !required[check.ID] || seen[check.ID] || len(check.Evidence) > 1024 || len(check.Gap) > 1024 || !utf8.ValidString(check.Evidence) || !utf8.ValidString(check.Gap) {
			return nil, false, ErrAcceptance
		}
		seen[check.ID] = true
		switch check.Status {
		case "pass":
			if !acceptanceText(check.Evidence, 1024) || check.Gap != "" {
				return nil, false, ErrAcceptance
			}
		case "fail":
			if !acceptanceText(check.Gap, 1024) {
				return nil, false, ErrAcceptance
			}
			feedback = append(feedback, check)
		case "unknown":
			unknown = true
		default:
			return nil, false, ErrAcceptance
		}
	}
	return feedback, unknown, nil
}

func RunAcceptance(ctx context.Context, input TaskInput, policy AcceptancePolicy, generate CandidateGenerator, review CandidateReviewer) (AcceptanceReport, error) {
	report := AcceptanceReport{PolicyVersion: policy.Version, Status: "failed"}
	if !acceptanceText(input.ID, 128) || !acceptanceText(input.Input, 4096) || !acceptanceText(policy.Version, 128) || len(policy.Criteria) == 0 || len(policy.Criteria) > 8 || policy.MaxCandidates < 1 || policy.MaxCandidates > 4 || generate == nil || review == nil {
		return report, ErrAcceptance
	}
	required := map[string]bool{}
	for _, id := range policy.Criteria {
		if !acceptanceText(id, 128) || required[id] {
			return report, ErrAcceptance
		}
		required[id] = true
	}
	for _, allowed := range [][]string{input.AllowedSources, input.AllowedTools} {
		if len(allowed) > 8 {
			return report, ErrAcceptance
		}
		for _, name := range allowed {
			if !acceptanceText(name, 128) {
				return report, ErrAcceptance
			}
		}
	}
	sources, tools := append([]string(nil), input.AllowedSources...), append([]string(nil), input.AllowedTools...)
	input.AllowedSources, input.AllowedTools = sources, tools
	policy.Criteria = append([]string(nil), policy.Criteria...)
	bound, err := json.Marshal(struct {
		Input  TaskInput
		Policy AcceptancePolicy
	}{input, policy})
	if err != nil {
		return report, ErrAcceptance
	}
	report.Binding = fmt.Sprintf("%x", sha256.Sum256(bound))
	var feedback []CriterionDecision
	for n := 1; n <= policy.MaxCandidates; n++ {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		view := input
		view.AllowedSources, view.AllowedTools = append([]string(nil), sources...), append([]string(nil), tools...)
		report.GenerationCalls++
		text, err := generate(ctx, view, append([]CriterionDecision(nil), feedback...))
		if stop := ctx.Err(); stop != nil {
			return report, stop
		}
		if err != nil {
			return report, fmt.Errorf("generate candidate: %w", err)
		}
		if !acceptanceText(text, 4096) {
			return report, ErrAcceptance
		}
		candidate := AcceptanceCandidate{Attempt: n, Text: text, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(text))), Binding: report.Binding}
		report.Attempts = append(report.Attempts, AcceptanceAttempt{Candidate: candidate})
		report.ReviewCalls++
		verdict, err := review(ctx, candidate)
		if stop := ctx.Err(); stop != nil {
			return report, stop
		}
		if err != nil {
			return report, fmt.Errorf("review candidate: %w", err)
		}
		var unknown bool
		feedback, unknown, err = checkAcceptanceVerdict(candidate, required, verdict)
		if err != nil {
			return report, err
		}
		report.Attempts[len(report.Attempts)-1].Checks = append([]CriterionDecision(nil), verdict.Checks...)
		if unknown {
			report.Status = "review"
			return report, ErrAcceptanceReview
		}
		if len(feedback) == 0 {
			report.Status, report.Output = "passed", text
			return report, nil
		}
	}
	return report, ErrAcceptanceBudget
}
