package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReportedTokenSumCannotWrap(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	p := policy()
	p.MaxTokens = maximum
	next := 0
	jobs := []Job{
		{Name: "first", Kind: "model", Execute: func(context.Context) (Result, error) { return Result{Text: "accepted", Tokens: 1}, nil }},
		{Name: "overflow", Kind: "model", Execute: func(context.Context) (Result, error) { return Result{Text: "rejected", Tokens: maximum}, nil }},
		{Name: "later", Kind: "tool", Execute: func(context.Context) (Result, error) { next++; return Result{}, nil }},
	}
	report, err := Run(context.Background(), p, jobs)
	if !errors.Is(err, ErrInvalid) || !report.UsageOverflow || report.OverflowUsage != maximum || report.Tokens != 1 || next != 0 || len(report.Outputs) != 1 || report.Outputs[0] != "accepted" {
		t.Fatalf("unrepresentable usage lost or wrapped: report=%+v next=%d err=%v", report, next, err)
	}
	checkTerminal(t, report, Failed)
}

func TestRunDeadlineIncludesRetryBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p := policy()
	p.Backoff = time.Hour
	calls := 0
	r, err := Run(ctx, p, []Job{{Name: "retry", Kind: "model", RetrySafe: true, Execute: func(context.Context) (Result, error) {
		calls++
		return Result{Tokens: 2}, ErrTemporary
	}}})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 || r.ModelCalls != 1 || r.Tokens != 2 || len(r.Outputs) != 0 {
		t.Fatalf("deadline reset during backoff: %+v calls=%d err=%v", r, calls, err)
	}
	checkTerminal(t, r, Canceled)
}

func TestRepresentableTokenTotalsKeepTheirMeaning(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	for _, c := range []struct {
		name                 string
		limit, first, second int
		cause                error
	}{
		{"exact maximum", maximum, maximum - 1, 1, nil},
		{"ordinary overshoot", 10, 1, 11, ErrBudget},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := policy()
			p.MaxTokens = c.limit
			jobs := []Job{
				{Name: "first", Kind: "model", Execute: func(context.Context) (Result, error) { return Result{Tokens: c.first}, nil }},
				{Name: "second", Kind: "model", Execute: func(context.Context) (Result, error) { return Result{Tokens: c.second}, nil }},
			}
			r, err := Run(context.Background(), p, jobs)
			if !errors.Is(err, c.cause) || r.Tokens != c.first+c.second || r.UsageOverflow || r.OverflowUsage != 0 || r.ModelCalls != 2 {
				t.Fatalf("report=%+v err=%v", r, err)
			}
			state := Completed
			if c.cause != nil {
				state = Failed
			}
			checkTerminal(t, r, state)
		})
	}
}

func TestCanceledOverflowPreservesBothUsageParts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := policy()
	p.MaxTokens = int(^uint(0) >> 1)
	jobs := []Job{
		{Name: "first", Kind: "model", Execute: func(context.Context) (Result, error) { return Result{Tokens: 1}, nil }},
		{Name: "late", Kind: "model", Execute: func(context.Context) (Result, error) { cancel(); return Result{Text: "late", Tokens: p.MaxTokens}, nil }},
	}
	r, err := Run(ctx, p, jobs)
	if !errors.Is(err, context.Canceled) || !r.UsageOverflow || r.Tokens != 1 || r.OverflowUsage != p.MaxTokens || len(r.Outputs) != 1 || r.ModelCalls != 2 {
		t.Fatalf("canceled usage lost: %+v %v", r, err)
	}
	checkTerminal(t, r, Canceled)
}
