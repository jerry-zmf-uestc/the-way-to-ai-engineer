package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestAccountingGeneratedRecordIsNotRepaired(t *testing.T) {
	data, err := os.ReadFile("testdata/accounting-generation-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var record AccountingGenerationRecord
	if json.Unmarshal(data, &record) != nil || record.PromptVersion != "resource-accounting-v1" || record.Profile != "qwen3-1.7b" || record.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" {
		t.Fatal("record identity")
	}
	dataset := accountingFixture(t)
	if record.DatasetVersion != dataset.Version || len(record.Runs) != 12 {
		t.Fatal("record coverage")
	}
	parsed := 0
	for n, run := range record.Runs {
		c := dataset.Cases[n]
		input, err := BuildAccountingInput(c)
		if err != nil || run.CaseID != c.ID || !reflect.DeepEqual(input, run.Input) || !accountingInputCaptured(input, run.Generation) {
			t.Fatal("actual input changed", c.ID, err)
		}
		row := run.Generation
		if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) {
			t.Fatal("record budget")
		}
		score, err := ScoreAccounting(c, row.Raw)
		errorText := ""
		if row.AtOutputLimit {
			score.Status = "failed"
			errorText = "output_limit"
		} else if err != nil {
			errorText = err.Error()
		} else {
			parsed++
		}
		if !reflect.DeepEqual(score, run.Score) || errorText != run.Error || score.Status != "failed" {
			t.Fatal("original failure altered", c.ID, score, err)
		}
	}
	if parsed != 1 || !strings.Contains(record.Runs[0].Generation.Raw, string([]byte{96, 96, 96})) {
		t.Fatal("fenced output silently repaired")
	}
}
