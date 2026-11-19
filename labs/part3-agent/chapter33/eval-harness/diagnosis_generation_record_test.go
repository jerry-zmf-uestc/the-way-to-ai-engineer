package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDiagnosisGeneratedRecordPreservesUnknownCauses(t *testing.T) {
	data, err := os.ReadFile("testdata/diagnosis-generation-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var record DiagnosisGenerationRecord
	if json.Unmarshal(data, &record) != nil || record.PromptVersion != "runtime-diagnosis-v1" || record.Profile != "qwen3-1.7b" || record.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" {
		t.Fatal("record identity")
	}
	dataset := diagnosisFixture(t)
	if record.DatasetVersion != dataset.Version || len(record.Runs) != len(dataset.Cases) {
		t.Fatal("dataset/run mismatch")
	}
	reviews, failed, actions, refs := 0, 0, 0, 0
	for n, run := range record.Runs {
		c := dataset.Cases[n]
		input, err := BuildDiagnosisInput(c)
		if err != nil || run.CaseID != c.ID || !reflect.DeepEqual(input, run.Input) || !diagnosisInputCaptured(input, run.Generation) {
			t.Fatal("original input changed", c.ID, err)
		}
		row := run.Generation
		if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) {
			t.Fatal("record budget changed")
		}
		score, err := ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v1", c, row.Raw, nil)
		errorText := ""
		if row.AtOutputLimit {
			score.Status = "failed"
			errorText = "output_limit"
		} else if err != nil {
			errorText = err.Error()
		}
		if !reflect.DeepEqual(score, run.Score) || errorText != run.Error || score.CauseKnown || score.Status == "passed" {
			t.Fatal("raw record repaired or cause fabricated", c.ID)
		}
		if score.Status == "review" {
			reviews++
		} else if score.Status == "failed" {
			failed++
		}
		if score.ActionCorrect {
			actions++
		}
		if score.EvidenceComplete {
			refs++
		}
	}
	if reviews != 2 || failed != 6 || actions != 6 || refs != 2 {
		t.Fatal("scoring scope changed", reviews, failed, actions, refs)
	}
	// The explanation/action disagreement stays in the original model record.
	if !strings.Contains(record.Runs[4].Generation.Raw, "report_role_completed") {
		t.Fatal("wrong recommendation repaired")
	}
}
