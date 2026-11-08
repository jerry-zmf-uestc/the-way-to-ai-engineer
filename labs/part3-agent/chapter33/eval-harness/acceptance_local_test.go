package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type acceptanceGenerationRow struct {
	Raw           string `json:"raw"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	AtOutputLimit bool   `json:"at_output_limit"`
	InputMessages []struct {
		Role, Content string
	} `json:"input_messages"`
}
type acceptanceGenerationBatch struct {
	Profile, Revision string
	PromptVersion     string                    `json:"prompt_version"`
	Rows              []acceptanceGenerationRow `json:"rows"`
}
type acceptanceGenerationRecord struct {
	Mode     string `json:",omitempty"`
	Requests []json.RawMessage
	Batches  []acceptanceGenerationBatch
	Report   AcceptanceReport
	Error    string
}
type acceptanceBuffer struct {
	bytes.Buffer
	limit int
}

func capturedHarborInputMatches(payload []byte, row acceptanceGenerationRow) bool {
	if len(row.InputMessages) != 2 || row.InputMessages[0].Role != "system" || row.InputMessages[1].Role != "user" {
		return false
	}
	if uniqueCoverageJSON(json.NewDecoder(strings.NewReader(row.InputMessages[1].Content)), 0) != nil {
		return false
	}
	var requests []struct {
		Question string
		Evidence []AcceptanceEvidence
	}
	if json.Unmarshal(payload, &requests) != nil || len(requests) != 1 {
		return false
	}
	object, ok := exactObject([]byte(row.InputMessages[1].Content), "question", "evidence")
	var question string
	var evidence []json.RawMessage
	if !ok || json.Unmarshal(object["question"], &question) != nil || question != requests[0].Question || json.Unmarshal(object["evidence"], &evidence) != nil || len(evidence) != len(requests[0].Evidence) {
		return false
	}
	for n, data := range evidence {
		fields, ok := exactObject(data, "evidence_id", "version", "text")
		var e AcceptanceEvidence
		if !ok || json.Unmarshal(fields["evidence_id"], &e.ID) != nil || json.Unmarshal(fields["version"], &e.Version) != nil || json.Unmarshal(fields["text"], &e.Text) != nil || e != requests[0].Evidence[n] {
			return false
		}
	}
	return true
}

func (b *acceptanceBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}

func TestActualAcceptanceLocalGeneration(t *testing.T) {
	if os.Getenv("RAG_GENERATION_E2E") != "1" {
		t.Skip("explicit local generation E2E")
	}
	mode := os.Getenv("ACCEPTANCE_GENERATION_MODE")
	if mode == "" {
		mode = "feedback"
	}
	if mode != "feedback" && mode != "no-feedback" {
		t.Fatal("registered acceptance input mode required")
	}
	python, assets := os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS")
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) {
		t.Fatal("isolated Python and fixed assets require absolute paths")
	}
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "..", "..", "chapter28", "evidence-answer", "local_generator.py")
	lock := filepath.Join(filepath.Dir(script), "generation-requirements.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, python, "-c", "import sys; from pathlib import Path; from importlib.metadata import version; pairs=[line.split('==') for line in Path(sys.argv[1]).read_text().splitlines() if line and not line.startswith('#')]; assert all(version(name)==wanted for name,wanted in pairs)", lock)
	check.Env = append(os.Environ(), "PYTHONNOUSERSITE=1")
	if err := check.Run(); err != nil {
		t.Fatal("locked generation runtime unavailable", err)
	}
	record := acceptanceGenerationRecord{Mode: mode}
	generate := func(ctx context.Context, input TaskInput, feedback []CriterionDecision) (string, error) {
		if mode == "no-feedback" {
			feedback = nil
		}
		payload, err := harborRequest(input, feedback)
		if err != nil {
			return "", err
		}
		record.Requests = append(record.Requests, bytes.Clone(payload))
		cmd := exec.CommandContext(ctx, python, script, assets, "qwen3-1.7b", "rag-joint-v3")
		cmd.Env = append(os.Environ(), "PYTHONNOUSERSITE=1", "HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "TOKENIZERS_PARALLELISM=false")
		cmd.Stdin = bytes.NewReader(payload)
		stdout, stderr := &acceptanceBuffer{limit: 128 << 10}, &acceptanceBuffer{limit: 4096}
		cmd.Stdout, cmd.Stderr = stdout, stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", errors.New("local acceptance generation failed")
		}
		if uniqueCoverageJSON(json.NewDecoder(bytes.NewReader(stdout.Bytes())), 0) != nil {
			return "", ErrAcceptance
		}
		dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
		dec.DisallowUnknownFields()
		var batch acceptanceGenerationBatch
		if dec.Decode(&batch) != nil || dec.Decode(new(any)) != io.EOF || batch.Profile != "qwen3-1.7b" || batch.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" || batch.PromptVersion != "rag-joint-v3" || len(batch.Rows) != 1 {
			return "", ErrAcceptance
		}
		record.Batches = append(record.Batches, batch)
		row := batch.Rows[0]
		if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) || len(row.InputMessages) != 2 || row.InputMessages[0].Role != "system" || row.InputMessages[1].Role != "user" {
			return "", ErrAcceptance
		}
		if !capturedHarborInputMatches(payload, row) {
			return "", ErrAcceptance
		}
		if row.AtOutputLimit {
			return "", ErrAcceptance
		}
		return row.Raw, nil
	}
	input := TaskInput{ID: "both-premises", Input: harborQuestion, AllowedSources: []string{"deployment", "policy"}}
	policy := AcceptancePolicy{Version: "harbor-frozen-v1", Criteria: []string{"protocol", "citations", "conclusion"}, MaxCandidates: 2}
	report, err := RunAcceptance(ctx, input, policy, generate, ReviewHarborCandidate)
	record.Report = report
	if err != nil {
		record.Error = err.Error()
	}
	if path := os.Getenv("ACCEPTANCE_RECORD_PATH"); path != "" {
		if !filepath.IsAbs(path) {
			t.Fatal("absolute new record path required")
		}
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, err = out.Write(data)
		closeErr := out.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
	}
	if len(record.Batches) == 0 || len(record.Batches) != report.GenerationCalls || report.GenerationCalls > 2 || report.Status != "passed" && report.Output != "" {
		t.Fatal("generation or delivery contract failed", record.Error)
	}
	t.Logf("actual local acceptance mode=%s status=%s generations=%d reviews=%d error=%s", mode, report.Status, report.GenerationCalls, report.ReviewCalls, record.Error)
}
