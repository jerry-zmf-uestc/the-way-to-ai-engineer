package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"
)

type LocalGeneration struct {
	Mode                      string
	Raw                       string
	InputTokens, OutputTokens int
	AtOutputLimit             bool
	Millis                    int
	ForwardTokens             int
	Candidates                []CandidateScore
}
type CandidateScore struct {
	Raw                       string
	MeanLogProb               float64
	InputTokens, ChoiceTokens int
}
type LocalRow struct {
	CaseID, Strategy, Error string
	Report                  Report
	Generations             []LocalGeneration
}
type LocalResult struct {
	Version, ModelRevision, PromptVersion string
	Rows                                  []LocalRow
}
type localPayload struct {
	Version       string
	PromptVersion string
	Cases         []Input
	Corpus        map[string]Document
	Limits        Limits
}

func localInput(cases []Case, docs map[string]Document, limits Limits) ([]byte, error) {
	if len(cases) != 6 || len(docs) > 32 || limits.Reads != 8 || limits.Decisions != 16 {
		return nil, ErrInput
	}
	p := localPayload{Version: "local-strategy-v1", PromptVersion: "strategy-decisions-v1", Corpus: docs, Limits: limits}
	for _, c := range cases {
		p.Cases = append(p.Cases, c.Input)
	}
	data, err := json.Marshal(p)
	if err != nil || len(data) > 128<<10 {
		return nil, ErrInput
	}
	return data, nil
}

type localBuffer struct {
	bytes.Buffer
	limit int
}

func (b *localBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}
func RunLocalStrategies(ctx context.Context, python, assets string, cases []Case, docs map[string]Document, limits Limits) (LocalResult, error) {
	return RunLocalStrategyVariant(ctx, python, assets, cases, docs, limits, "strategy-decisions-v1")
}
func RunLocalStrategyVariant(ctx context.Context, python, assets string, cases []Case, docs map[string]Document, limits Limits, prompt string) (LocalResult, error) {
	if prompt != "strategy-decisions-v1" && prompt != "strategy-decisions-v2" && prompt != "strategy-decisions-v3" && prompt != "strategy-decisions-v4" {
		return LocalResult{}, ErrInput
	}
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) {
		return LocalResult{}, ErrInput
	}
	data, err := localInput(cases, docs, limits)
	if err != nil {
		return LocalResult{}, err
	}
	var payload localPayload
	if json.Unmarshal(data, &payload) != nil {
		return LocalResult{}, ErrInput
	}
	payload.PromptVersion = prompt
	data, err = json.Marshal(payload)
	if err != nil {
		return LocalResult{}, err
	}
	_, file, _, _ := runtime.Caller(0)
	child, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	cmd := exec.CommandContext(child, python, filepath.Join(filepath.Dir(file), "local_strategies.py"), assets)
	cmd.Stdin = bytes.NewReader(data)
	stdout, stderr := &localBuffer{limit: 256 << 10}, &localBuffer{limit: 4096}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err = cmd.Run(); err != nil {
		if child.Err() != nil {
			return LocalResult{}, child.Err()
		}
		return LocalResult{}, errors.New("local strategy process failed")
	}
	var result LocalResult
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	dec.DisallowUnknownFields()
	if dec.Decode(&result) != nil {
		return LocalResult{}, ErrInput
	}
	var tail any
	expectedRows := 12
	if prompt == "strategy-decisions-v4" {
		expectedRows = 18
	}
	if dec.Decode(&tail) != io.EOF || result.Version != "local-strategy-v1" || result.ModelRevision != "c1899de289a04d12100db370d81485cdf75e47ca" || result.PromptVersion != prompt || len(result.Rows) != expectedRows {
		return LocalResult{}, ErrInput
	}
	for _, row := range result.Rows {
		if len(row.Generations) > 16 || row.Report.Reads > 8 || row.Report.Decisions > 16 || row.Report.Repairs < 0 || row.Report.Repairs > 1 {
			return LocalResult{}, ErrInput
		}
		for _, g := range row.Generations {
			if len(g.Raw) > 16384 || g.InputTokens < 1 || g.InputTokens > 1024 || g.Millis < 0 {
				return LocalResult{}, ErrInput
			}
			if g.Mode == "likelihood" {
				if (prompt != "strategy-decisions-v3" && prompt != "strategy-decisions-v4") || g.OutputTokens != 0 || g.AtOutputLimit || len(g.Candidates) < 1 || len(g.Candidates) > 10 {
					return LocalResult{}, ErrInput
				}
				forward, selected := 0, false
				for _, c := range g.Candidates {
					if len(c.Raw) > 1024 || c.InputTokens < 1 || c.InputTokens > 1024 || c.ChoiceTokens < 1 || c.ChoiceTokens >= c.InputTokens || math.IsNaN(c.MeanLogProb) || math.IsInf(c.MeanLogProb, 0) || c.MeanLogProb > 0 {
						return LocalResult{}, ErrInput
					}
					forward += c.InputTokens
					selected = selected || c.Raw == g.Raw
				}
				if forward != g.ForwardTokens || !selected {
					return LocalResult{}, ErrInput
				}
			} else if (g.Mode != "" && g.Mode != "free") || g.OutputTokens < 1 || g.OutputTokens > 192 || g.AtOutputLimit != (g.OutputTokens == 192) || len(g.Candidates) > 0 || g.ForwardTokens != 0 {
				return LocalResult{}, ErrInput
			}
		}
	}
	if err := validateLocalRows(result, cases, docs); err != nil {
		return LocalResult{}, err
	}
	return result, nil
}

func validateLocalRows(result LocalResult, cases []Case, docs map[string]Document) error {
	known := map[string]Input{}
	for _, c := range cases {
		known[c.Input.ID] = c.Input
	}
	seen := map[string]bool{}
	for _, row := range result.Rows {
		input, ok := known[row.CaseID]
		key := row.CaseID + "/" + row.Strategy
		strategyOK := row.Strategy == "react-local" || row.Strategy == "plan-local" || (result.PromptVersion == "strategy-decisions-v4" && row.Strategy == "reflection-local")
		if !ok || seen[key] || !strategyOK || row.Report.Reads < 0 || row.Report.Decisions < 0 {
			return ErrInput
		}
		seen[key] = true
		read := map[string]bool{}
		for _, event := range row.Report.Events {
			if event.Kind == "read" && event.Code == "" {
				read[event.Document] = true
			}
		}
		for goal, fact := range row.Report.Facts {
			doc, exists := docs[fact.Source]
			if !slices.Contains(input.Required, goal) || fact.Value == "" || !exists || !read[fact.Source] || doc.Version != input.Version || fact.Version != doc.Version || doc.Values[goal] != fact.Value {
				return ErrInput
			}
		}
	}
	strategies := 2
	if result.PromptVersion == "strategy-decisions-v4" {
		strategies = 3
	}
	if len(seen) != len(cases)*strategies {
		return ErrInput
	}
	return nil
}
