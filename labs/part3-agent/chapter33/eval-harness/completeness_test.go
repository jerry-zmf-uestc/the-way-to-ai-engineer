package main

import "testing"

func TestFullySupportedPartialAnswerStillFailsTask(t *testing.T) {
	// Synthetic labels illustrate scorer contracts, not human semantic calibration.
	conditional := "Harbor timeout is 43 seconds when disabled."
	current := "Harbor current timeout is 43 seconds."
	c := Case{ID: "current-timeout", Split: "regression", Input: "What is Harbor current timeout?",
		AllowedSources: []string{"combined-config"}, RequiredClaims: []string{current}}
	labels := []Label{
		{CaseID: c.ID, Text: conditional, Source: "combined-config", Verdict: "supported"},
		{CaseID: c.ID, Text: current, Source: "combined-config", Verdict: "supported"},
	}
	r := Result{CaseID: c.ID, RunID: "partial", Evidence: []string{"combined-config"},
		Claims: []Claim{{Text: conditional, Sources: []string{"combined-config"}}}}
	report, err := Evaluate("completion-contract-v1", "partial", []Case{c}, []Result{r}, labels)
	if err != nil || report.Passed != 0 || report.Failed != 1 || report.SupportRate != 1 || report.SupportCoverage != 1 || report.Rows[0].Status != "failed" {
		t.Fatal("support hid missing required conclusion", report, err)
	}
	r.RunID = "complete"
	r.Claims = append(r.Claims, Claim{Text: current, Sources: []string{"combined-config"}})
	report, err = Evaluate("completion-contract-v1", "complete", []Case{c}, []Result{r}, labels)
	if err != nil || report.Passed != 1 || report.Failed != 0 {
		t.Fatal("required supported conclusion rejected", report, err)
	}
}
