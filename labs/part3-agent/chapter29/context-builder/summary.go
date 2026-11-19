package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrSummary = errors.New("context: summary does not preserve declared facts")

// Snapshot由应用从可信状态取得；版本与来源标识本身不提供认证。
type Fact struct {
	Key, Value, Source, State string
}
type Snapshot struct {
	Version string
	Facts   []Fact
}

func factSet(s Snapshot) (map[string]Fact, error) {
	if strings.TrimSpace(s.Version) == "" || len(s.Version) > 128 || len(s.Facts) == 0 || len(s.Facts) > 32 {
		return nil, ErrSummary
	}
	seen := make(map[string]Fact, len(s.Facts))
	size := 0
	for _, f := range s.Facts {
		size += len(f.Key) + len(f.Value) + len(f.Source) + len(f.State)
		if size > 64<<10 || strings.TrimSpace(f.Key) == "" || len(f.Key) > 128 || strings.TrimSpace(f.Source) == "" || len(f.Source) > 128 {
			return nil, ErrSummary
		}
		if f.State != "known" && f.State != "unknown" || f.State == "known" && strings.TrimSpace(f.Value) == "" || f.State == "unknown" && f.Value != "" {
			return nil, ErrSummary
		}
		if _, ok := seen[f.Key]; ok {
			return nil, ErrSummary
		}
		seen[f.Key] = f
	}
	return seen, nil
}

// ValidateSummary检查显式保留字段，不解释自由文本，不修改原历史。
func ValidateSummary(ctx context.Context, current, candidate Snapshot) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	wanted, err := factSet(current)
	if err != nil {
		return Snapshot{}, err
	}
	actual, err := factSet(candidate)
	if err != nil || candidate.Version != current.Version || len(actual) != len(wanted) {
		return Snapshot{}, ErrSummary
	}
	for key, f := range wanted {
		if actual[key] != f {
			return Snapshot{}, ErrSummary
		}
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Version: candidate.Version, Facts: append([]Fact(nil), candidate.Facts...)}, nil
}

func summaryExample() Snapshot {
	return Snapshot{Version: "task-v2", Facts: []Fact{
		{Key: "budget_cny", Value: "6000", Source: "user-update-2", State: "known"},
		{Key: "offline_required", Value: "true", Source: "user-request-1", State: "known"},
		{Key: "option_b_offline", Source: "research-gap-1", State: "unknown"},
	}}
}

func summaryDemo() {
	current := summaryExample()
	for _, kind := range []string{"complete", "missing", "old_amount", "wrong_condition", "false_source", "unknown_to_known"} {
		candidate := Snapshot{Version: current.Version, Facts: append([]Fact(nil), current.Facts...)}
		switch kind {
		case "missing":
			candidate.Facts = candidate.Facts[:2]
		case "old_amount":
			candidate.Facts[0].Value = "5000"
		case "wrong_condition":
			candidate.Facts[1].Value = "false"
		case "false_source":
			candidate.Facts[0].Source = "model-suggestion"
		case "unknown_to_known":
			candidate.Facts[2].State, candidate.Facts[2].Value = "known", "true"
		}
		accepted, err := ValidateSummary(context.Background(), current, candidate)
		fmt.Printf("candidate=%s accepted=%t published_facts=%d\n", kind, err == nil, len(accepted.Facts))
	}
}
