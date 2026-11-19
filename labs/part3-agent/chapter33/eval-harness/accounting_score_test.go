package main

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAccountingContractChecksUnknownValueUnitsAndScope(t *testing.T) {
	dataset := accountingFixture(t)
	for _, c := range dataset.Cases {
		reference, err := AccountingOracle(c)
		if err != nil {
			t.Fatal(err)
		}
		refs := []string{}
		for _, fact := range c.Facts {
			refs = append(refs, fact.ID)
		}
		data, _ := json.Marshal(map[string]any{"known": reference.Known, "value": reference.Value, "unit": reference.Unit, "refs": refs})
		score, err := ScoreAccounting(c, string(data))
		if err != nil || score.Status != "numeric_contract_passed" {
			t.Fatal(c.ID, score, err)
		}
	}
	c := dataset.Cases[0]
	for _, raw := range []string{
		"{\"known\":true,\"value\":51,\"unit\":\"microcredit\",\"refs\":[\"limit\"]}",
		"{\"known\":true,\"value\":50,\"unit\":\"USD\",\"refs\":[\"limit\"]}",
		"{\"known\":true,\"value\":50,\"unit\":\"microcredit\",\"refs\":[\"unknown\"]}",
	} {
		score, err := ScoreAccounting(c, raw)
		if err != nil || score.Status != "failed" {
			t.Fatal(score, err)
		}
	}
}

func TestAccountingParserDoesNotTurnUnknownIntoZero(t *testing.T) {
	for _, raw := range []string{
		"{\"known\":false,\"value\":0,\"unit\":\"token\",\"refs\":[\"bytes\"]}",
		"{\"known\":true,\"value\":null,\"unit\":\"token\",\"refs\":[\"bytes\"]}",
		"{\"known\":null,\"value\":null,\"unit\":\"token\",\"refs\":[\"bytes\"]}",
		"{\"known\":true,\"value\":1.5,\"unit\":\"token\",\"refs\":[\"bytes\"]}",
		"{\"known\":true,\"known\":false,\"value\":null,\"unit\":\"token\",\"refs\":[\"bytes\"]}",
		"{\"known\":true,\"value\":0,\"unit\":\"token\",\"refs\":[\"bytes\"],\"score\":1}",
	} {
		if _, err := ParseAccountingAnswer(raw); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid contract accepted", err)
		}
	}
	zero := "{\"known\":true,\"value\":0,\"unit\":\"microcredit\",\"refs\":[\"handler_calls\"]}"
	unknown := "{\"known\":false,\"value\":null,\"unit\":\"microcredit\",\"refs\":[\"reserved\"]}"
	dataset := accountingFixture(t)
	if score, err := ScoreAccounting(dataset.Cases[3], zero); err != nil || score.Status != "numeric_contract_passed" {
		t.Fatal("confirmed zero lost", score, err)
	}
	if score, err := ScoreAccounting(dataset.Cases[1], unknown); err != nil || score.Status != "numeric_contract_passed" {
		t.Fatal("legitimate unknown lost", score, err)
	}
}
