package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

type capturedJointRecord struct {
	Schema     int                   `json:"schema"`
	CaseID     string                `json:"case_id"`
	Submitted  []PromptRequest       `json:"submitted"`
	Generation jointGenerationRecord `json:"generation"`
}

func TestCapturedJointRecordsBindSubmittedInputWithoutRepair(t *testing.T) {
	data, err := os.ReadFile("testdata/joint-captured-1.7b-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []capturedJointRecord
	if err := json.Unmarshal(data, &records); err != nil || len(records) != 3 {
		t.Fatal(err)
	}
	for n, c := range jointGenerationCases() {
		r := records[n]
		if r.Schema != 1 || r.CaseID != c.id || r.Generation.CaseID != c.id || len(r.Submitted) != 1 {
			t.Fatal("record identity", r.CaseID)
		}
		payload, err := modelPayload([]DraftRequest{{Question: "What is Harbor current timeout in seconds?", Evidence: c.items}})
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(payload)
		var expected []PromptRequest
		if err := json.Unmarshal(payload, &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(r.Submitted, expected) || r.Generation.Batch.InputSHA256 != hex.EncodeToString(h[:]) {
			t.Fatal("input/record drift", c.id)
		}
		batch := r.Generation.Batch
		if batch.Profile != "qwen3-1.7b" || batch.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" || batch.PromptVersion != "rag-joint-v2" || len(batch.Rows) != 1 {
			t.Fatal("generation identity", c.id)
		}
		answer, runErr := RunJoint(context.Background(), "u", r.Submitted[0].Question, c.items, LocalDraftModel(batch.Rows[0]), fixtureJointJudge)
		errorText := ""
		if runErr != nil {
			errorText = runErr.Error()
		}
		if errorText != r.Generation.Error || !reflect.DeepEqual(answer, r.Generation.Answer) || r.Generation.Pass {
			t.Fatal("original failure changed", c.id, runErr)
		}
	}
}

func TestActualJointCapturedInputs(t *testing.T) {
	if os.Getenv("RAG_GENERATION_E2E") != "1" {
		t.Skip("explicit local generation gate")
	}
	profile, prompt := os.Getenv("RAG_JOINT_PROFILE"), os.Getenv("RAG_JOINT_PROMPT")
	if profile != "qwen3-1.7b" || prompt != "rag-joint-v2" {
		t.Fatal("fixed registered capture profile required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	records := []capturedJointRecord{}
	for _, c := range jointGenerationCases() {
		var submitted []PromptRequest
		var batch GeneratedBatch
		model := func(ctx context.Context, question string, authorized []Evidence) (string, error) {
			requests := []DraftRequest{{Question: question, Evidence: authorized}}
			payload, err := modelPayload(requests)
			if err != nil {
				return "", err
			}
			if err := json.Unmarshal(payload, &submitted); err != nil {
				return "", err
			}
			batch, err = GenerateLocalVariant(ctx, os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS"), profile, prompt, requests)
			if err != nil {
				return "", err
			}
			h := sha256.Sum256(payload)
			if batch.InputSHA256 != hex.EncodeToString(h[:]) {
				t.Fatal("captured projection differs from actual submission")
			}
			return LocalDraftModel(batch.Rows[0])(ctx, question, authorized)
		}
		answer, err := RunJoint(ctx, "u", "What is Harbor current timeout in seconds?", c.items, model, fixtureJointJudge)
		if len(batch.Rows) != 1 || len(submitted) != 1 {
			t.Fatal("capture incomplete", c.id, err)
		}
		passed := err == nil && answer.Abstained == c.refusal
		if !c.refusal {
			passed = passed && len(answer.Claims) == 1 && answer.Claims[0].Text == "Harbor current timeout is 43 seconds."
		}
		errorText := ""
		if err != nil {
			errorText = err.Error()
		}
		records = append(records, capturedJointRecord{Schema: 1, CaseID: c.id, Submitted: submitted, Generation: jointGenerationRecord{c.id, batch, answer, errorText, passed}})
	}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("captured_joint_records=%s", data)
}
