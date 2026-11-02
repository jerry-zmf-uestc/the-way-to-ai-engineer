package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const generatorRevision = "12fd25f77366fa6b3b4b768ec3050bf629380bac"

type Hypothesis struct {
	Text          string `json:"text"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	AtOutputLimit bool   `json:"at_output_limit"`
	Trust         string `json:"trust"`
}
type Hypotheses struct {
	Revision      string       `json:"revision"`
	PromptVersion string       `json:"prompt_version"`
	Rows          []Hypothesis `json:"rows"`
}

func ValidateHypotheses(h Hypotheses, count int) error {
	if h.Revision != generatorRevision || h.PromptVersion != "hypothetical-v1" || count < 1 || count > 8 || len(h.Rows) != count {
		return ErrInput
	}
	for _, row := range h.Rows {
		if row.Trust != "hypothetical" || strings.TrimSpace(row.Text) == "" || len(row.Text) > 4096 || row.InputTokens < 1 || row.InputTokens > 256 || row.OutputTokens < 1 || row.OutputTokens > 64 || row.AtOutputLimit != (row.OutputTokens == 64) {
			return ErrInput
		}
	}
	return nil
}
func GenerateHypothetical(ctx context.Context, python, assets string, questions []Question) (Hypotheses, error) {
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) || len(questions) == 0 || len(questions) > 8 {
		return Hypotheses{}, ErrInput
	}
	input := []string{}
	for _, q := range questions {
		if q.ID == "" || strings.TrimSpace(q.Text) == "" || len(q.Text) > 4096 {
			return Hypotheses{}, ErrInput
		}
		input = append(input, q.Text)
	}
	_, file, _, _ := runtime.Caller(0)
	data, err := json.Marshal(map[string]any{"questions": input})
	if err != nil {
		return Hypotheses{}, err
	}
	child, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(child, python, filepath.Join(filepath.Dir(file), "generator.py"), assets)
	cmd.Stdin = bytes.NewReader(data)
	output := &cappedBuffer{limit: 64 << 10}
	cmd.Stdout = output
	stderr := &cappedBuffer{limit: 4096}
	cmd.Stderr = stderr
	if err = cmd.Run(); err != nil {
		if child.Err() != nil {
			return Hypotheses{}, child.Err()
		}
		return Hypotheses{}, errors.New("local hypothetical generator failed")
	}
	dec := json.NewDecoder(bytes.NewReader(output.Bytes()))
	dec.DisallowUnknownFields()
	var h Hypotheses
	if err = dec.Decode(&h); err != nil {
		return Hypotheses{}, ErrInput
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return Hypotheses{}, ErrInput
	}
	if err = ValidateHypotheses(h, len(questions)); err != nil {
		return Hypotheses{}, err
	}
	return h, nil
}
