package main

import (
	"errors"
	"testing"
)

func coverageFixture() ([]Case, []EvidenceRequirement) {
	cases := []Case{
		{ID: "conditional", Split: "development", Input: "What is the current timeout?", AllowedSources: []string{"rule-v2", "rule-copy-v2", "rule-equivalent-v2", "state-v2"}},
		{ID: "unlabeled", Split: "regression", Input: "Other task", AllowedSources: []string{"rule-v2"}},
		{ID: "missing", Split: "adversarial", Input: "Missing run", AllowedSources: []string{"state-v2"}},
	}
	labels := []EvidenceRequirement{
		{CaseID: "conditional", Groups: []EvidenceGroup{{ID: "rule", Alternatives: []string{"rule-v2", "rule-equivalent-v2"}}, {ID: "state", Alternatives: []string{"state-v2"}}}},
		{CaseID: "missing", Groups: []EvidenceGroup{{ID: "state", Alternatives: []string{"state-v2"}}}},
	}
	return cases, labels
}

func TestEvidenceGroupsDistinguishCandidateAndInputLoss(t *testing.T) {
	cases, labels := coverageFixture()
	for _, item := range []struct {
		candidates, input                []string
		candidateComplete, inputComplete bool
		missingCandidate, missingInput   string
	}{
		{[]string{"rule-v2", "rule-copy-v2"}, []string{"rule-v2", "rule-copy-v2"}, false, false, "state", "state"},
		{[]string{"rule-v2", "state-v2"}, []string{"rule-v2"}, true, false, "", "state"},
		{[]string{"rule-equivalent-v2", "state-v2"}, []string{"rule-equivalent-v2", "state-v2"}, true, true, "", ""},
	} {
		report, err := ScoreEvidenceCoverage("tasks-v1", "groups-v1", cases, labels, []CoverageRun{{CaseID: "conditional", RunID: "run1", CandidatesKnown: true, InputKnown: true, Candidates: item.candidates, Input: item.input}})
		if err != nil || report.Total != 3 || report.Annotated != 2 || report.Unannotated != 1 || report.MissingRuns != 2 || report.CandidatesObserved != 1 || report.InputObserved != 1 {
			t.Fatal(report, err)
		}
		row := report.Rows[0]
		if row.CandidatesComplete != item.candidateComplete || row.InputComplete != item.inputComplete {
			t.Fatal(row)
		}
		if item.missingCandidate != "" && (len(row.MissingCandidateGroups) != 1 || row.MissingCandidateGroups[0] != item.missingCandidate) {
			t.Fatal(row)
		}
		if item.missingInput != "" && (len(row.MissingInputGroups) != 1 || row.MissingInputGroups[0] != item.missingInput) {
			t.Fatal(row)
		}
	}
}

func TestEvidenceGroupsUnknownAndObservedEmptyAreDifferent(t *testing.T) {
	cases, labels := coverageFixture()
	report, err := ScoreEvidenceCoverage("tasks-v1", "groups-v1", cases, labels, []CoverageRun{{CaseID: "conditional", RunID: "r1", CandidatesKnown: true}})
	if err != nil {
		t.Fatal(err)
	}
	row := report.Rows[0]
	if !row.CandidatesKnown || row.InputKnown || len(row.MissingCandidateGroups) != 2 || row.MissingInputGroups != nil || report.CandidatesComplete != 0 || report.InputObserved != 0 {
		t.Fatal(report)
	}
	if report.Rows[1].Annotated || report.Rows[2].Recorded {
		t.Fatal(report)
	}
}

func TestEvidenceGroupsRejectIdentitySourcesAndLabelConflicts(t *testing.T) {
	for _, mutate := range []func(*[]EvidenceRequirement, *[]CoverageRun){
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { *l = append(*l, (*l)[0]) },
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { (*l)[0].Groups[1].ID = "rule" },
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { (*l)[0].Groups[0].Alternatives = []string{"foreign"} },
		func(l *[]EvidenceRequirement, r *[]CoverageRun) {
			(*l)[0].Groups[0].Alternatives = []string{"rule-v2", "rule-v2"}
		},
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { (*l)[0].CaseID = "unknown" },
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { *r = append(*r, (*r)[0]) },
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { (*r)[0].Candidates = []string{"foreign"} },
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { (*r)[0].Input = []string{"state-v2"} },
		func(l *[]EvidenceRequirement, r *[]CoverageRun) { (*r)[0].CandidatesKnown = false },
	} {
		cases, labels := coverageFixture()
		runs := []CoverageRun{{CaseID: "conditional", RunID: "run1", CandidatesKnown: true, InputKnown: true, Candidates: []string{"rule-v2"}, Input: []string{"rule-v2"}}}
		mutate(&labels, &runs)
		if report, err := ScoreEvidenceCoverage("tasks-v1", "groups-v1", cases, labels, runs); !errors.Is(err, ErrInvalid) || report.Total != 0 {
			t.Fatal(report, err)
		}
	}
}
