package main

import (
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

type CitationPromptEvidence struct {
	EvidenceID string `json:"evidence_id"`
	Version    string `json:"version"`
	Text       string `json:"text"`
}

func validateCitationInput(row GeneratedDraft, request DraftRequest) error {
	if len(row.InputMessages) != 2 || row.InputMessages[0].Role != "system" || row.InputMessages[1].Role != "user" {
		return ErrDraft
	}
	for _, m := range row.InputMessages {
		if m.Content == "" || len(m.Content) > 16<<10 || !utf8.ValidString(m.Content) {
			return ErrDraft
		}
	}
	var actual struct {
		Question string                   `json:"question"`
		Evidence []CitationPromptEvidence `json:"evidence"`
	}
	if uniqueFields(json.NewDecoder(strings.NewReader(row.InputMessages[1].Content)), 0) != nil {
		return ErrDraft
	}
	dec := json.NewDecoder(strings.NewReader(row.InputMessages[1].Content))
	dec.DisallowUnknownFields()
	if dec.Decode(&actual) != nil || actual.Question != request.Question || len(actual.Evidence) != len(request.Evidence) {
		return ErrDraft
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return ErrDraft
	}
	for n, expected := range request.Evidence {
		if actual.Evidence[n] != (CitationPromptEvidence{expected.ID, expected.Version, expected.Text}) {
			return ErrDraft
		}
	}
	return nil
}
