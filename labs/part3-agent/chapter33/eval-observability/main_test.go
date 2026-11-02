package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRecordedExecutionReplayAndCriticalGate(t *testing.T) {
	for _, config := range []string{"baseline", "regressed"} {
		archive, err := record(context.Background(), config)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := load(strings.NewReader(encoded(archive)))
		if err != nil {
			t.Fatal(err)
		}
		report, err := evaluate(context.Background(), loaded)
		if err != nil || report.Total != 4 || report.UsageKnown || report.CostKnown {
			t.Fatal(report, err)
		}
		for n, row := range report.Rows {
			if row.RunID != archive.Runs[n].RunID || row.CaseID != cases()[n].ID || row.Split != cases()[n].Split || row.Steps != 3 {
				t.Fatal("lost execution-to-score association", row)
			}
		}
		if config == "baseline" && (!report.Gate || report.Passed != 4 || report.CriticalFailed != 0) {
			t.Fatal("baseline rejected", report)
		}
		if config == "regressed" && (report.Gate || report.Passed != 3 || report.CriticalFailed != 1) {
			t.Fatal("75% average hid required refusal failure", report)
		}
	}
}

func TestReplayRejectsMismatchIncompleteAndRelabeledRuns(t *testing.T) {
	archive, _ := record(context.Background(), "baseline")
	for _, mutate := range []func(*Archive){
		func(a *Archive) { a.Runs[0].Calls[1].InputHash = strings.Repeat("0", 64) },
		func(a *Archive) { a.Runs[0].Calls = a.Runs[0].Calls[:2] },
		func(a *Archive) { a.Runs[0].Calls[0].Seq = 2 },
		func(a *Archive) { a.Runs[1].RunID = a.Runs[0].RunID },
		func(a *Archive) { a.Runs[0].Answer.Text = "changed" },
		func(a *Archive) { a.Runs[0].Calls[2].Output = `{"Text":"x","text":"y"}` },
		func(a *Archive) { a.Dataset = "different" },
	} {
		copy, err := load(strings.NewReader(encoded(archive)))
		if err != nil {
			t.Fatal(err)
		}
		mutate(&copy)
		if report, err := evaluate(context.Background(), copy); !errors.Is(err, ErrRecord) || report.Total != 0 {
			t.Fatal("invalid archive produced report", report, err)
		}
	}
}

func TestCancellationPreservesAllCaseDenominators(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	archive, err := record(ctx, "baseline")
	if err != nil || len(archive.Runs) != 4 {
		t.Fatal(archive, err)
	}
	report, err := evaluate(context.Background(), archive)
	if err != nil || report.Total != 4 || report.Passed != 0 || report.CriticalFailed != 2 || report.Gate {
		t.Fatal(report, err)
	}
	for _, run := range archive.Runs {
		if run.Error != "canceled" || len(run.Calls) != 0 || run.Answer != (Answer{}) {
			t.Fatal("canceled case disappeared or produced output", run)
		}
	}
}

func TestArchiveRejectsAmbiguousUnknownAndOversizedJSON(t *testing.T) {
	for _, input := range []string{`{"Schema":1,"schema":2}`, `{"Unknown":1}`, `{}` + `{}`, strings.Repeat("x", (128<<10)+1), string([]byte{0xff})} {
		if _, err := load(strings.NewReader(input)); !errors.Is(err, ErrRecord) {
			t.Fatal("invalid JSON accepted", err)
		}
	}
	// Task projection has no Expected/Critical/Split fields.
	data, _ := json.Marshal(cases()[0].Input)
	for _, label := range []string{"Expected", "Critical", "Split"} {
		if strings.Contains(string(data), label) {
			t.Fatal("scoring label entered executor")
		}
	}
}

func TestComparisonUsesSameCaseAndScorerIdentity(t *testing.T) {
	base, _ := record(context.Background(), "baseline")
	next, _ := record(context.Background(), "regressed")
	before, _ := evaluate(context.Background(), base)
	after, _ := evaluate(context.Background(), next)
	regressions, err := compare(before, after)
	if err != nil || len(regressions) != 1 || regressions[0] != "absent" {
		t.Fatal(regressions, err)
	}
	after.Scorer = "other"
	if _, err := compare(before, after); !errors.Is(err, ErrRecord) {
		t.Fatal("mixed scorer comparison", err)
	}
}

