package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRetryArchiveRetainsFailureAndReplaysOutcome(t *testing.T) {
	for _, config := range []string{"tool-recovered", "tool-exhausted"} {
		t.Run(config, func(t *testing.T) {
			archive, err := record(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			archive, err = load(strings.NewReader(encoded(archive)))
			if err != nil || archive.Schema != 3 {
				t.Fatal(archive, err)
			}
			run := archive.Runs[1]
			if run.ToolLimit != 2 || run.Calls[1].Error != "tool_failed" || run.Calls[1].Output != "" || run.Calls[2].Kind != "tool-read" || run.Calls[2].InputHash != run.Calls[1].InputHash {
				t.Fatal("retry lost original failure or changed input", run)
			}
			report, err := evaluate(context.Background(), archive)
			if err != nil || report.Total != 4 || !report.Rows[1].ReplayVerified || report.UsageKnown || report.CostKnown {
				t.Fatal(report, err)
			}
			if config == "tool-recovered" {
				if len(run.Calls) != 4 || report.Passed != 4 || !report.Gate || report.ExecutionFailed != 0 || report.Rows[1].Steps != 3 || report.Rows[1].Attempts != 4 {
					t.Fatal("recovery hid extra attempt", run, report)
				}
			} else if len(run.Calls) != 3 || run.Error != "tool_failed" || report.Passed != 3 || report.Gate || report.ExecutionFailed != 1 || report.Rows[1].Steps != 1 || report.Rows[1].Attempts != 3 {
				t.Fatal("exhaustion lost denominator or final failure", run, report)
			}
		})
	}
}

func TestRetryReplayRejectsChangedPolicyAndTrace(t *testing.T) {
	archive, _ := record(context.Background(), "tool-recovered")
	for _, mutate := range []func(*Archive){
		func(a *Archive) { a.Schema = 2 },
		func(a *Archive) { a.Runs[1].ToolLimit = 0 },
		func(a *Archive) { a.Runs[1].ToolLimit = 3 },
		func(a *Archive) { a.Runs[1].Calls[2].InputHash = hash("changed") },
		func(a *Archive) { a.Runs[1].Calls[2].Kind = "model-final" },
		func(a *Archive) { a.Runs[1].Calls[1].Error = "protocol_failed" },
		func(a *Archive) { a.Runs[1].Calls[2].Error = "tool_failed"; a.Runs[1].Calls[2].Output = "" },
		func(a *Archive) { a.Runs[1].Calls = append(a.Runs[1].Calls, a.Runs[1].Calls[3]) },
		func(a *Archive) { a.Runs[1].Calls = a.Runs[1].Calls[:3] },
	} {
		copy, _ := load(strings.NewReader(encoded(archive)))
		mutate(&copy)
		if report, err := evaluate(context.Background(), copy); !errors.Is(err, ErrRecord) || report.Total != 0 {
			t.Fatal("invalid retry archive accepted", report, err)
		}
	}
}

func TestBoundedRetryStopsOnCancellationProtocolAndBudget(t *testing.T) {
	for _, limit := range []int{0, 3} {
		_, err := executeBounded(context.Background(), cases()[1].Input, func(context.Context, string, string) (string, error) {
			t.Fatal("invalid limit reached driver")
			return "", nil
		}, limit)
		if !errors.Is(err, ErrRecord) {
			t.Fatal(limit, err)
		}
	}
	for _, failure := range []error{ErrTool, ErrRecord, context.Canceled, context.DeadlineExceeded, errors.Join(ErrTool, context.Canceled)} {
		calls := 0
		_, err := executeBounded(context.Background(), cases()[1].Input, func(_ context.Context, kind, input string) (string, error) {
			if kind == "model-select" {
				return input, nil
			}
			if kind != "tool-read" {
				t.Fatal("continued to final", kind)
			}
			calls++
			return "", failure
		}, 2)
		want := 1
		if failure == ErrTool {
			want = 2
		}
		if !errors.Is(err, failure) || calls != want {
			t.Fatal(failure, err, calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := executeBounded(ctx, cases()[1].Input, func(_ context.Context, kind, input string) (string, error) {
		if kind == "model-select" {
			return input, nil
		}
		calls++
		cancel()
		return "", ErrTool
	}, 2)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal(err, calls)
	}
}
