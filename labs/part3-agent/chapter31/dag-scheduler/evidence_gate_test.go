package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCompletedDependencyStillNeedsBusinessInput(t *testing.T) {
	for _, value := range []string{"30", "not found"} {
		called := false
		nodes := []Node{
			{ID: "timeout", Run: func(context.Context, map[string]string) (string, error) { return value, nil }},
			{ID: "retry", Run: func(context.Context, map[string]string) (string, error) { return "2", nil }},
			{ID: "check", Depends: []string{"timeout", "retry"}, Run: CheckReportInputs},
			{ID: "summary", Depends: []string{"check"}, Run: func(_ context.Context, deps map[string]string) (string, error) {
				called = true
				return deps["check"], nil
			}},
			{ID: "independent", Run: func(context.Context, map[string]string) (string, error) { return "valid", nil }},
		}
		report, err := Run(context.Background(), nodes, 2, 8)
		if report.States["timeout"] != "completed" || report.States["retry"] != "completed" || report.States["independent"] != "completed" {
			t.Fatal("execution results lost", report)
		}
		if value == "30" {
			if err != nil || !called || report.Results["summary"] != "timeout_seconds=30 retry_count=2" {
				t.Fatal(report, err)
			}
		} else if !errors.Is(err, ErrEvidence) || !errors.Is(err, ErrFailed) || called || report.States["check"] != "failed" || report.States["summary"] != "blocked" {
			t.Fatal("incomplete input reached summary", report, err)
		}
	}
}

func TestReportInputContractAndCancellation(t *testing.T) {
	for _, deps := range []map[string]string{
		nil, {"timeout": "30"}, {"timeout": "30", "retry": "2", "extra": "x"},
		{"timeout": "0", "retry": "2"}, {"timeout": "301", "retry": "2"},
		{"timeout": "30秒", "retry": "2"}, {"timeout": "30", "retry": "-1"},
		{"timeout": "30", "retry": "11"}, {"timeout": "30", "retry": "2.5"},
		{"timeout": strings.Repeat("0", 32) + "1", "retry": "2"},
	} {
		if text, err := CheckReportInputs(context.Background(), deps); !errors.Is(err, ErrEvidence) || text != "" {
			t.Fatal("invalid input accepted", text, err)
		}
	}
	for _, deps := range []map[string]string{{"timeout": "1", "retry": "0"}, {"timeout": "300", "retry": "10"}, {"timeout": strings.Repeat("0", 31) + "1", "retry": "0"}} {
		if _, err := CheckReportInputs(context.Background(), deps); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if text, err := CheckReportInputs(ctx, map[string]string{"timeout": "30", "retry": "2"}); !errors.Is(err, context.Canceled) || text != "" {
		t.Fatal("canceled input passed", text, err)
	}
}
