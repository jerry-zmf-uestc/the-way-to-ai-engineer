package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestExportSelectedOriginalEvidenceWithoutLabels(t *testing.T) {
	docs := []Document{{"d", "demo", "v1", "Original source."}, {"foreign", "other", "v1", "Do not export."}}
	questions := []Question{{"q", "What is the source?"}}
	batch := Batch{Revision: revision, Space: space, Dimension: 384, Truncated: make([]bool, 3)}
	for range 3 {
		v := make([]float64, 384)
		v[0] = 1
		batch.Vectors = append(batch.Vectors, v)
	}
	got, err := ExportCandidates(context.Background(), docs, questions, batch)
	if err != nil || len(got.Rows) != 1 || len(got.Rows[0].Evidence) != 1 || got.Rows[0].Evidence[0].ID != "d" || got.Rows[0].Evidence[0].Text != docs[0].Text {
		t.Fatal(got, err)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "Relevant") || strings.Contains(string(raw), "foreign") || strings.Contains(string(raw), "Score") {
		t.Fatal("evaluation or unauthorized data exported")
	}
	for _, i := range []int{0, 2} {
		batch.Truncated[i] = true
		if _, err = ExportCandidates(context.Background(), docs, questions, batch); !errors.Is(err, ErrInput) {
			t.Fatal("truncated input exported", err)
		}
		batch.Truncated[i] = false
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = ExportCandidates(ctx, docs, questions, batch); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
