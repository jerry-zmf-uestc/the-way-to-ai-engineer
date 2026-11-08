package main

import (
	"context"
	"errors"
	"testing"
)

func TestRerankOutputCannotRewriteOriginalCandidates(t *testing.T) {
	hits := []Hit{{Chunk: Chunk{ID: "a", Vector: []float64{1, 0}}, Score: 3}}
	out, err := Rerank(context.Background(), hits, func(_ context.Context, c Chunk) (float64, error) { c.Vector[0] = 99; return 1, nil })
	if err != nil || hits[0].Chunk.Vector[0] != 1 || out[0].Chunk.Vector[0] != 1 {
		t.Fatal("callback altered snapshots", hits, out, err)
	}
	out[0].Chunk.Vector[0] = 7
	if hits[0].Chunk.Vector[0] != 1 || hits[0].Score != 3 {
		t.Fatal("output shared input", hits)
	}
	if _, err = Rerank(context.Background(), hits, nil); !errors.Is(err, ErrInvalid) {
		t.Fatal("nil scorer", err)
	}
	if _, err = Rerank(context.Background(), make([]Hit, 129), func(context.Context, Chunk) (float64, error) { t.Fatal("invalid batch executed"); return 0, nil }); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
