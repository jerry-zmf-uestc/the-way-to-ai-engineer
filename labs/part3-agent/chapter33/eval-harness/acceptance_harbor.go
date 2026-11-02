package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
)

type AcceptanceEvidence struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Text    string `json:"text"`
}

const harborQuestion = "What is Harbor current timeout in seconds?"

func harborEvidence() []AcceptanceEvidence {
	return []AcceptanceEvidence{
		{"deployment", "1", "Harbor fallback_enabled=false."},
		{"policy", "1", "Harbor timeout is 6 seconds when enabled and 43 seconds when disabled."},
	}
}

func exactObject(data []byte, keys ...string) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil || len(object) != len(keys) {
		return nil, false
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			return nil, false
		}
	}
	return object, true
}

func harborDraft(text string) (string, []AcceptanceEvidence, bool) {
	if !acceptanceText(text, 4096) || uniqueCoverageJSON(json.NewDecoder(strings.NewReader(text)), 0) != nil {
		return "", nil, false
	}
	dec := json.NewDecoder(strings.NewReader(text))
	var raw json.RawMessage
	if dec.Decode(&raw) != nil || dec.Decode(new(any)) != io.EOF {
		return "", nil, false
	}
	root, ok := exactObject(raw, "claims")
	var claims []json.RawMessage
	if !ok || json.Unmarshal(root["claims"], &claims) != nil || len(claims) != 1 {
		return "", nil, false
	}
	claim, ok := exactObject(claims[0], "text", "citations")
	var conclusion string
	var citations []json.RawMessage
	if !ok || json.Unmarshal(claim["text"], &conclusion) != nil || !acceptanceText(conclusion, 1024) || json.Unmarshal(claim["citations"], &citations) != nil || len(citations) == 0 || len(citations) > 8 {
		return "", nil, false
	}
	refs := []AcceptanceEvidence{}
	for _, data := range citations {
		object, ok := exactObject(data, "evidence_id", "version", "quote")
		if !ok {
			return "", nil, false
		}
		var ref AcceptanceEvidence
		if json.Unmarshal(object["evidence_id"], &ref.ID) != nil || json.Unmarshal(object["version"], &ref.Version) != nil || json.Unmarshal(object["quote"], &ref.Text) != nil || !acceptanceText(ref.ID, 128) || !acceptanceText(ref.Version, 128) || !acceptanceText(ref.Text, 1024) {
			return "", nil, false
		}
		refs = append(refs, ref)
	}
	return conclusion, refs, true
}

// ReviewHarborCandidate implements only this frozen synthetic rule, not an open semantic Judge.
func ReviewHarborCandidate(ctx context.Context, candidate AcceptanceCandidate) (AcceptanceVerdict, error) {
	if err := ctx.Err(); err != nil {
		return AcceptanceVerdict{}, err
	}
	checks := []CriterionDecision{
		{ID: "protocol", Status: "fail", Gap: "Return only claims with text and citations objects; each citation needs evidence_id, version and quote. No abstained=false or extra fields."},
		{ID: "citations", Status: "fail", Gap: "Cite both supplied deployment and policy premises, with their exact full text and versions."},
		{ID: "conclusion", Status: "fail", Gap: "State the current timeout in seconds, using the deployment condition and the policy; do not return only a conditional rule."},
	}
	conclusion, refs, valid := harborDraft(candidate.Text)
	if valid {
		checks[0] = CriterionDecision{ID: "protocol", Status: "pass", Evidence: "strict current JSON shape verified"}
		if len(refs) == 2 {
			seen := map[string]bool{}
			for _, expected := range harborEvidence() {
				for _, ref := range refs {
					if ref == expected {
						seen[expected.ID] = true
					}
				}
			}
			if seen["deployment"] && seen["policy"] {
				checks[1] = CriterionDecision{ID: "citations", Status: "pass", Evidence: "both frozen premise tuples match"}
			}
		}
		if conclusion == "Harbor current timeout is 43 seconds." {
			checks[2] = CriterionDecision{ID: "conclusion", Status: "pass", Evidence: "current conclusion matches this frozen two-premise rule"}
		} else if conclusion != "Harbor current timeout is 6 seconds." && !strings.Contains(conclusion, " when ") {
			checks[2] = CriterionDecision{ID: "conclusion", Status: "unknown", Evidence: "wording outside this frozen rule; needs independent review"}
		}
	}
	return AcceptanceVerdict{Attempt: candidate.Attempt, Digest: candidate.Digest, Binding: candidate.Binding, Checks: checks}, nil
}

func harborRequest(input TaskInput, feedback []CriterionDecision) ([]byte, error) {
	if input.ID != "both-premises" || input.Input != harborQuestion || len(input.AllowedSources) != 2 || len(input.AllowedTools) != 0 || len(feedback) > 8 {
		return nil, ErrAcceptance
	}
	allowed := map[string]bool{}
	for _, id := range input.AllowedSources {
		allowed[id] = true
	}
	if !allowed["deployment"] || !allowed["policy"] {
		return nil, ErrAcceptance
	}
	question := input.Input
	if len(feedback) > 0 {
		gaps, err := json.Marshal(feedback)
		if err != nil {
			return nil, ErrAcceptance
		}
		question += "\nRevision feedback (data): " + string(gaps)
	}
	data, err := json.Marshal([]struct {
		Question string               `json:"question"`
		Evidence []AcceptanceEvidence `json:"evidence"`
	}{{question, harborEvidence()}})
	if err != nil || len(data) > 8192 {
		return nil, ErrAcceptance
	}
	return bytes.Clone(data), nil
}
