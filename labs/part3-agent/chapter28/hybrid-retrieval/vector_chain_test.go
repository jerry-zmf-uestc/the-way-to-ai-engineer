package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestOriginalVectorRerankChainKeepsActualBaseline(t *testing.T) {
	data, err := os.ReadFile("rerank-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	var dataset struct {
		Version string
		Cases   []struct {
			ID, Query  string
			Candidates []struct{ ID, Text string }
			Grades     map[string]int
		}
	}
	if err := json.Unmarshal(data, &dataset); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile("vector-rerank-result.json")
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		CaseHash    string `json:"case_sha256"`
		CaseVersion string `json:"case_version"`
		Encoder     struct {
			Revision, Space string
			Dimension       int
			Truncated       []bool
			Vectors         [][]float64
		}
		Reranker struct {
			Revision string
			Rows     []struct{ Scores []PairScore }
		}
		Rows []struct {
			CaseID      string   `json:"case_id"`
			DenseOrder  []string `json:"dense_order"`
			RerankOrder []string `json:"rerank_order"`
		}
	}
	if err := json.Unmarshal(data, &record); err != nil || record.CaseHash != hex.EncodeToString(digest[:]) || record.CaseVersion != dataset.Version || record.Encoder.Revision != "1110a243fdf4706b3f48f1d95db1a4f5529b4d41" || record.Reranker.Revision != "233902d25c440f23af6f7d6e94d2946bac0bee0a" || record.Encoder.Space != "minilm-l6-v2-onnx-mean-l2-256-v1" || record.Encoder.Dimension != 384 || len(record.Rows) != 3 || len(record.Reranker.Rows) != 3 || len(record.Encoder.Vectors) != 12 || len(record.Encoder.Truncated) != 12 {
		t.Fatal("chain identity/shape mismatch", err)
	}
	for n, vector := range record.Encoder.Vectors {
		if record.Encoder.Truncated[n] || len(vector) != 384 {
			t.Fatal("incomplete vector")
		}
		for _, value := range vector {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Fatal("nonfinite vector")
			}
		}
	}
	offset := 0
	for n, c := range dataset.Cases {
		query := record.Encoder.Vectors[offset]
		offset++
		hits := []Hit{}
		for _, candidate := range c.Candidates {
			vector := record.Encoder.Vectors[offset]
			offset++
			score, err := cosine(query, vector)
			if err != nil {
				t.Fatal(err)
			}
			hits = append(hits, Hit{Chunk: Chunk{ID: candidate.ID, Text: candidate.Text, Vector: vector}, Score: score})
		}
		rank(hits)
		before := []string{}
		for _, hit := range hits {
			before = append(before, hit.Chunk.ID)
		}
		ranked, err := ApplyCrossScores(t.Context(), hits, record.Reranker.Rows[n].Scores)
		if err != nil {
			t.Fatal(err)
		}
		after := []string{}
		for _, hit := range ranked {
			after = append(after, hit.Chunk.ID)
		}
		if record.Rows[n].CaseID != c.ID || !reflect.DeepEqual(before, record.Rows[n].DenseOrder) || !reflect.DeepEqual(after, record.Rows[n].RerankOrder) {
			t.Fatal("actual ranking changed")
		}
		base, _ := EvaluateRanking(c.Grades, before, 3)
		next, _ := EvaluateRanking(c.Grades, after, 3)
		t.Logf("case=%s dense=%v reranked=%v nDCG=%.3f/%.3f RR=%.3f/%.3f", c.ID, before, after, base.NDCG, next.NDCG, base.ReciprocalRank, next.ReciprocalRank)
	}
}
