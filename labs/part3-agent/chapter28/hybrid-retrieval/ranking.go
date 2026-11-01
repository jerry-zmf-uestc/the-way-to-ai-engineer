package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type RankingEvaluation struct {
	Recall, ReciprocalRank, NDCG    float64
	RecallKnown, RRKnown, NDCGKnown bool
	Relevant, Judged, Ranked        int
}

// EvaluateRanking使用独立等级标签；没有标签的结果不当作不相关。
func EvaluateRanking(labels map[string]int, ranking []string, k int) (RankingEvaluation, error) {
	if len(labels) > 128 || len(ranking) > 128 || k < 1 || k > 128 {
		return RankingEvaluation{}, ErrInvalid
	}
	ideal := make([]int, 0, len(labels))
	r := RankingEvaluation{}
	for id, grade := range labels {
		if strings.TrimSpace(id) == "" || len(id) > 256 || grade < 0 || grade > 3 {
			return RankingEvaluation{}, ErrInvalid
		}
		ideal = append(ideal, grade)
		if grade > 0 {
			r.Relevant++
		}
	}
	seen := map[string]bool{}
	for _, id := range ranking {
		if strings.TrimSpace(id) == "" || len(id) > 256 || seen[id] {
			return RankingEvaluation{}, ErrInvalid
		}
		seen[id] = true
	}
	ranking = ranking[:min(k, len(ranking))]
	r.Ranked = len(ranking)
	found, first := 0, 0
	unknownBeforeFirst := false
	dcg := 0.0
	for n, id := range ranking {
		grade, known := labels[id]
		if !known {
			if first == 0 {
				unknownBeforeFirst = true
			}
			continue
		}
		r.Judged++
		if grade > 0 {
			found++
			if first == 0 {
				first = n + 1
			}
		}
		dcg += (math.Exp2(float64(grade)) - 1) / math.Log2(float64(n+2))
	}
	if r.Relevant == 0 {
		return r, nil
	}
	r.RecallKnown = true
	r.Recall = float64(found) / float64(r.Relevant)
	if !unknownBeforeFirst {
		r.RRKnown = true
		if first > 0 {
			r.ReciprocalRank = 1 / float64(first)
		}
	}
	if r.Judged == r.Ranked {
		sort.Sort(sort.Reverse(sort.IntSlice(ideal)))
		idcg := 0.0
		for n, grade := range ideal[:min(k, len(ideal))] {
			idcg += (math.Exp2(float64(grade)) - 1) / math.Log2(float64(n+2))
		}
		r.NDCGKnown = true
		r.NDCG = dcg / idcg
	}
	return r, nil
}

func rankingDemo() {
	labels := map[string]int{"critical": 3, "useful": 1, "noise": 0}
	for _, order := range [][]string{{"noise", "useful", "critical"}, {"critical", "useful", "noise"}} {
		r, err := EvaluateRanking(labels, order, 3)
		if err != nil {
			panic(err)
		}
		fmt.Printf("order=%v recall=%.2f rr=%.2f ndcg=%.3f judged=%d/%d\n", order, r.Recall, r.ReciprocalRank, r.NDCG, r.Judged, r.Ranked)
	}
}
