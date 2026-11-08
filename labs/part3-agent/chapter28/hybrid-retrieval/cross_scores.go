package main

import (
	"context"
	"math"
)

type PairScore struct {
	ID     string  `json:"id"`
	Score  float64 `json:"score"`
	Tokens int     `json:"tokens"`
}

// The caller binds this batch to the same query and authorized candidate snapshot.
func ApplyCrossScores(ctx context.Context, hits []Hit, scores []PairScore) ([]Hit, error) {
	if len(hits) == 0 || len(hits) > 8 || len(scores) != len(hits) {
		return nil, ErrInvalid
	}
	known := map[string]bool{}
	for _, hit := range hits {
		if hit.Chunk.ID == "" || known[hit.Chunk.ID] {
			return nil, ErrInvalid
		}
		known[hit.Chunk.ID] = true
	}
	values := map[string]float64{}
	for _, score := range scores {
		_, duplicate := values[score.ID]
		if !known[score.ID] || duplicate || score.Tokens < 1 || score.Tokens > 512 || math.IsNaN(score.Score) || math.IsInf(score.Score, 0) {
			return nil, ErrInvalid
		}
		values[score.ID] = score.Score
	}
	return Rerank(ctx, hits, func(_ context.Context, chunk Chunk) (float64, error) { return values[chunk.ID], nil })
}