func TestFailedToolIsRecordedReplayedAndCompared(t *testing.T) {
	archive, err := record(context.Background(), "tool-failure")
	if err != nil {
		t.Fatal(err)
	}
	failed := archive.Runs[1]
	if failed.Error != "tool_failed" || failed.Answer != (Answer{}) || len(failed.Calls) != 2 || failed.Calls[1].Error != "tool_failed" || failed.Calls[1].Output != "" {
		t.Fatal("failed call missing or final continued", failed)
	}
	answer, err := replay(context.Background(), cases()[1], failed)
	if !errors.Is(err, ErrTool) || answer != (Answer{}) {
		t.Fatal("failure not reproduced", answer, err)
	}
	report, err := evaluate(context.Background(), archive)
	if err != nil || report.Total != 4 || report.Passed != 3 || report.ExecutionFailed != 1 || report.Gate || report.Rows[1].Steps != 1 || report.Rows[1].Attempts != 2 || !report.Rows[1].ReplayVerified {
		t.Fatal(report, err)
	}
	baseline, _ := record(context.Background(), "baseline")
	before, _ := evaluate(context.Background(), baseline)
	regressions, err := compare(before, report)
	if err != nil || len(regressions) != 1 || regressions[0] != "retry" {
		t.Fatal(regressions, err)
	}
}

func TestTerminalStopAndCallerCancellationAreSeparate(t *testing.T) {
	archive, _ := record(context.Background(), "baseline")
	// A protocol fixture represents a stop after the final response was recorded.
	archive.Runs[0].Error = "canceled"
	archive.Runs[0].Answer = Answer{}
	answer, err := replay(context.Background(), cases()[0], archive.Runs[0])
	if !errors.Is(err, context.Canceled) || answer != (Answer{}) {
		t.Fatal("recorded terminal stop lost", answer, err)
	}
	report, err := evaluate(context.Background(), archive)
	if err != nil || report.Passed != 3 || report.ExecutionFailed != 1 || report.Gate || !report.Rows[0].ReplayVerified {
		t.Fatal(report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if report, err := evaluate(ctx, archive); !errors.Is(err, context.Canceled) || report.Total != 0 {
		t.Fatal("caller cancellation fabricated verification", report, err)
	}
}

func TestFailedArchiveCannotBypassTraceVerification(t *testing.T) {
	archive, _ := record(context.Background(), "tool-failure")
	for _, mutate := range []func(*Run){
		func(r *Run) { r.Calls[1].InputHash = strings.Repeat("0", 64) },
		func(r *Run) { r.Calls[1].Output = "partial data" },
		func(r *Run) { r.Calls[1].Error = "unknown" },
		func(r *Run) { r.Calls[0].Error = "tool_failed" },
		func(r *Run) { r.Error = "protocol_failed" },
		func(r *Run) {
			r.Calls = append(r.Calls, Call{Seq: 3, Kind: "model-final", InputHash: hash("unused"), Output: encoded(Answer{})})
		},
	} {
		copy, _ := load(strings.NewReader(encoded(archive)))
		mutate(&copy.Runs[1])
		if report, err := evaluate(context.Background(), copy); !errors.Is(err, ErrRecord) || report.Total != 0 {
			t.Fatal("failed row escaped trace validation", report, err)
		}
	}
}

func TestLegacySuccessAndIncompleteFailureStayDistinct(t *testing.T) {
	archive, _ := record(context.Background(), "baseline")
	archive.Schema = 1
	report, err := evaluate(context.Background(), archive)
	if err != nil || report.Passed != 4 {
		t.Fatal("legacy success broken", report, err)
	}
	archive.Runs[1].Calls = archive.Runs[1].Calls[:1]
	archive.Runs[1].Error = "execution_failed"
	archive.Runs[1].Answer = Answer{}
	report, err = evaluate(context.Background(), archive)
	if err != nil || report.Total != 4 || report.Passed != 3 || report.Rows[1].ReplayVerified {
		t.Fatal("legacy summary claimed full failure replay", report, err)
	}
}
