package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func accountingFixture(t *testing.T) AccountingDataset {
	t.Helper()
	file, err := os.Open("accounting-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	dataset, err := LoadAccountingCases(file)
	if err != nil {
		t.Fatal(err)
	}
	return dataset
}

func TestAccountingDefinitionsPreserveUnknownAndSeparateInput(t *testing.T) {
	dataset := accountingFixture(t)
	if len(dataset.Cases) != 12 {
		t.Fatal("case coverage changed")
	}
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	scenarios := map[string]bool{}
	unknown := 0
	for _, c := range dataset.Cases {
		if scenarios[c.Scenario] {
			t.Fatal("renamed scenario counted twice")
		}
		scenarios[c.Scenario] = true
		assertContractSource(t, root, c.Source.Path)
		input, err := BuildAccountingInput(c)
		if err != nil {
			t.Fatal(err)
		}
		changed := c
		changed.Expected.Rule = "hidden-oracle"
		changed.Source.Contract = "scoring only"
		other, err := BuildAccountingInput(changed)
		if err != nil || !reflect.DeepEqual(input, other) {
			t.Fatal("scoring fields changed model input", err)
		}
		var view map[string]json.RawMessage
		if json.Unmarshal([]byte(input.Input), &view) != nil || len(view) != 2 || view["expected"] != nil || view["source"] != nil {
			t.Fatal("scoring metadata leaked")
		}
		result, err := AccountingOracle(c)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Known {
			unknown++
			if result.Value != nil {
				t.Fatal("unknown became zero")
			}
		}
	}
	if unknown != 2 {
		t.Fatal("unknown fee/tokenizer tasks lost")
	}
}

func TestAccountingOracleAndUnitsRejectBrokenDefinitions(t *testing.T) {
	dataset := accountingFixture(t)
	for _, change := range []func(*AccountingDataset){
		func(d *AccountingDataset) { d.Cases[0].Expected.Unit = "USD" },
		func(d *AccountingDataset) { value := int64(0); d.Cases[1].Expected.Value = &value },
		func(d *AccountingDataset) { d.Cases[0].Expected.Known = false },
		func(d *AccountingDataset) { d.Cases[0].Facts[0].ID = "wrong" },
		func(d *AccountingDataset) { d.Cases[0].Facts[1].ID = "limit" },
		func(d *AccountingDataset) { d.Cases[0].Facts[0].Value = 1_000_001 },
		func(d *AccountingDataset) { d.Cases[0].Facts[0].Value = 1 },
		func(d *AccountingDataset) { d.Cases[1].ID = d.Cases[0].ID },
	} {
		raw, _ := json.Marshal(dataset)
		var altered AccountingDataset
		if json.Unmarshal(raw, &altered) != nil {
			t.Fatal("bad fixture")
		}
		change(&altered)
		bad, _ := json.Marshal(altered)
		if _, err := LoadAccountingCases(bytes.NewReader(bad)); err == nil {
			t.Fatal("bad oracle relationship accepted")
		}
	}
}
