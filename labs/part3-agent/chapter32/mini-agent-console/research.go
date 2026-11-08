package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// The corpus is original teaching material, not downloaded documentation.
// Source IDs and exact excerpts are the contract; relevance is keyword based.
type evidence struct {
	ID    string `json:"id"`
	Quote string `json:"quote"`
}
type researchResult struct {
	Sources []evidence `json:"sources"`
}
type researchClaim struct {
	SourceID string
	Quote    string
}

var ErrNoEvidence = errors.New("research: no matching evidence")
var ErrInvalidCitation = errors.New("research: unsupported citation")

func researchTool(query string) (string, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return "", errors.New("research: empty query")
	}
	corpus := []struct {
		evidence
		keywords []string
	}{
		{evidence{"context", "Cancellation requires the downstream operation to observe context."}, []string{"context", "取消"}},
		{evidence{"tools", "Tool permission is enforced by the application registry."}, []string{"tool", "工具", "权限"}},
		{evidence{"rag", "A source ID alone does not prove that a generated claim follows from the source."}, []string{"rag", "引用", "证据"}},
	}
	result := researchResult{Sources: []evidence{}}
	for _, doc := range corpus {
		for _, word := range doc.keywords {
			if strings.Contains(q, word) {
				result.Sources = append(result.Sources, doc.evidence)
				break
			}
		}
	}
	data, err := json.Marshal(result)
	return string(data), err
}

// validateClaims proves exact excerpt membership only, not semantic entailment.
func validateClaims(sources []evidence, claims []researchClaim) error {
	if len(sources) == 0 || len(claims) == 0 {
		return ErrNoEvidence
	}
	allowed := make(map[string]string, len(sources))
	for _, source := range sources {
		if source.ID == "" || source.Quote == "" {
			return ErrInvalidCitation
		}
		if _, exists := allowed[source.ID]; exists {
			return ErrInvalidCitation
		}
		allowed[source.ID] = source.Quote
	}
	for _, claim := range claims {
		quote, ok := allowed[claim.SourceID]
		if !ok || quote != claim.Quote {
			return ErrInvalidCitation
		}
	}
	return nil
}

func researchReply(result string) (string, error) {
	var parsed researchResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return "", fmt.Errorf("research: invalid result: %w", err)
	}
	// A deterministic extractive fake; replacing this with a Provider still
	// requires validation before sending any answer chunks.
	claims := make([]researchClaim, len(parsed.Sources))
	for i, source := range parsed.Sources {
		claims[i] = researchClaim{source.ID, source.Quote}
	}
	if err := validateClaims(parsed.Sources, claims); err != nil {
		return "", err
	}
	var lines []string
	for _, claim := range claims {
		lines = append(lines, fmt.Sprintf("[%s] %s", claim.SourceID, claim.Quote))
	}
	return strings.Join(lines, "\n"), nil
}
