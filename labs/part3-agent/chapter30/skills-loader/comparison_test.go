package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestComparisonUsesActualLoadsWithoutScoringLabels(t *testing.T) {
	l, _ := loader(t, 1024)
	var output bytes.Buffer
	if err := exportComparisons(context.Background(), l.root, &output, "development-v1"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"required_numbers", "expected_missing", "expected_selected"} {
		if strings.Contains(output.String(), label) {
			t.Fatal("scoring label exported", label)
		}
	}
	var result struct {
		Version string            `json:"version"`
		Cases   []ComparisonInput `json:"cases"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Version != "skills-development-v1" || len(result.Cases) != 6 {
		t.Fatal(err)
	}
	for _, row := range result.Cases {
		full, selected := row.Modes[0], row.Modes[1]
		if full.Name != "full" || selected.Name != "selected" || !reflect.DeepEqual(full.Catalog, selected.Catalog) || len(full.Skills) != 2 || full.UsedBytes < selected.UsedBytes {
			t.Fatal("unpaired comparison", row.ID)
		}
		for _, mode := range row.Modes {
			used := 0
			for _, s := range mode.Skills {
				if s.Version != "v1" || s.Digest != hash([]byte(s.Text)) {
					t.Fatal("not actual loaded content")
				}
				used += len(s.Text)
			}
			for _, r := range mode.Resources {
				used += len(r.Text)
			}
			if used != mode.UsedBytes {
				t.Fatal("loading accounting", row.ID)
			}
		}
		if row.ID == "no-skill" && (len(selected.Skills) != 0 || selected.UsedBytes != 0) {
			t.Fatal("unrelated skills loaded")
		}
	}
}

func TestComparisonDatasetSelectionIsClosedAndFresh(t *testing.T) {
	l, _ := loader(t, 1024)
	var output bytes.Buffer
	if err := exportComparisons(context.Background(), l.root, &output, "../../other"); err == nil || output.Len() != 0 {
		t.Fatal("unregistered task path accepted")
	}
	if err := exportComparisons(context.Background(), l.root, &output, "holdout-v1"); err != nil {
		t.Fatal(err)
	}
	var data struct {
		Version string            `json:"version"`
		Cases   []ComparisonInput `json:"cases"`
	}
	if err := json.Unmarshal(output.Bytes(), &data); err != nil || data.Version != "skills-holdout-v1" || len(data.Cases) != 6 {
		t.Fatal("holdout identity", err)
	}
	if len(data.Cases[0].Evidence) != 2 || len(data.Cases[1].Evidence) != 2 || data.Cases[0].ID == "diagnosis" {
		t.Fatal("fresh multi-source tasks missing")
	}
	for _, label := range []string{"required_numbers", "expected_selected", "expected_missing"} {
		if strings.Contains(output.String(), label) {
			t.Fatal("holdout label exported")
		}
	}
}
