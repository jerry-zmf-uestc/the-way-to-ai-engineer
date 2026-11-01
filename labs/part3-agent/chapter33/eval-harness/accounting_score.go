package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

type AccountingAnswer struct {
	Known bool
	Value *int64
	Unit  string
	Refs  []string
}
type AccountingScore struct {
	CaseID, Status                                     string
	KnownCorrect, ValueCorrect, UnitCorrect, RefsValid bool
}

func ParseAccountingAnswer(raw string) (AccountingAnswer, error) {
	if !acceptanceText(raw, 4096) || uniqueCoverageJSON(json.NewDecoder(strings.NewReader(raw)), 0) != nil {
		return AccountingAnswer{}, ErrInvalid
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	var data json.RawMessage
	if dec.Decode(&data) != nil || dec.Decode(new(any)) != io.EOF {
		return AccountingAnswer{}, ErrInvalid
	}
	fields, ok := exactObject(data, "known", "value", "unit", "refs")
	var answer AccountingAnswer
	if !ok {
		return answer, ErrInvalid
	}
	known := bytes.TrimSpace(fields["known"])
	if !bytes.Equal(known, []byte("true")) && !bytes.Equal(known, []byte("false")) {
		return answer, ErrInvalid
	}
	if json.Unmarshal(fields["known"], &answer.Known) != nil || json.Unmarshal(fields["value"], &answer.Value) != nil || json.Unmarshal(fields["unit"], &answer.Unit) != nil || json.Unmarshal(fields["refs"], &answer.Refs) != nil || !acceptanceText(answer.Unit, 32) || len(answer.Refs) == 0 || len(answer.Refs) > 8 {
		return AccountingAnswer{}, ErrInvalid
	}
	if answer.Known && (answer.Value == nil || *answer.Value < 0 || *answer.Value > 4_000_000_000_000) || !answer.Known && answer.Value != nil {
		return AccountingAnswer{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range answer.Refs {
		if !acceptanceText(id, 128) || seen[id] {
			return AccountingAnswer{}, ErrInvalid
		}
		seen[id] = true
	}
	return answer, nil
}

func ScoreAccounting(c AccountingCase, raw string) (AccountingScore, error) {
	score := AccountingScore{CaseID: c.ID, Status: "failed"}
	if _, err := BuildAccountingInput(c); err != nil {
		return score, err
	}
	reference, err := AccountingOracle(c)
	if err != nil {
		return score, err
	}
	answer, err := ParseAccountingAnswer(raw)
	if err != nil {
		return score, err
	}
	score.KnownCorrect = answer.Known == reference.Known
	score.ValueCorrect = answer.Value == nil && reference.Value == nil || answer.Value != nil && reference.Value != nil && *answer.Value == *reference.Value
	score.UnitCorrect = answer.Unit == reference.Unit
	allowed := map[string]bool{}
	for _, fact := range c.Facts {
		allowed[fact.ID] = true
	}
	score.RefsValid = true
	for _, id := range answer.Refs {
		if !allowed[id] {
			score.RefsValid = false
		}
	}
	if score.KnownCorrect && score.ValueCorrect && score.UnitCorrect && score.RefsValid {
		score.Status = "numeric_contract_passed"
	}
	return score, nil
}
