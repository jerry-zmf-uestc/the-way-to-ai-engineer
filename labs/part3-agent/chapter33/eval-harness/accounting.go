package main

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

type AccountingFact struct {
	ID    string
	Value int64
}
type AccountingCase struct {
	ID, Split, Scenario, Question string
	Facts                         []AccountingFact
	Expected                      struct {
		Rule  string
		Known bool
		Value *int64
		Unit  string
	}
	Source struct{ Path, Contract string }
}
type AccountingDataset struct {
	Version, Family, Kind, Split, Provenance string
	Cases                                    []AccountingCase
}
type AccountingReference struct {
	Known bool
	Value *int64
	Unit  string
}

func AccountingOracle(c AccountingCase) (AccountingReference, error) {
	values := map[string]int64{}
	for _, fact := range c.Facts {
		if !acceptanceText(fact.ID, 128) || fact.Value < 0 || fact.Value > 1_000_000 {
			return AccountingReference{}, ErrInvalid
		}
		if _, exists := values[fact.ID]; exists {
			return AccountingReference{}, ErrInvalid
		}
		values[fact.ID] = fact.Value
	}
	keys := map[string][]string{
		"available_budget":   {"limit", "used", "held"},
		"unknown_cost":       {"reserved"},
		"sum_attempts":       {"failed_one", "failed_two", "success"},
		"call_product":       {"handler_calls", "rate_per_call"},
		"shared_total":       {"ledger_used", "child_a_included", "child_b_included"},
		"usage_product":      {"input", "cache_read", "cache_write", "output", "input_rate", "cache_read_rate", "cache_write_rate", "output_rate"},
		"output_headroom":    {"window", "input", "margin"},
		"active_capacity":    {"limit", "running", "canceling_not_exited"},
		"all_or_none":        {"queue_limit", "queued", "batch"},
		"remaining_attempts": {"limit", "restored_used"},
		"critical_path":      {"catalog_ms", "query_a_ms", "query_b_ms", "summary_ms"},
		"unknown_tokens":     {"utf8_bytes"},
	}
	required, exists := keys[c.Expected.Rule]
	if !exists || len(values) != len(required) {
		return AccountingReference{}, ErrInvalid
	}
	for _, key := range required {
		if _, ok := values[key]; !ok {
			return AccountingReference{}, ErrInvalid
		}
	}
	value := int64(0)
	unit := "microcredit"
	known := true
	switch c.Expected.Rule {
	case "available_budget":
		value = values["limit"] - values["used"] - values["held"]
	case "unknown_cost":
		known = false
	case "sum_attempts":
		value = values["failed_one"] + values["failed_two"] + values["success"]
	case "call_product":
		value = values["handler_calls"] * values["rate_per_call"]
	case "shared_total":
		value = values["ledger_used"]
		if values["child_a_included"]+values["child_b_included"] > value {
			return AccountingReference{}, ErrInvalid
		}
	case "usage_product":
		value = values["input"]*values["input_rate"] + values["cache_read"]*values["cache_read_rate"] + values["cache_write"]*values["cache_write_rate"] + values["output"]*values["output_rate"]
	case "output_headroom":
		unit = "token"
		value = values["window"] - values["input"] - values["margin"]
	case "active_capacity":
		unit = "slot"
		value = values["limit"] - values["running"] - values["canceling_not_exited"]
	case "all_or_none":
		unit = "task"
		if values["queued"] > values["queue_limit"] {
			return AccountingReference{}, ErrInvalid
		}
		if values["batch"] <= values["queue_limit"]-values["queued"] {
			value = values["batch"]
		}
	case "remaining_attempts":
		unit = "attempt"
		value = values["limit"] - values["restored_used"]
	case "critical_path":
		unit = "millisecond"
		value = values["catalog_ms"] + max(values["query_a_ms"], values["query_b_ms"]) + values["summary_ms"]
	case "unknown_tokens":
		unit = "token"
		known = false
	}
	if value < 0 {
		return AccountingReference{}, ErrInvalid
	}
	result := AccountingReference{Known: known, Unit: unit}
	if known {
		result.Value = &value
	}
	return result, nil
}

func BuildAccountingInput(c AccountingCase) (TaskInput, error) {
	if !acceptanceText(c.ID, 128) || !acceptanceText(c.Question, 2048) || len(c.Facts) == 0 || len(c.Facts) > 8 {
		return TaskInput{}, ErrInvalid
	}
	seen := map[string]bool{}
	sources := []string{}
	for _, fact := range c.Facts {
		if !acceptanceText(fact.ID, 128) || seen[fact.ID] || fact.Value < 0 || fact.Value > 1_000_000 {
			return TaskInput{}, ErrInvalid
		}
		seen[fact.ID] = true
		sources = append(sources, fact.ID)
	}
	data, err := json.Marshal(map[string]any{"question": c.Question, "facts": c.Facts})
	if err != nil || len(data) > 4096 {
		return TaskInput{}, ErrInvalid
	}
	return TaskInput{ID: c.ID, Input: string(data), AllowedSources: sources}, nil
}

func LoadAccountingCases(reader io.Reader) (AccountingDataset, error) {
	data, err := io.ReadAll(io.LimitReader(reader, (64<<10)+1))
	if err != nil || len(data) > 64<<10 || !utf8.Valid(data) || uniqueCoverageJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return AccountingDataset{}, ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var dataset AccountingDataset
	if dec.Decode(&dataset) != nil || dec.Decode(new(any)) != io.EOF || dataset.Version != "resource-accounting-development-v1" || dataset.Family != "resource-accounting" || dataset.Kind != "model-input" || dataset.Split != "development" || !acceptanceText(dataset.Provenance, 1024) || len(dataset.Cases) == 0 || len(dataset.Cases) > 32 {
		return AccountingDataset{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, c := range dataset.Cases {
		if seen[c.ID] || c.Split != "development" || !acceptanceText(c.Scenario, 128) || !acceptanceText(c.Source.Path, 256) || !acceptanceText(c.Source.Contract, 1024) {
			return AccountingDataset{}, ErrInvalid
		}
		if _, err := BuildAccountingInput(c); err != nil {
			return AccountingDataset{}, err
		}
		oracle, err := AccountingOracle(c)
		if err != nil || oracle.Known != c.Expected.Known || oracle.Unit != c.Expected.Unit || oracle.Value == nil && c.Expected.Value != nil || oracle.Value != nil && (c.Expected.Value == nil || *oracle.Value != *c.Expected.Value) {
			return AccountingDataset{}, ErrInvalid
		}
		seen[c.ID] = true
	}
	return dataset, nil
}
