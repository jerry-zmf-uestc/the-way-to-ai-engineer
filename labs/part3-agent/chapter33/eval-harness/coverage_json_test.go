package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCoverageJSONScoresOneVersionedPayload(t *testing.T) {
	cases, labels := coverageFixture()
	f := CoverageFile{Schema: 1, DatasetVersion: "tasks-v1", RequirementVersion: "groups-v1", Cases: cases, Requirements: labels, Runs: []CoverageRun{{CaseID: "conditional", RunID: "r1", CandidatesKnown: true, InputKnown: true, Candidates: []string{"rule-v2", "state-v2"}, Input: []string{"rule-v2"}}}}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	r, err := LoadCoverage(bytes.NewReader(data))
	if err != nil || r.Total != 3 || r.Annotated != 2 || r.CandidatesComplete != 1 || r.InputComplete != 0 || r.MissingRuns != 2 || r.RequirementVersion != "groups-v1" {
		t.Fatal(r, err)
	}
}
func TestCoverageJSONRejectsAmbiguousAndUnboundedInput(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"Schema":1,"schema":1}`), []byte(`{"Unknown":1}`), []byte(`{"Schema":2}`), []byte(`{} {}`), []byte{0xff},
		[]byte(strings.Repeat("[", 10) + "0" + strings.Repeat("]", 10)), bytes.Repeat([]byte(" "), (256<<10)+1),
	} {
		if r, err := LoadCoverage(bytes.NewReader(data)); !errors.Is(err, ErrInvalid) || r.Total != 0 {
			t.Fatal(r, err)
		}
	}
}
