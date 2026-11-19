package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

type jointGenerationRecord struct {
	CaseID string         `json:"case_id"`
	Batch  GeneratedBatch `json:"batch"`
	Answer JointAnswer    `json:"answer"`
	Error  string         `json:"error"`
	Pass   bool           `json:"rubric_pass"`
}

// This counterexample changes one field name in a copy; live generation never repairs it.
func TestJointFieldRepairCannotSupplyMissingPremise(t *testing.T) {
	data, err := os.ReadFile("testdata/joint-generation-1.7b-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []jointGenerationRecord
	if err := json.Unmarshal(data, &records); err != nil || len(records) != 3 {
		t.Fatal(err)
	}
	raw := records[0].Batch.Rows[0].Raw
	if _, err := ParseJoint(raw); !errors.Is(err, ErrDraft) {
		t.Fatal("original malformed field accepted", err)
	}
	changed := strings.ReplaceAll(raw, `"id":`, `"evidence_id":`)
	draft, err := ParseJoint(changed)
	if err != nil || len(draft.Claims) != 1 || len(draft.Claims[0].Citations) != 1 || draft.Claims[0].Citations[0].EvidenceID != "policy" {
		t.Fatal("counterexample did more than field repair", draft, err)
	}
	answer, err := RunJoint(context.Background(), "u", "What is Harbor current timeout in seconds?", jointGenerationCases()[0].items, scripted(changed), fixtureJointJudge)
	if !errors.Is(err, ErrReview) || len(answer.Claims) != 0 || answer.Abstained {
		t.Fatal("field repair invented a missing premise", answer, err)
	}
}

func TestJointGeneratedRecordsReplayWithoutRepair(t *testing.T) {
	for _, config := range []struct{ file, prompt, profile, revision string }{
		{"joint-generation-v1.json", "rag-joint-v1", "qwen3", "c1899de289a04d12100db370d81485cdf75e47ca"},
		{"joint-generation-v2.json", "rag-joint-v2", "qwen3", "c1899de289a04d12100db370d81485cdf75e47ca"},
		{"joint-generation-1.7b-v2.json", "rag-joint-v2", "qwen3-1.7b", "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e"},
		{"joint-generation-1.7b-v3.json", "rag-joint-v3", "qwen3-1.7b", "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e"},
	} {
		prompt := config.prompt
		data, err := os.ReadFile("testdata/" + config.file)
		if err != nil {
			t.Fatal(err)
		}
		var records []jointGenerationRecord
		if err := json.Unmarshal(data, &records); err != nil || len(records) != 3 {
			t.Fatal("incomplete generation records", err)
		}
		for n, c := range jointGenerationCases() {
			record := records[n]
			batch := record.Batch
			if record.CaseID != c.id || batch.Profile != config.profile || batch.Revision != config.revision || batch.PromptVersion != prompt || len(batch.Rows) != 1 {
				t.Fatal("record identity mismatch")
			}
			row := batch.Rows[0]
			if prompt == "rag-joint-v3" && validateCitationInput(row, DraftRequest{Question: "What is Harbor current timeout in seconds?", Evidence: c.items}) != nil {
				t.Fatal("actual input messages changed", c.id)
			}
			if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) || len(row.Raw) > 16384 {
				t.Fatal("record budget mismatch")
			}
			answer, err := RunJoint(context.Background(), "u", "What is Harbor current timeout in seconds?", c.items, LocalDraftModel(row), fixtureJointJudge)
			errorText := ""
			if err != nil {
				errorText = err.Error()
			}
			passed := err == nil && answer.Abstained == c.refusal
			if !c.refusal {
				passed = passed && len(answer.Claims) == 1 && answer.Claims[0].Text == "Harbor current timeout is 43 seconds."
			}
			if errorText != record.Error || !reflect.DeepEqual(answer, record.Answer) || passed != record.Pass {
				t.Fatal("original draft behavior changed", prompt, c.id, answer, err)
			}
		}
	}
}

func jointGenerationCases() []struct {
	id      string
	items   []Evidence
	refusal bool
} {
	_, registry := jointFixture()
	return []struct {
		id      string
		items   []Evidence
		refusal bool
	}{
		{"both-premises", []Evidence{registry["deployment"], registry["policy"]}, false},
		{"policy-only", []Evidence{registry["policy"]}, true},
		{"deployment-only", []Evidence{registry["deployment"]}, true},
	}
}

func TestActualJointGeneratedDrafts(t *testing.T) {
	if os.Getenv("RAG_GENERATION_E2E") != "1" {
		t.Skip("explicit local joint RAG generation E2E")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	prompt := os.Getenv("RAG_JOINT_PROMPT")
	if prompt == "" {
		prompt = "rag-joint-v1"
	}
	if prompt != "rag-joint-v1" && prompt != "rag-joint-v2" && prompt != "rag-joint-v3" {
		t.Fatal("registered joint prompt required")
	}
	profile := os.Getenv("RAG_JOINT_PROFILE")
	if profile == "" {
		profile = "qwen3"
	}
	if profile != "qwen3" && profile != "qwen3-1.7b" {
		t.Fatal("registered joint model required")
	}
	records := []jointGenerationRecord{}
	for _, c := range jointGenerationCases() {
		var batch GeneratedBatch
		model := func(ctx context.Context, question string, authorized []Evidence) (string, error) {
			var err error
			batch, err = GenerateLocalVariant(ctx, os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS"), profile, prompt, []DraftRequest{{Question: question, Evidence: authorized}})
			if err != nil {
				return "", err
			}
			return LocalDraftModel(batch.Rows[0])(ctx, question, authorized)
		}
		answer, err := RunJoint(ctx, "u", "What is Harbor current timeout in seconds?", c.items, model, fixtureJointJudge)
		if len(batch.Rows) != 1 {
			t.Fatal("generation did not finish", c.id, err)
		}
		if err != nil && (len(answer.Claims) != 0 || answer.Abstained) {
			t.Fatal("failed draft released", c.id, answer, err)
		}
		passed := err == nil && answer.Abstained == c.refusal
		if !c.refusal {
			passed = passed && len(answer.Claims) == 1 && answer.Claims[0].Text == "Harbor current timeout is 43 seconds."
		}
		errorText := ""
		if err != nil {
			errorText = err.Error()
		}
		records = append(records, jointGenerationRecord{c.id, batch, answer, errorText, passed})
	}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("joint_generation_records=%s", data)
}
