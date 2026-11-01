package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type DiagnosisAnswer struct {
	Cause  string   `json:"cause"`
	Action string   `json:"action"`
	Refs   []string `json:"refs"`
}
type DiagnosisJudgment struct {
	DatasetVersion, RubricVersion, CaseID string
	InputSHA256, AnswerSHA256, Verdict    string
}
type DiagnosisScore struct {
	CaseID, Status                                 string
	ActionAllowed, ActionCorrect, EvidenceComplete bool
	CauseKnown, CauseSupported                     bool
}

func diagnosisSHA(text string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(text))) }

func ParseDiagnosisAnswer(raw string) (DiagnosisAnswer, error) {
	if !acceptanceText(raw, 4096) || uniqueCoverageJSON(json.NewDecoder(strings.NewReader(raw)), 0) != nil {
		return DiagnosisAnswer{}, ErrInvalid
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	var data json.RawMessage
	if dec.Decode(&data) != nil || dec.Decode(new(any)) != io.EOF {
		return DiagnosisAnswer{}, ErrInvalid
	}
	object, ok := exactObject(data, "cause", "action", "refs")
	var answer DiagnosisAnswer
	if !ok || json.Unmarshal(object["cause"], &answer.Cause) != nil || json.Unmarshal(object["action"], &answer.Action) != nil || json.Unmarshal(object["refs"], &answer.Refs) != nil || !acceptanceText(answer.Cause, 1024) || !acceptanceText(answer.Action, 128) || len(answer.Refs) == 0 || len(answer.Refs) > 8 {
		return DiagnosisAnswer{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range answer.Refs {
		if !acceptanceText(id, 128) || seen[id] {
			return DiagnosisAnswer{}, ErrInvalid
		}
		seen[id] = true
	}
	return answer, nil
}

func ScoreDiagnosis(datasetVersion, rubricVersion string, c DiagnosisCase, raw string, judgment *DiagnosisJudgment) (DiagnosisScore, error) {
	score := DiagnosisScore{CaseID: c.ID, Status: "failed"}
	if !acceptanceText(datasetVersion, 128) || !acceptanceText(rubricVersion, 128) || validateDiagnosisCase(c) != nil {
		return score, ErrInvalid
	}
	answer, err := ParseDiagnosisAnswer(raw)
	if err != nil {
		return score, err
	}
	input, err := BuildDiagnosisInput(c)
	if err != nil {
		return score, err
	}
	if judgment != nil {
		if judgment.DatasetVersion != datasetVersion || judgment.RubricVersion != rubricVersion || judgment.CaseID != c.ID || judgment.InputSHA256 != diagnosisSHA(input.Input) || judgment.AnswerSHA256 != diagnosisSHA(raw) || judgment.Verdict != "supported" && judgment.Verdict != "unsupported" && judgment.Verdict != "unknown" {
			return score, ErrInvalid
		}
		score.CauseKnown = judgment.Verdict != "unknown"
		score.CauseSupported = judgment.Verdict == "supported"
	}
	for _, allowed := range c.AllowedActions {
		if allowed == answer.Action {
			score.ActionAllowed = true
		}
	}
	score.ActionCorrect = answer.Action == c.Expected.Action
	events, refs := map[string]bool{}, map[string]bool{}
	for _, event := range c.Events {
		events[event.ID] = true
	}
	for _, id := range answer.Refs {
		if !events[id] {
			return score, nil
		}
		refs[id] = true
	}
	score.EvidenceComplete = true
	for _, required := range c.Expected.Refs {
		if !refs[required] {
			score.EvidenceComplete = false
		}
	}
	if !score.ActionAllowed || !score.ActionCorrect || !score.EvidenceComplete {
		return score, nil
	}
	if !score.CauseKnown {
		score.Status = "review"
	} else if score.CauseSupported {
		score.Status = "passed"
	}
	return score, nil
}
