package main

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCitationInputBindsOriginalQuestionAndEvidence(t *testing.T) {
	request := DraftRequest{Question: "current timeout?", Evidence: []Evidence{{ID: "policy", Version: "1", Text: "conditional rule"}}}
	content := `{"question":"current timeout?","evidence":[{"evidence_id":"policy","version":"1","text":"conditional rule"}]}`
	row := GeneratedDraft{InputMessages: []GeneratorMessage{{Role: "system", Content: "fixed instructions"}, {Role: "user", Content: content}}}
	if err := validateCitationInput(row, request); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"question":"different","evidence":[{"evidence_id":"policy","version":"1","text":"conditional rule"}]}`,
		`{"question":"current timeout?","evidence":[{"id":"policy","version":"1","text":"conditional rule"}]}`,
		`{"question":"current timeout?","evidence":[{"evidence_id":"policy","version":"2","text":"conditional rule"}]}`,
		`{"question":"current timeout?","evidence":[{"evidence_id":"policy","version":"1","text":"conditional rule"}],"expected":"hidden"}`,
		`{"question":"current timeout?","question":"different","evidence":[]}`,
	} {
		bad := row
		bad.InputMessages = append([]GeneratorMessage(nil), row.InputMessages...)
		bad.InputMessages[1].Content = body
		if !errors.Is(validateCitationInput(bad, request), ErrDraft) {
			t.Fatal("changed projection accepted", body)
		}
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var restored GeneratedDraft
	if json.Unmarshal(data, &restored) != nil || validateCitationInput(restored, request) != nil {
		t.Fatal("input record roundtrip")
	}
}
