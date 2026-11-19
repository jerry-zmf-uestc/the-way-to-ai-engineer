package main

import (
	"context"
	"errors"
	"slices"
	"testing"
)

var commonLimits = Limits{Reads: 8, Decisions: 16, Repairs: 1, Candidates: 2}

func caseNamed(t *testing.T, name string) (Case, map[string]Document) {
	t.Helper()
	cases, docs := fixtures()
	for _, c := range cases {
		if c.Input.ID == name {
			return c, docs
		}
	}
	t.Fatal(name)
	return Case{}, nil
}
func TestSameCasesAndBudgetKeepFailures(t *testing.T) {
	cases, docs := fixtures()
	strategies := []string{"direct", "react-control", "self-ask-control", "plan-control", "reflection-control"}
	counts := map[string]int{}
	for _, strategy := range strategies {
		for _, c := range cases {
			report, err := Run(context.Background(), c.Input, docs, commonLimits, strategy)
			if Complete(c.Input, c.Labels, report, err) {
				counts[strategy]++
			}
			if report.Reads > commonLimits.Reads || report.Decisions > commonLimits.Decisions || report.Repairs > commonLimits.Repairs || report.UsageKnown {
				t.Fatal(report)
			}
		}
	}
	if counts["reflection-control"] != len(cases) || counts["plan-control"] >= counts["reflection-control"] || counts["self-ask-control"] == len(cases) {
		t.Fatal(counts)
	}
}
func TestSubquestionUsesObservationToContinueMultiHop(t *testing.T) {
	c, docs := caseNamed(t, "nested")
	report, err := Run(context.Background(), c.Input, docs, commonLimits, "self-ask-control")
	if !Complete(c.Input, c.Labels, report, err) || report.Reads != 3 {
		t.Fatal(report, err)
	}
	parents := []string{}
	for _, e := range report.Events {
		if e.Kind == "followup_question" {
			parents = append(parents, e.Document)
		}
	}
	if !slices.Equal(parents, []string{"nested", "router"}) {
		t.Fatal(parents)
	}
}
func TestCritiqueUsesRequirementsNotScoringAnswers(t *testing.T) {
	c, docs := caseNamed(t, "alias")
	first, err := Run(context.Background(), c.Input, docs, commonLimits, "reflection-control")
	if err != nil || first.Repairs != 1 || first.Reads != 3 || first.Facts["timeout"].Value != "30" {
		t.Fatal(first, err)
	}
	c.Labels.Values["timeout"] = "gold_changed"
	second, err := Run(context.Background(), c.Input, docs, commonLimits, "reflection-control")
	if err != nil || second.Facts["timeout"] != first.Facts["timeout"] || Complete(c.Input, c.Labels, second, err) {
		t.Fatal(second, err)
	}
	if len(Missing(c.Input, first.Facts).Missing) != 0 {
		t.Fatal("critic used gold")
	}
}
func TestVersionFailureAndBudgetCannotBeHiddenByRepair(t *testing.T) {
	c, docs := caseNamed(t, "stale")
	report, err := Run(context.Background(), c.Input, docs, commonLimits, "plan-control")
	if err != nil || !report.Refused || len(report.Facts) != 0 {
		t.Fatal(report, err)
	}
	limited := commonLimits
	limited.Reads = 2
	repaired, err := Run(context.Background(), c.Input, docs, limited, "reflection-control")
	if !errors.Is(err, ErrBudget) || repaired.Reads != 2 || Complete(c.Input, c.Labels, repaired, err) {
		t.Fatal(repaired, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	canceled, err := Run(ctx, c.Input, docs, commonLimits, "react-control")
	if !errors.Is(err, context.Canceled) || canceled.Reads != 0 {
		t.Fatal(canceled, err)
	}
}

func TestRepairPreservesCompletedFactsAndDoesNotHideConflict(t *testing.T) {
	_, docs := fixtures()
	docs["two-stage"] = Document{ID: "two-stage", Version: "v2", Values: map[string]string{"retry": "2"}, Links: map[string][]string{"timeout": {"absent", "timeout"}}}
	input := Input{ID: "preserve", Root: "two-stage", Version: "v2", Required: []string{"retry", "timeout"}}
	timeout := docs["timeout"]
	timeout.Values["retry"] = "2"
	docs["timeout"] = timeout
	report, err := Run(context.Background(), input, docs, commonLimits, "reflection-control")
	if err != nil || report.Facts["retry"].Source != "two-stage" || report.Refused {
		t.Fatal(report, err)
	}
	timeout.Values["retry"] = "999"
	docs["timeout"] = timeout
	conflict, err := Run(context.Background(), input, docs, commonLimits, "reflection-control")
	if err != nil || !conflict.Refused || len(conflict.Conflicts) != 1 || conflict.Facts["retry"].Value != "2" {
		t.Fatal(conflict, err)
	}
}
