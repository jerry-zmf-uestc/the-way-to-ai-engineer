package main

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

func TestEmbeddingContractRejectsWrongSpaceShapeAndNonfinite(t *testing.T) {
	batch := Batch{Revision: revision, Space: space, Dimension: 384, Truncated: []bool{false}, Vectors: [][]float64{make([]float64, 384)}}
	batch.Vectors[0][0] = 1
	if err := Validate(batch, 1); err != nil {
		t.Fatal(err)
	}
	copy := batch
	copy.Space = "another"
	if err := Validate(copy, 1); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	copy = batch
	copy.Revision = "new"
	if err := Validate(copy, 1); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	batch.Vectors[0][0] = math.NaN()
	if err := Validate(batch, 1); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
}
func TestRetrievalScopeAndUnknownRelevance(t *testing.T) {
	docs, queries, labels := fixtures()
	hits := Lexical(docs, queries[1].Text, "demo", "v1", 1)
	for _, hit := range hits {
		if hit.ID == "other-user" || hit.ID == "old-timeout" {
			t.Fatal(hit)
		}
	}
	if _, known, err := Recall(labels[len(labels)-1], hits); err != nil || known {
		t.Fatal(known, err)
	}
}
func TestActualLocalPretrainedEmbedding(t *testing.T) {
	if os.Getenv("EMBEDDING_E2E") != "1" {
		t.Skip("explicit pretrained embedding")
	}
	if os.Getenv("EMBEDDING_PYTHON") == "" || os.Getenv("EMBEDDING_ASSETS") == "" {
		t.Fatal("local assets/runtime required")
	}
	docs, queries, labels := fixtures()
	texts := []string{}
	for _, d := range docs {
		texts = append(texts, d.Text)
	}
	for _, q := range queries {
		texts = append(texts, q.Text)
	}
	batch, err := Encode(context.Background(), os.Getenv("EMBEDDING_PYTHON"), os.Getenv("EMBEDDING_ASSETS"), texts)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Evaluate(docs, queries, labels, batch)
	if err != nil {
		t.Fatal(err)
	}
	covered, lexical, dense := 0, 0.0, 0.0
	for _, row := range rows {
		t.Logf("case=%s lex=%s dense=%s known=%t recall=%.0f/%.0f truncated=%t", row.ID, row.Lexical[0].ID, row.Dense[0].ID, row.RecallKnown, row.LexicalRecall, row.DenseRecall, row.Truncated)
		if row.RecallKnown {
			covered++
			lexical += row.LexicalRecall
			dense += row.DenseRecall
		}
	}
	if len(rows) != len(queries) || covered != 5 {
		t.Fatal(rows)
	}
	// This verifies actual output coverage/shape. Do not make a tiny corpus universally require neural superiority.
	longBatch, err := Encode(context.Background(), os.Getenv("EMBEDDING_PYTHON"), os.Getenv("EMBEDDING_ASSETS"), []string{strings.Repeat("hello ", 500)})
	if err != nil || !longBatch.Truncated[0] {
		t.Fatal("truncation not observable", err)
	}
	t.Logf("revision=%s space=%s dim=%d known_cases=%d lexical_recall@1=%.2f dense_recall@1=%.2f", batch.Revision, batch.Space, batch.Dimension, covered, lexical/float64(covered), dense/float64(covered))
}
