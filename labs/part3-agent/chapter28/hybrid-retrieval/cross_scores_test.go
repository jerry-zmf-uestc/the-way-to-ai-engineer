package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"
)

func TestOriginalLocalCrossEncoderScoresKeepSameCandidatePool(t *testing.T) {
	data, err := os.ReadFile("rerank-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var dataset struct {
		Version string `json:"version"`
		Cases   []struct {
			ID, Query  string
			Candidates []struct{ ID, Text string }
			Grades     map[string]int
		}
	}
	digest := sha256.Sum256(data)
	if err := json.Unmarshal(data, &dataset); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile("rerank-result.json")
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Revision    string `json:"revision"`
		CaseVersion string `json:"case_version"`
		CaseHash    string `json:"case_sha256"`
		Rows        []struct {
			Scores  []PairScore `json:"scores"`
			Seconds float64     `json:"seconds"`
		}
	}
	if err := json.Unmarshal(data, &record); err != nil || record.CaseHash != hex.EncodeToString(digest[:]) || record.CaseVersion != dataset.Version || record.Revision != "233902d25c440f23af6f7d6e94d2946bac0bee0a" || len(record.Rows) != len(dataset.Cases) {
		t.Fatal("record identity", err)
	}
	for n, c := range dataset.Cases {
		hits := []Hit{}
		before := []string{}
		for _, candidate := range c.Candidates {
			hits = append(hits, Hit{Chunk: Chunk{ID: candidate.ID, Text: candidate.Text, Tenant: "demo", Vector: []float64{1}}})
			before = append(before, candidate.ID)
		}
		ranked, err := ApplyCrossScores(context.Background(), hits, record.Rows[n].Scores)
		if err != nil {
			t.Fatal(err)
		}
		after := []string{}
		for _, hit := range ranked {
			after = append(after, hit.Chunk.ID)
		}
		base, _ := EvaluateRanking(c.Grades, before, 3)
		next, _ := EvaluateRanking(c.Grades, after, 3)
		if base.Recall != next.Recall || len(ranked) != len(hits) {
			t.Fatal("rerank changed recall pool")
		}
		t.Logf("case=%s before=%v after=%v recall=%.3f RR=%.3f/%.3f nDCG=%.3f/%.3f seconds=%.4f", c.ID, before, after, next.Recall, base.ReciprocalRank, next.ReciprocalRank, base.NDCG, next.NDCG, record.Rows[n].Seconds)
	}
}

func TestCrossScoreBatchRejectsNewMissingDuplicateOrInvalidEntries(t *testing.T) {
	hits := []Hit{{Chunk: Chunk{ID: "a"}}, {Chunk: Chunk{ID: "b"}}}
	for _, scores := range [][]PairScore{
		{{"a", 1, 8}}, {{"a", 1, 8}, {"a", 2, 8}}, {{"a", 1, 8}, {"new", 2, 8}},
		{{"a", math.NaN(), 8}, {"b", 2, 8}}, {{"a", 1, 513}, {"b", 2, 8}},
	} {
		if result, err := ApplyCrossScores(context.Background(), hits, scores); !errors.Is(err, ErrInvalid) || result != nil {
			t.Fatal(result, err)
		}
	}
}
