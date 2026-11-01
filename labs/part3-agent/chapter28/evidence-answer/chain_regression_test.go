package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// Replaying recorded drafts proves validation behavior, not a fresh model execution.
func TestRetrievedRecordedOutcomes(t *testing.T) {
	raw, err := os.ReadFile("testdata/retrieved-regression-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Version string          `json:"dataset_version"`
		Input   json.RawMessage `json:"retrieval"`
		Rows    []ChainRow      `json:"generation_rows"`
	}
	if json.Unmarshal(raw, &record) != nil || record.Version != "retrieved-regression-v1" || len(record.Rows) != 6 {
		t.Fatal("invalid regression record")
	}
	batch, err := ReadRetrieved(bytes.NewReader(record.Input))
	if err != nil || len(batch.Rows) != len(record.Rows) {
		t.Fatal(err)
	}
	for i, row := range record.Rows {
		if row.ID != batch.Rows[i].ID || row.Profile != "qwen3" || row.ModelRevision != "c1899de289a04d12100db370d81485cdf75e47ca" || row.PromptVersion != "rag-draft-v4" {
			t.Fatal("record identity mismatch")
		}
		draft := GeneratedDraft{Raw: row.DiagnosticDraft, AtOutputLimit: row.AtOutputLimit}
		answer, err := Run(context.Background(), "demo", batch.Rows[i].Question, batch.Rows[i].Evidence, LocalDraftModel(draft), ExactSourceJudge)
		switch row.Status {
		case "accepted_extract":
			if err != nil || answer.Abstained || len(answer.Claims) == 0 {
				t.Fatal(row.ID, answer, err)
			}
		case "support_unknown":
			if !errors.Is(err, ErrReview) || len(answer.Claims) != 0 {
				t.Fatal(row.ID, answer, err)
			}
		case "draft_failed":
			if !errors.Is(err, ErrDraft) || len(answer.Claims) != 0 {
				t.Fatal(row.ID, answer, err)
			}
		case "abstained":
			if err != nil || !answer.Abstained || answer.Reason != "model_insufficient_evidence" {
				t.Fatal(row.ID, answer, err)
			}
		default:
			t.Fatal("unknown recorded outcome", row.Status)
		}
	}
}
