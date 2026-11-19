package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRetrievedTransportRejectsIdentityAndAmbiguityBeforeGeneration(t *testing.T) {
	batch := RetrievedBatch{Protocol: "retrieved-evidence-v1", Revision: "1110a243fdf4706b3f48f1d95db1a4f5529b4d41", Space: "minilm-l6-v2-onnx-mean-l2-256-v1", Rows: []RetrievedRow{{ID: "q", Question: "Timeout?", Evidence: []Evidence{{ID: "d", Source: "fixture:d", Tenant: "demo", Version: "v1", Text: "Timeout is 9 seconds."}}}}}
	raw, _ := json.Marshal(batch)
	if _, err := ReadRetrieved(strings.NewReader(string(raw))); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(string(raw), `"demo"`, `"foreign"`, 1),
		strings.Replace(string(raw), `"v1"`, `"old"`, 1),
		strings.Replace(string(raw), `"fixture:d"`, `"forged"`, 1),
		strings.Replace(string(raw), `"Rows":`, `"expected":"gold","Rows":`, 1),
		strings.Replace(string(raw), `"Protocol":`, `"protocol":"retrieved-evidence-v1","Protocol":`, 1),
		strings.Replace(string(raw), `"Space":`, `"ſpace":"minilm-l6-v2-onnx-mean-l2-256-v1","Space":`, 1),
		string(raw) + `{}`,
		strings.Repeat("x", (64<<10)+1),
	} {
		if _, err := ReadRetrieved(strings.NewReader(bad)); !errors.Is(err, ErrEvidence) {
			t.Fatal("invalid exchange accepted", err)
		}
	}
	batch.Rows[0].Evidence[0].Tenant = "foreign"
	if _, err := RunRetrieved(context.Background(), batch, "/missing/python", "/missing/assets", "qwen3"); !errors.Is(err, ErrEvidence) {
		t.Fatal("invalid input reached generator", err)
	}
}

func TestRetrievedEmptyEvidenceDoesNotInvokeGenerator(t *testing.T) {
	batch := RetrievedBatch{Protocol: "retrieved-evidence-v1", Revision: "1110a243fdf4706b3f48f1d95db1a4f5529b4d41", Space: "minilm-l6-v2-onnx-mean-l2-256-v1", Rows: []RetrievedRow{{ID: "q", Question: "Timeout?"}}}
	rows, err := RunRetrieved(context.Background(), batch, "/missing/python", "/missing/assets", "qwen3")
	if err != nil || len(rows) != 1 || rows[0].Status != "abstained" || rows[0].Answer.Reason != "no_evidence" || rows[0].InputTokens != 0 {
		t.Fatal(rows, err)
	}
}
