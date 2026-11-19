package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func diagnosisFixture(t *testing.T) DiagnosisDataset {
	t.Helper()
	file, err := os.Open("diagnosis-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	dataset, err := LoadDiagnosisCases(file)
	if err != nil {
		t.Fatal(err)
	}
	return dataset
}

func TestDiagnosisCasesProjectOnlyTaskEventsAndActions(t *testing.T) {
	dataset := diagnosisFixture(t)
	if len(dataset.Cases) != 8 {
		t.Fatal("definition coverage changed")
	}
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	scenarios := map[string]bool{}
	for _, c := range dataset.Cases {
		if scenarios[c.Scenario] {
			t.Fatal("renamed scenario counted twice")
		}
		scenarios[c.Scenario] = true
		assertContractSource(t, root, c.Source.Path)
		input, err := BuildDiagnosisInput(c)
		if err != nil {
			t.Fatal(err)
		}
		var view map[string]json.RawMessage
		if json.Unmarshal([]byte(input.Input), &view) != nil || len(view) != 3 || view["expected"] != nil || view["source"] != nil || view["scenario"] != nil {
			t.Fatal("scoring metadata reached model")
		}
		changed := c
		changed.Expected.Cause, changed.Expected.Action = "hidden-label", "hidden-action"
		changed.Source.Contract = "private scoring instructions"
		other, err := BuildDiagnosisInput(changed)
		if err != nil || !reflect.DeepEqual(input, other) {
			t.Fatal("scoring-side change altered task input", err)
		}
		if len(input.AllowedSources) != len(c.Events) || len(input.AllowedTools) != 0 {
			t.Fatal("event scope not projected")
		}
	}
}

func TestDiagnosisDefinitionsRejectBrokenScoringRelations(t *testing.T) {
	dataset := diagnosisFixture(t)
	for _, alter := range []func(*DiagnosisDataset){
		func(d *DiagnosisDataset) { d.Cases[1].ID = d.Cases[0].ID },
		func(d *DiagnosisDataset) { d.Cases[0].Expected.Action = "not-allowed" },
		func(d *DiagnosisDataset) { d.Cases[0].Expected.Refs = []string{"unknown"} },
		func(d *DiagnosisDataset) { d.Cases[0].Expected.Refs = []string{"e1", "e1"} },
		func(d *DiagnosisDataset) { d.Cases[0].Events[1].ID = "e1" },
		func(d *DiagnosisDataset) { d.Cases[0].Events[0].Text = strings.Repeat("x", 1025) },
		func(d *DiagnosisDataset) { d.Cases[0].AllowedActions = []string{"check", "check"} },
		func(d *DiagnosisDataset) { d.Cases[0].Split = "heldout" },
	} {
		original, _ := json.Marshal(dataset)
		var bad DiagnosisDataset
		if json.Unmarshal(original, &bad) != nil {
			t.Fatal("bad fixture")
		}
		alter(&bad)
		encoded, _ := json.Marshal(bad)
		if _, err := LoadDiagnosisCases(bytes.NewReader(encoded)); err == nil {
			t.Fatal("invalid definition accepted")
		}
	}
}
