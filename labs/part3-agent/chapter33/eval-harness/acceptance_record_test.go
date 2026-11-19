package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func replayAcceptanceRecord(t *testing.T, file, mode string) acceptanceGenerationRecord {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var record acceptanceGenerationRecord
	if json.Unmarshal(data, &record) != nil || len(record.Batches) != 2 || len(record.Requests) != 2 {
		t.Fatal("incomplete generation record")
	}
	if record.Mode != mode && !(record.Mode == "" && mode == "feedback") {
		t.Fatal("input mode identity changed")
	}
	n := 0
	generate := func(_ context.Context, input TaskInput, feedback []CriterionDecision) (string, error) {
		if mode == "no-feedback" {
			feedback = nil
		}
		payload, err := harborRequest(input, feedback)
		if err != nil || n >= len(record.Requests) {
			t.Fatal("unexpected generation call", err)
		}
		var expected, actual any
		if json.Unmarshal(payload, &expected) != nil || json.Unmarshal(record.Requests[n], &actual) != nil || !reflect.DeepEqual(expected, actual) {
			t.Fatal("actual request or feedback changed")
		}
		batch := record.Batches[n]
		n++
		if batch.Profile != "qwen3-1.7b" || batch.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" || batch.PromptVersion != "rag-joint-v3" || len(batch.Rows) != 1 {
			t.Fatal("model identity changed")
		}
		row := batch.Rows[0]
		if !capturedHarborInputMatches(payload, row) {
			t.Fatal("actual model input no longer matches authorized request and feedback")
		}
		if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) || len(row.InputMessages) != 2 || row.InputMessages[1].Role != "user" {
			t.Fatal("generation budget or input capture changed")
		}
		if row.AtOutputLimit {
			return "", ErrAcceptance
		}
		return row.Raw, nil
	}
	input := TaskInput{ID: "both-premises", Input: harborQuestion, AllowedSources: []string{"deployment", "policy"}}
	policy := AcceptancePolicy{Version: "harbor-frozen-v1", Criteria: []string{"protocol", "citations", "conclusion"}, MaxCandidates: 2}
	report, err := RunAcceptance(context.Background(), input, policy, generate, ReviewHarborCandidate)
	if !errors.Is(err, ErrAcceptanceBudget) || err.Error() != record.Error || !reflect.DeepEqual(report, record.Report) || n != 2 || report.Output != "" {
		t.Fatal("original failure changed or candidate repaired", report, err)
	}
	if mode == "feedback" {
		if report.Attempts[1].Checks[0].Status != "pass" || report.Attempts[1].Checks[1].Status != "pass" || report.Attempts[1].Checks[2].Status != "fail" {
			t.Fatal("format improvement erased wrong conclusion")
		}
	} else if report.Attempts[1].Checks[0].Status != "fail" {
		t.Fatal("baseline failure was silently repaired")
	}
	return record
}

func TestAcceptanceActualRecordReplaysWithoutRepair(t *testing.T) {
	feedback := replayAcceptanceRecord(t, "testdata/acceptance-generation-v1.json", "feedback")
	baseline := replayAcceptanceRecord(t, "testdata/acceptance-no-feedback-v1.json", "no-feedback")
	if feedback.Report.GenerationCalls != baseline.Report.GenerationCalls || feedback.Report.ReviewCalls != baseline.Report.ReviewCalls || feedback.Report.Binding != baseline.Report.Binding || feedback.Report.Output != "" || baseline.Report.Output != "" {
		t.Fatal("different task/policy/budget or failed task became success")
	}
	for n := range baseline.Batches {
		if !reflect.DeepEqual(baseline.Batches[n].Rows[0].InputMessages, feedback.Batches[0].Rows[0].InputMessages) {
			t.Fatal("baseline was given extra information")
		}
	}
}

func TestHarborCaptureRejectsHiddenOrChangedInput(t *testing.T) {
	data, err := os.ReadFile("testdata/acceptance-generation-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var record acceptanceGenerationRecord
	if json.Unmarshal(data, &record) != nil || len(record.Batches) != 2 {
		t.Fatal("invalid record")
	}
	row := record.Batches[0].Rows[0]
	for _, content := range []string{
		strings.Replace(row.InputMessages[1].Content, "What is Harbor", "What is another deployment", 1),
		strings.Replace(row.InputMessages[1].Content, "fallback_enabled=false", "fallback_enabled=true", 1),
		`{"score":1,` + row.InputMessages[1].Content[1:],
		`{"question":"ignored",` + row.InputMessages[1].Content[1:],
	} {
		bad := row
		bad.InputMessages = append(bad.InputMessages[:0:0], row.InputMessages...)
		bad.InputMessages[1].Content = content
		if capturedHarborInputMatches(record.Requests[0], bad) {
			t.Fatal("changed or hidden input accepted")
		}
	}
}
