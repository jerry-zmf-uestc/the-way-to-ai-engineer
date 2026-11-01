package main

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

type DiagnosisEvent struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type DiagnosisCase struct {
	ID             string           `json:"id"`
	Split          string           `json:"split"`
	Scenario       string           `json:"scenario"`
	Question       string           `json:"question"`
	Events         []DiagnosisEvent `json:"events"`
	AllowedActions []string         `json:"allowed_actions"`
	Expected       struct {
		Cause  string   `json:"cause"`
		Action string   `json:"action"`
		Refs   []string `json:"refs"`
	} `json:"expected"`
	Source struct {
		Path     string `json:"path"`
		Contract string `json:"contract"`
	} `json:"source"`
}
type DiagnosisDataset struct {
	Version    string          `json:"version"`
	Family     string          `json:"family"`
	Kind       string          `json:"kind"`
	Split      string          `json:"split"`
	Provenance string          `json:"provenance"`
	Cases      []DiagnosisCase `json:"cases"`
}

func LoadDiagnosisCases(reader io.Reader) (DiagnosisDataset, error) {
	data, err := io.ReadAll(io.LimitReader(reader, (64<<10)+1))
	if err != nil || len(data) > 64<<10 || !utf8.Valid(data) || uniqueCoverageJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return DiagnosisDataset{}, ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var dataset DiagnosisDataset
	if dec.Decode(&dataset) != nil || dec.Decode(new(any)) != io.EOF || !acceptanceText(dataset.Version, 128) || dataset.Family != "runtime-diagnosis" || dataset.Kind != "model-input" || dataset.Split != "development" || !acceptanceText(dataset.Provenance, 1024) || len(dataset.Cases) == 0 || len(dataset.Cases) > 32 {
		return DiagnosisDataset{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, c := range dataset.Cases {
		if seen[c.ID] || c.Split != dataset.Split {
			return DiagnosisDataset{}, ErrInvalid
		}
		if err := validateDiagnosisCase(c); err != nil {
			return DiagnosisDataset{}, err
		}
		seen[c.ID] = true
	}
	return dataset, nil
}

func BuildDiagnosisInput(c DiagnosisCase) (TaskInput, error) {
	if !acceptanceText(c.ID, 128) || !acceptanceText(c.Question, 2048) || len(c.Events) == 0 || len(c.Events) > 8 || len(c.AllowedActions) == 0 || len(c.AllowedActions) > 8 {
		return TaskInput{}, ErrInvalid
	}
	ids, actions := map[string]bool{}, map[string]bool{}
	sources := []string{}
	for _, event := range c.Events {
		if !acceptanceText(event.ID, 128) || ids[event.ID] || !acceptanceText(event.Text, 1024) {
			return TaskInput{}, ErrInvalid
		}
		ids[event.ID] = true
		sources = append(sources, event.ID)
	}
	for _, action := range c.AllowedActions {
		if !acceptanceText(action, 128) || actions[action] {
			return TaskInput{}, ErrInvalid
		}
		actions[action] = true
	}
	data, err := json.Marshal(struct {
		Question       string           `json:"question"`
		Events         []DiagnosisEvent `json:"events"`
		AllowedActions []string         `json:"allowed_actions"`
	}{c.Question, c.Events, c.AllowedActions})
	if err != nil || len(data) > 4096 {
		return TaskInput{}, ErrInvalid
	}
	return TaskInput{ID: c.ID, Input: string(data), AllowedSources: sources}, nil
}

func validateDiagnosisCase(c DiagnosisCase) error {
	if c.Split != "development" || !acceptanceText(c.Scenario, 128) || !acceptanceText(c.Source.Path, 256) || !acceptanceText(c.Source.Contract, 1024) || !acceptanceText(c.Expected.Cause, 128) {
		return ErrInvalid
	}
	if _, err := BuildDiagnosisInput(c); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, action := range c.AllowedActions {
		allowed[action] = true
	}
	if !allowed[c.Expected.Action] || len(c.Expected.Refs) == 0 || len(c.Expected.Refs) > len(c.Events) {
		return ErrInvalid
	}
	events, refs := map[string]bool{}, map[string]bool{}
	for _, event := range c.Events {
		events[event.ID] = true
	}
	for _, id := range c.Expected.Refs {
		if !events[id] || refs[id] {
			return ErrInvalid
		}
		refs[id] = true
	}
	return nil
}
