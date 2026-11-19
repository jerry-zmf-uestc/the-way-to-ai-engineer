package main

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

type ExperienceScope struct {
	Run, TaskKind, DataVersion, ToolVersion, EvaluatorVersion string
}
type FeedbackMemo struct {
	Scope                ExperienceScope
	SourceAttempt        int
	FeedbackCode, Advice string
	ExpiresAt            time.Time
}

func validExperienceScope(s ExperienceScope) bool {
	for _, v := range []string{s.Run, s.TaskKind, s.DataVersion, s.ToolVersion, s.EvaluatorVersion} {
		if strings.TrimSpace(v) == "" || len(v) > 128 || !utf8.ValidString(v) {
			return false
		}
	}
	return true
}

// SelectExperience选择语言建议，不把它当作事实、授权或评分答案。
func SelectExperience(ctx context.Context, scope ExperienceScope, attempt int, now time.Time, memos []FeedbackMemo, byteBudget int) ([]FeedbackMemo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validExperienceScope(scope) || attempt < 1 || attempt > 16 || now.IsZero() || len(memos) > 8 || byteBudget < 1 || byteBudget > 4096 {
		return nil, ErrInput
	}
	seen := map[int]bool{}
	for _, m := range memos {
		if !validExperienceScope(m.Scope) || m.SourceAttempt < 1 || m.SourceAttempt > 16 || m.ExpiresAt.IsZero() || strings.TrimSpace(m.Advice) == "" || len(m.Advice) > 1024 || !utf8.ValidString(m.Advice) {
			return nil, ErrInput
		}
		if m.FeedbackCode != "missing_evidence" && m.FeedbackCode != "invalid_plan" && m.FeedbackCode != "task_failed" {
			return nil, ErrInput
		}
		if m.Scope == scope {
			if seen[m.SourceAttempt] {
				return nil, ErrInput
			}
			seen[m.SourceAttempt] = true
		}
	}
	selected := []FeedbackMemo{}
	for _, m := range memos {
		if m.Scope != scope || m.SourceAttempt >= attempt || !now.Before(m.ExpiresAt) {
			continue
		}
		selected = append(selected, m)
	}
	payload, err := json.Marshal(selected)
	if err != nil || len(payload) > byteBudget {
		return nil, ErrBudget
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return selected, nil
}
