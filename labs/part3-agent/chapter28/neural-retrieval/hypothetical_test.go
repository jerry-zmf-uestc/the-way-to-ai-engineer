package main

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestHypotheticalContractCannotBecomeEvidence(t *testing.T) {
	h := Hypotheses{Revision: generatorRevision, PromptVersion: "hypothetical-v1", Rows: []Hypothesis{{Text: "unverified passage", InputTokens: 10, OutputTokens: 5, Trust: "hypothetical"}}}
	if err := ValidateHypotheses(h, 1); err != nil {
		t.Fatal(err)
	}
	h.Rows[0].Trust = "verified"
	if err := ValidateHypotheses(h, 1); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	h.Rows[0].Trust = "hypothetical"
	h.Rows[0].OutputTokens = 65
	if err := ValidateHypotheses(h, 1); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
}
func TestActualHypotheticalQueryContrast(t *testing.T) {
	if os.Getenv("HYPOTHETICAL_E2E") != "1" {
		t.Skip("explicit local hypothetical E2E")
	}
	if os.Getenv("EMBEDDING_E2E") != "1" {
		t.Fatal("actual encoder required")
	}
	ctx := context.Background()
	docs, questions, labels := fixtures()
	generated, err := GenerateHypothetical(ctx, os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS"), questions)
	if err != nil {
		t.Fatal(err)
	}
	// Neither corpus nor labels are passed to the generator. All corpus IDs remain real documents.
	texts := []string{}
	for _, d := range docs {
		texts = append(texts, d.Text)
	}
	for _, q := range questions {
		texts = append(texts, q.Text)
	}
	for _, h := range generated.Rows {
		texts = append(texts, h.Text)
	}
	batch, err := Encode(ctx, os.Getenv("EMBEDDING_PYTHON"), os.Getenv("EMBEDDING_ASSETS"), texts)
	if err != nil {
		t.Fatal(err)
	}
	known, directSum, hypSum := 0, 0.0, 0.0
	for n, q := range questions {
		direct, err := Dense(docs, batch.Vectors[:len(docs)], batch.Vectors[len(docs)+n], "demo", "v1", 1)
		if err != nil {
			t.Fatal(err)
		}
		alternative, err := Dense(docs, batch.Vectors[:len(docs)], batch.Vectors[len(docs)+len(questions)+n], "demo", "v1", 1)
		if err != nil {
			t.Fatal(err)
		}
		dr, covered, err := Recall(labels[n], direct)
		if err != nil {
			t.Fatal(err)
		}
		hr, _, err := Recall(labels[n], alternative)
		if err != nil {
			t.Fatal(err)
		}
		if covered {
			known++
			directSum += dr
			hypSum += hr
		}
		h := generated.Rows[n]
		t.Logf("case=%s direct=%s hypothetical=%s recall_known=%t recall=%.0f/%.0f generator_tokens=%d/%d at_limit=%t text=%q", q.ID, direct[0].ID, alternative[0].ID, covered, dr, hr, h.InputTokens, h.OutputTokens, h.AtOutputLimit, h.Text)
	}
	if known != 5 {
		t.Fatal(known)
	}
	t.Logf("generator_revision=%s direct_recall@1=%.2f hypothetical_recall@1=%.2f generation_source=not_evidence", generated.Revision, directSum/float64(known), hypSum/float64(known))
}
