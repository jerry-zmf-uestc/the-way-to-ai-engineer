package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLocalPayloadKeepsScoringLabelsOutside(t *testing.T) {
	cases, docs := fixtures()
	limits := Limits{Reads: 8, Decisions: 16, Repairs: 1, Candidates: 2}
	cases[0].Labels.Values = map[string]string{"secret": "gold-canary-not-model-input"}
	data, err := localInput(cases, docs, limits)
	if err != nil || strings.Contains(string(data), "gold-canary") || strings.Contains(string(data), "Labels") {
		t.Fatal(err, string(data))
	}
}
func TestActualLocalStrategyDecisions(t *testing.T) {
	if os.Getenv("LOCAL_STRATEGY_E2E") != "1" {
		t.Skip("explicit local model strategy evaluation")
	}
	dataset := os.Getenv("STRATEGY_DATASET")
	if dataset == "" {
		dataset = "fixture-v1"
	}
	prompt := os.Getenv("STRATEGY_PROMPT")
	if prompt == "" {
		prompt = "strategy-decisions-v1"
	}
	cases, docs, err := LocalDataset(dataset)
	if err != nil {
		t.Fatal(err)
	}
	limits := Limits{Reads: 8, Decisions: 16, Repairs: 1, Candidates: 2}
	result, err := RunLocalStrategyVariant(context.Background(), os.Getenv("STRATEGY_PYTHON"), os.Getenv("STRATEGY_ASSETS"), cases, docs, limits, prompt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("dataset=%s prompt=%s", dataset, prompt)
	byID := map[string]Case{}
	for _, c := range cases {
		byID[c.Input.ID] = c
	}
	seen := map[string]bool{}
	for _, row := range result.Rows {
		c, ok := byID[row.CaseID]
		key := row.CaseID + "/" + row.Strategy
		if !ok || seen[key] || (row.Strategy != "react-local" && row.Strategy != "plan-local" && row.Strategy != "reflection-local") {
			t.Fatal("invalid case identity", row)
		}
		seen[key] = true
		var executionError error
		if row.Error != "" {
			executionError = ErrInput
		}
		t.Logf("case=%s strategy=%s complete=%t reads=%d decisions=%d error=%s generations=%d", row.CaseID, row.Strategy, Complete(c.Input, c.Labels, row.Report, executionError), row.Report.Reads, row.Report.Decisions, row.Error, len(row.Generations))
	}
	raw, _ := json.Marshal(result)
	t.Log("raw bounded synthetic diagnostics: " + string(raw))
}

func TestIndependentDatasetKeepsValuesOutsideExecutionLabels(t *testing.T) {
	cases, docs, err := LocalDataset("independent-v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = LocalDataset("../arbitrary.json"); err == nil {
		t.Fatal("unregistered dataset accepted")
	}
	cases[0].Labels.Values = map[string]string{"secret": "independent-gold-canary"}
	data, err := localInput(cases, docs, Limits{Reads: 8, Decisions: 16})
	if err != nil || strings.Contains(string(data), "independent-gold-canary") {
		t.Fatal(err)
	}
}

func TestRecordedIndependentV2DoesNotHideFormatFailures(t *testing.T) {
	data, err := os.ReadFile("testdata/local-strategy-v2-independent.json")
	if err != nil {
		t.Fatal(err)
	}
	var result LocalResult
	if json.Unmarshal(data, &result) != nil || result.PromptVersion != "strategy-decisions-v2" {
		t.Fatal("invalid record")
	}
	cases, docs, err := LocalDataset("independent-v1")
	if err != nil || validateLocalRows(result, cases, docs) != nil {
		t.Fatal("record identity/source mismatch", err)
	}
	for _, row := range result.Rows {
		if row.Error != "invalid_decision_json" || row.Report.Reads != 1 || len(row.Generations) != 1 {
			t.Fatal("failure filtered or repaired", row)
		}
	}
}

func TestRecordedV3SeparatesSyntaxFromTaskSuccess(t *testing.T) {
	data, err := os.ReadFile("testdata/local-strategy-v3-holdout.json")
	if err != nil {
		t.Fatal(err)
	}
	var result LocalResult
	if json.Unmarshal(data, &result) != nil || result.PromptVersion != "strategy-decisions-v3" {
		t.Fatal("invalid record")
	}
	cases, docs, err := LocalDataset("holdout-v3")
	if err != nil || validateLocalRows(result, cases, docs) != nil {
		t.Fatal(err)
	}
	byID := map[string]Case{}
	for _, c := range cases {
		byID[c.Input.ID] = c
	}
	passed := map[string]int{}
	for _, row := range result.Rows {
		if row.Error != "" {
			t.Fatal("constrained syntax failed", row)
		}
		for _, g := range row.Generations {
			if g.Mode != "likelihood" || g.OutputTokens != 0 || g.ForwardTokens <= g.InputTokens || len(g.Candidates) == 0 {
				t.Fatal("scoring fabricated as generation", g)
			}
		}
		c := byID[row.CaseID]
		if Complete(c.Input, c.Labels, row.Report, nil) {
			passed[row.Strategy]++
		}
	}
	if passed["react-local"] != 1 || passed["plan-local"] != 5 {
		t.Fatal(passed)
	}
}

func TestRecordedV4KeepsRepairBudgetConflictAndPartialSources(t *testing.T) {
	data, err := os.ReadFile("testdata/local-strategy-v4-holdout.json")
	if err != nil {
		t.Fatal(err)
	}
	var result LocalResult
	if json.Unmarshal(data, &result) != nil || result.PromptVersion != "strategy-decisions-v4" {
		t.Fatal("invalid record")
	}
	cases, docs, err := LocalDataset("holdout-v4")
	if err != nil || validateLocalRows(result, cases, docs) != nil {
		t.Fatal(err)
	}
	byID := map[string]Case{}
	for _, c := range cases {
		byID[c.Input.ID] = c
	}
	passed := map[string]int{}
	for _, row := range result.Rows {
		c := byID[row.CaseID]
		if row.Error != "" || row.Report.Reads > 8 || row.Report.Decisions > 16 || row.Report.Repairs > 1 {
			t.Fatal(row)
		}
		if Complete(c.Input, c.Labels, row.Report, nil) {
			passed[row.Strategy]++
		}
		if row.Strategy == "reflection-local" && row.CaseID == "new-q2" && (row.Report.Repairs != 1 || !row.Report.Refused || row.Report.Reads != 3) {
			t.Fatal("repair budget escaped", row)
		}
		if row.Strategy == "reflection-local" && row.CaseID == "new-q1" && row.Report.Facts["quota"].Source != "p0" {
			t.Fatal("completed source overwritten", row)
		}
		if row.Strategy == "reflection-local" && row.CaseID == "new-q3" && (len(row.Report.Conflicts) != 1 || !row.Report.Refused || row.Report.Facts["left"].Value != "5") {
			t.Fatal("conflict masked", row)
		}
	}
	if passed["react-local"] != 2 || passed["plan-local"] != 4 || passed["reflection-local"] != 5 {
		t.Fatal(passed)
	}
}

func TestRecordedLocalStrategyResultKeepsFailuresAndSourceOwnership(t *testing.T) {
	data, err := os.ReadFile("testdata/local-strategy-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var result LocalResult
	if json.Unmarshal(data, &result) != nil {
		t.Fatal("invalid record")
	}
	cases, docs := fixtures()
	if validateLocalRows(result, cases, docs) != nil {
		t.Fatal("invalid source ownership")
	}
	byID := map[string]Case{}
	for _, c := range cases {
		byID[c.Input.ID] = c
	}
	passed := map[string]int{}
	for _, row := range result.Rows {
		var executionError error
		if row.Error != "" {
			executionError = ErrInput
		}
		c := byID[row.CaseID]
		if Complete(c.Input, c.Labels, row.Report, executionError) {
			passed[row.Strategy]++
		}
	}
	if passed["react-local"] != 1 || passed["plan-local"] != 1 {
		t.Fatal(passed)
	}
	result.Rows[1].Report.Facts["timeout"] = Fact{Value: "30", Source: "old-timeout", Version: "v2"}
	if validateLocalRows(result, cases, docs) == nil {
		t.Fatal("forged source accepted")
	}
}
