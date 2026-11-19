package main

import (
	"errors"
	"testing"
)

func TestCalibrationPreservesUnknownAndHighRiskFailures(t *testing.T) {
	r, err := Calibrate("d", "r", "rubric", "judge", calibrationDemo())
	if err != nil {
		t.Fatal(err)
	}
	c := r.Total
	if c.Total != 6 || c.ReferenceKnown != 5 || c.ReferenceUnknown != 1 || c.JudgedKnown != 4 || c.Agree != 3 || c.FalseSupport != 1 || c.FalseReject != 0 {
		t.Fatal(c)
	}
	rates := c.Rates()
	if rates.Agreement != (Ratio{.6, true}) || rates.Coverage != (Ratio{.8, true}) || rates.FalseSupport != (Ratio{.5, true}) || rates.FalseReject != (Ratio{0, true}) {
		t.Fatal(rates)
	}
	if len(r.Groups) != 3 || r.Groups[0].Stratum != "ambiguous" || r.Groups[2].Stratum != "risk" || r.Groups[2].Counts.Rates().FalseSupport != (Ratio{.5, true}) {
		t.Fatal(r.Groups)
	}
	if r.Groups[0].Counts.Rates().Agreement.Known || r.Groups[0].Counts.Rates().Coverage.Known || r.Groups[1].Counts.Rates().FalseSupport.Known {
		t.Fatal("missing denominator treated as zero")
	}
}

func TestCalibrationRejectsInvalidOrRepeatedAssessments(t *testing.T) {
	for _, rows := range [][]Assessment{nil, {{"id", "risk", "unsupported", "invalid"}}, {{"id", "risk", "unsupported", "supported"}, {"id", "normal", "supported", "supported"}}} {
		if _, err := Calibrate("d", "r", "rubric", "judge", rows); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := Calibrate("d", "", "rubric", "judge", calibrationDemo()); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestApprovedProductionLikeSplitIsAcceptedWithoutInventingProductionEvidence(t *testing.T) {
	if _, err := caseIDs([]Case{{ID: "synthetic-capacity", Split: "production-like", Input: "fixed synthetic task"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := caseIDs([]Case{{ID: "invalid-split", Split: "anything", Input: "fixed synthetic task"}}); !errors.Is(err, ErrInvalid) {
		t.Fatal("unregistered split accepted", err)
	}
}
