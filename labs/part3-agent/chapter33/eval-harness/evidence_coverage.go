package main

import "unicode/utf8"

type EvidenceGroup struct {
	ID           string
	Alternatives []string
}
type EvidenceRequirement struct {
	CaseID string
	Groups []EvidenceGroup
}
type CoverageRun struct {
	CaseID, RunID               string
	CandidatesKnown, InputKnown bool
	Candidates, Input           []string
}
type CoverageRow struct {
	CaseID, RunID                              string
	Annotated, Recorded                        bool
	CandidatesKnown, InputKnown                bool
	CandidatesComplete, InputComplete          bool
	MissingCandidateGroups, MissingInputGroups []string
}
type CoverageReport struct {
	DatasetVersion, RequirementVersion         string
	Total, Annotated, Unannotated, MissingRuns int
	CandidatesObserved, InputObserved          int
	CandidatesComplete, InputComplete          int
	Rows                                       []CoverageRow
}

func coverageID(id string) bool { return id != "" && len(id) <= 128 && utf8.ValidString(id) }
func coverageSources(ids []string, allowed map[string]bool) (map[string]bool, error) {
	if len(ids) > 64 {
		return nil, ErrInvalid
	}
	set := map[string]bool{}
	for _, id := range ids {
		if !coverageID(id) || !allowed[id] || set[id] {
			return nil, ErrInvalid
		}
		set[id] = true
	}
	return set, nil
}
func missingEvidenceGroups(groups []EvidenceGroup, ids map[string]bool) []string {
	missing := []string{}
	for _, group := range groups {
		covered := false
		for _, id := range group.Alternatives {
			covered = covered || ids[id]
		}
		if !covered {
			missing = append(missing, group.ID)
		}
	}
	return missing
}

// Requirements are independent scoring labels; this function never supplies model input.
func ScoreEvidenceCoverage(dataset, requirementVersion string, cases []Case, requirements []EvidenceRequirement, runs []CoverageRun) (CoverageReport, error) {
	if !coverageID(dataset) || !coverageID(requirementVersion) {
		return CoverageReport{}, ErrInvalid
	}
	ids, err := caseIDs(cases)
	if err != nil || len(requirements) > len(cases) || len(runs) > len(cases) {
		return CoverageReport{}, ErrInvalid
	}
	allowed := map[string]map[string]bool{}
	for _, c := range cases {
		if !coverageID(c.ID) {
			return CoverageReport{}, ErrInvalid
		}
		set := map[string]bool{}
		if len(c.AllowedSources) > 64 {
			return CoverageReport{}, ErrInvalid
		}
		for _, id := range c.AllowedSources {
			if !coverageID(id) || set[id] {
				return CoverageReport{}, ErrInvalid
			}
			set[id] = true
		}
		allowed[c.ID] = set
	}
	labels := map[string][]EvidenceGroup{}
	for _, r := range requirements {
		if !ids[r.CaseID] || labels[r.CaseID] != nil || len(r.Groups) < 1 || len(r.Groups) > 8 {
			return CoverageReport{}, ErrInvalid
		}
		groups := map[string]bool{}
		for _, group := range r.Groups {
			if !coverageID(group.ID) || groups[group.ID] || len(group.Alternatives) < 1 || len(group.Alternatives) > 8 {
				return CoverageReport{}, ErrInvalid
			}
			groups[group.ID] = true
			if _, err := coverageSources(group.Alternatives, allowed[r.CaseID]); err != nil {
				return CoverageReport{}, err
			}
		}
		labels[r.CaseID] = r.Groups
	}
	byCase := map[string]CoverageRun{}
	runIDs := map[string]bool{}
	for _, run := range runs {
		if !ids[run.CaseID] || byCase[run.CaseID].RunID != "" || !coverageID(run.RunID) || runIDs[run.RunID] || !run.CandidatesKnown && len(run.Candidates) != 0 || !run.InputKnown && len(run.Input) != 0 {
			return CoverageReport{}, ErrInvalid
		}
		candidates, err := coverageSources(run.Candidates, allowed[run.CaseID])
		if err != nil {
			return CoverageReport{}, err
		}
		input, err := coverageSources(run.Input, allowed[run.CaseID])
		if err != nil {
			return CoverageReport{}, err
		}
		if run.CandidatesKnown && run.InputKnown {
			for id := range input {
				if !candidates[id] {
					return CoverageReport{}, ErrInvalid
				}
			}
		}
		byCase[run.CaseID] = run
		runIDs[run.RunID] = true
	}
	report := CoverageReport{DatasetVersion: dataset, RequirementVersion: requirementVersion, Total: len(cases)}
	for _, c := range cases {
		run, recorded := byCase[c.ID]
		row := CoverageRow{CaseID: c.ID, RunID: run.RunID, Recorded: recorded, Annotated: labels[c.ID] != nil, CandidatesKnown: run.CandidatesKnown, InputKnown: run.InputKnown}
		if !recorded {
			report.MissingRuns++
		}
		if !row.Annotated {
			report.Unannotated++
		} else {
			report.Annotated++
			row.CandidatesKnown, row.InputKnown = run.CandidatesKnown, run.InputKnown
			if recorded && run.CandidatesKnown {
				report.CandidatesObserved++
				set, _ := coverageSources(run.Candidates, allowed[c.ID])
				row.MissingCandidateGroups = missingEvidenceGroups(labels[c.ID], set)
				row.CandidatesComplete = len(row.MissingCandidateGroups) == 0
				if row.CandidatesComplete {
					report.CandidatesComplete++
				}
			}
			if recorded && run.InputKnown {
				report.InputObserved++
				set, _ := coverageSources(run.Input, allowed[c.ID])
				row.MissingInputGroups = missingEvidenceGroups(labels[c.ID], set)
				row.InputComplete = len(row.MissingInputGroups) == 0
				if row.InputComplete {
					report.InputComplete++
				}
			}
		}
		report.Rows = append(report.Rows, row)
	}
	return report, nil
}
