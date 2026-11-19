package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func diagnosisJudgmentFor(t *testing.T, version string, c DiagnosisCase, raw, verdict string) DiagnosisJudgment {
	t.Helper()
	input, err := BuildDiagnosisInput(c)
	if err != nil {
		t.Fatal(err)
	}
	return DiagnosisJudgment{DatasetVersion: version, RubricVersion: "diagnosis-rubric-v1", CaseID: c.ID, InputSHA256: diagnosisSHA(input.Input), AnswerSHA256: diagnosisSHA(raw), Verdict: verdict}
}

func TestDiagnosisEvidenceAndActionDoNotProveCause(t *testing.T) {
	dataset := diagnosisFixture(t)
	c := dataset.Cases[0]
	encoded, _ := json.Marshal(DiagnosisAnswer{Cause: "队列接纳失败，执行器尚未开始。", Action: c.Expected.Action, Refs: c.Expected.Refs})
	raw := string(encoded)
	r, err := ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v1", c, raw, nil)
	if err != nil || r.Status != "review" || r.CauseKnown || !r.ActionCorrect || !r.EvidenceComplete {
		t.Fatal("structural checks manufactured supported cause", r, err)
	}
	for _, verdict := range []string{"supported", "unsupported", "unknown"} {
		label := diagnosisJudgmentFor(t, dataset.Version, c, raw, verdict)
		r, err = ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v1", c, raw, &label)
		wanted := map[string]string{"supported": "passed", "unsupported": "failed", "unknown": "review"}[verdict]
		if err != nil || r.Status != wanted {
			t.Fatal(r, err)
		}
	}
	// Matching an internal reference code is still not an independent semantic judgment.
	encoded, _ = json.Marshal(DiagnosisAnswer{Cause: c.Expected.Cause, Action: c.Expected.Action, Refs: c.Expected.Refs})
	r, err = ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v1", c, string(encoded), nil)
	if err != nil || r.Status != "review" {
		t.Fatal("hidden cause string used as automatic semantic proof", r, err)
	}
}

func TestDiagnosisJudgmentMustMatchInputAnswerAndVersion(t *testing.T) {
	dataset := diagnosisFixture(t)
	c := dataset.Cases[0]
	data, _ := json.Marshal(DiagnosisAnswer{Cause: "capacity rejected", Action: c.Expected.Action, Refs: c.Expected.Refs})
	raw := string(data)
	label := diagnosisJudgmentFor(t, dataset.Version, c, raw, "supported")
	changed := c
	changed.Question += " 请同时分析其他任务。"
	if _, err := ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v1", changed, raw, &label); !errors.Is(err, ErrInvalid) {
		t.Fatal("stale input judgment accepted", err)
	}
	if _, err := ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v1", c, raw+" ", &label); !errors.Is(err, ErrInvalid) {
		t.Fatal("different raw answer used old judgment", err)
	}
	if _, err := ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v2", c, raw, &label); !errors.Is(err, ErrInvalid) {
		t.Fatal("old rubric judgment accepted", err)
	}
}

func TestDiagnosisCauseSupportCannotHideBadActionOrReferences(t *testing.T) {
	dataset := diagnosisFixture(t)
	c := dataset.Cases[0]
	for _, answer := range []DiagnosisAnswer{
		{Cause: "capacity", Action: "report_success", Refs: c.Expected.Refs},
		{Cause: "capacity", Action: c.Expected.Action, Refs: []string{"e1"}},
		{Cause: "capacity", Action: c.Expected.Action, Refs: []string{"unknown"}},
	} {
		encoded, _ := json.Marshal(answer)
		raw := string(encoded)
		label := diagnosisJudgmentFor(t, dataset.Version, c, raw, "supported")
		r, err := ScoreDiagnosis(dataset.Version, "diagnosis-rubric-v1", c, raw, &label)
		if err != nil || r.Status != "failed" {
			t.Fatal("cause score erased another required failure", r, err)
		}
	}
	for _, raw := range []string{
		`{"cause":"a","action":"check","refs":["e1"],"score":1}`,
		`{"cause":"a","cause":"b","action":"check","refs":["e1"]}`,
		`{"cause":"a","action":"check","refs":["e1","e1"]}`,
		`{"cause":"` + strings.Repeat("x", 1025) + `","action":"check","refs":["e1"]}`,
	} {
		if _, err := ParseDiagnosisAnswer(raw); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid answer accepted", err)
		}
	}
}
