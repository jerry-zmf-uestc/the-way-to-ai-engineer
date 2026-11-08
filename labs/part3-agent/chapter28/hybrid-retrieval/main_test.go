package main

import (
	"context"
	"errors"
	"math"
	"testing"
)

func fixture(t *testing.T) Index {
	t.Helper()
	index, err := NewIndex("s", 2, []Chunk{
		{ID: "lex", Source: "a", Version: "1", Tenant: "u", Text: "error E42", Space: "s", Vector: []float64{0, 1}},
		{ID: "vector", Source: "b", Version: "1", Tenant: "u", Text: "deadline reference", Space: "s", Vector: []float64{1, 0}},
		{ID: "both", Source: "c", Version: "1", Tenant: "u", Text: "E42 timeout", Space: "s", Vector: []float64{0.8, 0.2}},
		{ID: "secret", Source: "d", Version: "1", Tenant: "other", Text: "E42 private", Space: "s", Vector: []float64{1, 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return index
}
func TestFusionAndPreFilter(t *testing.T) {
	i := fixture(t)
	hits, err := i.Search(context.Background(), "u", "E42", "s", []float64{1, 0}, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 || hits[0].Chunk.ID != "both" || hits[0].LexicalRank == 0 || hits[0].VectorRank == 0 {
		t.Fatalf("fusion=%+v", hits)
	}
	for _, h := range hits {
		if h.Chunk.Tenant != "u" {
			t.Fatal("tenant leak")
		}
	}
	none, err := i.Search(context.Background(), "absent", "E42", "s", []float64{1, 0}, 3, 3)
	if err != nil || len(none) != 0 {
		t.Fatal("empty retrieval became error")
	}
}
func TestSplitVersionsAndCopies(t *testing.T) {
	a, err := Split("doc", "1", "u", "甲乙丙\n\n丁", 2)
	if err != nil || len(a) != 3 || a[0].Text != "甲乙" {
		t.Fatalf("chunks=%+v err=%v", a, err)
	}
	b, _ := Split("doc", "2", "u", "甲乙丙\n\n丁", 2)
	if a[0].ID == b[0].ID {
		t.Fatal("version lost")
	}
	chunks := []Chunk{{ID: "x", Source: "d", Version: "1", Tenant: "u", Text: "x", Space: "s", Vector: []float64{1, 0}}}
	i, err := NewIndex("s", 2, chunks)
	if err != nil {
		t.Fatal(err)
	}
	chunks[0].Vector[0] = 0
	hits, _ := i.Search(context.Background(), "u", "x", "s", []float64{1, 0}, 1, 1)
	hits[0].Chunk.Vector[0] = 0
	again, _ := i.Search(context.Background(), "u", "x", "s", []float64{1, 0}, 1, 1)
	if len(again) != 1 || again[0].Chunk.Vector[0] != 1 {
		t.Fatal("shared vector")
	}
}
func TestRejectsBadSpacesVectorsAndLimits(t *testing.T) {
	i := fixture(t)
	for _, v := range [][]float64{{}, {0, 0}, {1}, {math.NaN(), 1}, {math.Inf(1), 1}} {
		if _, err := i.Search(context.Background(), "u", "q", "s", v, 2, 1); !errors.Is(err, ErrInvalid) {
			t.Fatalf("bad vector accepted: %v", v)
		}
	}
	if _, err := i.Search(context.Background(), "u", "q", "other-space", []float64{1, 0}, 2, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal("same dimension different model accepted")
	}
	if _, err := i.Search(context.Background(), "u", "q", "s", []float64{1, 0}, 1, 129); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid limits")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := i.Search(ctx, "u", "q", "s", []float64{1, 0}, 2, 1); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
}
func TestRerankerCannotAddDocumentsOrHideErrors(t *testing.T) {
	hits, _ := fixture(t).Search(context.Background(), "u", "E42", "s", []float64{1, 0}, 3, 3)
	out, err := Rerank(context.Background(), hits, func(_ context.Context, c Chunk) (float64, error) {
		if c.ID == "lex" {
			return 10, nil
		}
		return 0, nil
	})
	if err != nil || out[0].Chunk.ID != "lex" || len(out) != len(hits) {
		t.Fatal("rerank failed")
	}
	cause := errors.New("rank service down")
	if _, err := Rerank(context.Background(), hits, func(context.Context, Chunk) (float64, error) { return 0, cause }); !errors.Is(err, cause) {
		t.Fatal("cause lost")
	}
	if _, err := Rerank(context.Background(), hits, func(context.Context, Chunk) (float64, error) { return math.NaN(), nil }); !errors.Is(err, ErrInvalid) {
		t.Fatal("bad score accepted")
	}
}
