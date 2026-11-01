package main

import (
	"errors"
	"math"
	"testing"
)

func TestSameRecallCanHideDifferentRankQuality(t *testing.T) {
	labels := map[string]int{"critical": 3, "useful": 1, "noise": 0}
	a, err := EvaluateRanking(labels, []string{"noise", "useful", "critical"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EvaluateRanking(labels, []string{"critical", "useful", "noise"}, 3)
	if err != nil || a.Recall != 1 || b.Recall != 1 || a.ReciprocalRank != .5 || b.ReciprocalRank != 1 || !(a.NDCG < b.NDCG) || math.Abs(b.NDCG-1) > 1e-12 {
		t.Fatal(a, b, err)
	}
	c, err := EvaluateRanking(labels, []string{"useful", "critical", "noise"}, 3)
	if err != nil || c.ReciprocalRank != b.ReciprocalRank || !(c.NDCG < b.NDCG) {
		t.Fatal("graded priority not represented", b, c, err)
	}
}

func TestUnjudgedAndZeroRelevanceDoNotInventScores(t *testing.T) {
	labels := map[string]int{"relevant": 3, "noise": 0}
	r, err := EvaluateRanking(labels, []string{"unknown", "relevant"}, 2)
	if err != nil || !r.RecallKnown || r.Recall != 1 || r.RRKnown || r.NDCGKnown || r.Judged != 1 || r.Ranked != 2 {
		t.Fatal(r, err)
	}
	r, err = EvaluateRanking(labels, []string{"relevant", "unknown"}, 2)
	if err != nil || !r.RRKnown || r.ReciprocalRank != 1 || r.NDCGKnown {
		t.Fatal(r, err)
	}
	r, err = EvaluateRanking(labels, nil, 2)
	if err != nil || !r.RecallKnown || !r.RRKnown || !r.NDCGKnown || r.Recall != 0 || r.NDCG != 0 {
		t.Fatal(r, err)
	}
	r, err = EvaluateRanking(map[string]int{"noise": 0}, []string{"noise"}, 1)
	if err != nil || r.RecallKnown || r.RRKnown || r.NDCGKnown {
		t.Fatal(r, err)
	}
}

func TestRankingCutoffAndInvalidInputs(t *testing.T) {
	r, err := EvaluateRanking(map[string]int{"a": 3, "b": 1}, []string{"b", "a"}, 1)
	if err != nil || r.Recall != .5 || r.ReciprocalRank != 1 || math.Abs(r.NDCG-1.0/7) > 1e-12 {
		t.Fatal(r, err)
	}
	for _, c := range []struct {
		labels  map[string]int
		ranking []string
		k       int
	}{
		{map[string]int{"a": 4}, nil, 1}, {map[string]int{"": 1}, nil, 1},
		{map[string]int{"a": 1}, []string{"a", "a"}, 1},
		{map[string]int{"a": 1}, []string{""}, 1},
		{nil, nil, 0}, {nil, make([]string, 129), 1},
	} {
		if _, err := EvaluateRanking(c.labels, c.ranking, c.k); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid ranking accepted", c, err)
		}
	}
}
