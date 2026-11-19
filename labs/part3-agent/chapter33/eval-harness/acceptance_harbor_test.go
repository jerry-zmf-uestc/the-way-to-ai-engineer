package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func canonicalHarborDraft() string {
	refs := []map[string]string{}
	for _, e := range harborEvidence() {
		refs = append(refs, map[string]string{"evidence_id": e.ID, "version": e.Version, "quote": e.Text})
	}
	data, _ := json.Marshal(map[string]any{"claims": []any{map[string]any{"text": "Harbor current timeout is 43 seconds.", "citations": refs}}})
	return string(data)
}

func TestHarborAcceptanceChecksAreTaskSpecific(t *testing.T) {
	canonical := canonicalHarborDraft()
	for _, mode := range []string{"canonical", "false-abstention", "wrong-value", "wrong-source", "wrong-version", "duplicate-field", "partial-quote"} {
		text := canonical
		want := ""
		switch mode {
		case "false-abstention":
			text = strings.Replace(text, `{"claims":`, `{"abstained":false,"claims":`, 1)
			want = "protocol"
		case "wrong-value":
			text = strings.Replace(text, "current timeout is 43", "current timeout is 6", 1)
			want = "conclusion"
		case "wrong-source":
			text = strings.Replace(text, `"deployment"`, `"unknown"`, 1)
			want = "citations"
		case "wrong-version":
			text = strings.Replace(text, `"version":"1"`, `"version":"2"`, 1)
			want = "citations"
		case "duplicate-field":
			text = strings.Replace(text, `"text":`, `"text":"ignored","text":`, 1)
			want = "protocol"
		case "partial-quote":
			text = strings.Replace(text, "Harbor fallback_enabled=false.", "fallback_enabled=false", 1)
			want = "citations"
		}
		v, err := ReviewHarborCandidate(context.Background(), AcceptanceCandidate{Attempt: 1, Text: text, Digest: "d", Binding: "b"})
		if err != nil {
			t.Fatal(err)
		}
		failed := map[string]bool{}
		for _, c := range v.Checks {
			failed[c.ID] = c.Status == "fail"
		}
		if want == "" && (failed["protocol"] || failed["citations"] || failed["conclusion"]) || want != "" && !failed[want] {
			t.Fatal(mode, v)
		}
	}
	paraphrase := strings.Replace(canonical, "Harbor current timeout is 43 seconds.", "The configured timeout equals forty-three seconds.", 1)
	v, _ := ReviewHarborCandidate(context.Background(), AcceptanceCandidate{Text: paraphrase})
	if v.Checks[2].Status != "unknown" {
		t.Fatal("fixed wording rule claimed open semantic coverage", v)
	}
}

func TestHarborRequestKeepsFeedbackSeparateFromEvidence(t *testing.T) {
	input := TaskInput{ID: "both-premises", Input: harborQuestion, AllowedSources: []string{"deployment", "policy"}}
	feedback := []CriterionDecision{{ID: "protocol", Status: "fail", Gap: "remove extra fields"}}
	data, err := harborRequest(input, feedback)
	if err != nil || !strings.Contains(string(data), "remove extra fields") {
		t.Fatal(string(data), err)
	}
	var parsed []struct {
		Evidence []AcceptanceEvidence
	}
	if json.Unmarshal(data, &parsed) != nil || len(parsed) != 1 || len(parsed[0].Evidence) != 2 || parsed[0].Evidence[0].Text != harborEvidence()[0].Text {
		t.Fatal("feedback changed authorized premise")
	}
	input.AllowedSources = []string{"policy"}
	if _, err := harborRequest(input, nil); err == nil {
		t.Fatal("missing authorization still supplied evidence")
	}
}
