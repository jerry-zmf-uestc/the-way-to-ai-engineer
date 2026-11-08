package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

type AccountingGenerationRun struct {
	CaseID     string
	Input      TaskInput
	Generation acceptanceGenerationRow
	Score      AccountingScore
	Error      string
}
type AccountingGenerationRecord struct {
	DatasetVersion, PromptVersion, Profile, Revision string
	Runs                                             []AccountingGenerationRun
}

func accountingInputCaptured(input TaskInput, row acceptanceGenerationRow) bool {
	if len(row.InputMessages) != 2 || row.InputMessages[0].Role != "system" || row.InputMessages[1].Role != "user" {
		return false
	}
	var expected, actual any
	if uniqueCoverageJSON(json.NewDecoder(bytes.NewBufferString(row.InputMessages[1].Content)), 0) != nil || json.Unmarshal([]byte(input.Input), &expected) != nil || json.Unmarshal([]byte(row.InputMessages[1].Content), &actual) != nil {
		return false
	}
	return reflect.DeepEqual(expected, actual)
}

func TestActualAccountingLocalGeneration(t *testing.T) {
	if os.Getenv("ACCOUNTING_GENERATION_E2E") != "1" {
		t.Skip("explicit resource accounting generation E2E")
	}
	python, assets := os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS")
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) {
		t.Fatal("locked local paths required")
	}
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "..", "..", "chapter28", "evidence-answer", "local_generator.py")
	lock := filepath.Join(filepath.Dir(script), "generation-requirements.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, python, "-c", "import sys; from pathlib import Path; from importlib.metadata import version; pairs=[line.split('==') for line in Path(sys.argv[1]).read_text().splitlines() if line and not line.startswith('#')]; assert all(version(n)==v for n,v in pairs)", lock)
	check.Env = append(os.Environ(), "PYTHONNOUSERSITE=1")
	if err := check.Run(); err != nil {
		t.Fatal("runtime lock", err)
	}
	dataset := accountingFixture(t)
	if dataset.Version != "resource-accounting-development-v1" || len(dataset.Cases) != 12 {
		t.Fatal("frozen accounting task scope required")
	}
	record := AccountingGenerationRecord{DatasetVersion: dataset.Version, PromptVersion: "resource-accounting-v1", Profile: "qwen3-1.7b", Revision: "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e"}
	defer func() {
		if path := os.Getenv("ACCOUNTING_RECORD_PATH"); path != "" {
			if !filepath.IsAbs(path) {
				t.Fatal("absolute new record path")
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
	}()
	for start := 0; start < len(dataset.Cases); start += 4 {
		end := min(start+4, len(dataset.Cases))
		inputs := []TaskInput{}
		requests := []map[string]any{}
		for _, c := range dataset.Cases[start:end] {
			input, err := BuildAccountingInput(c)
			if err != nil {
				t.Fatal(err)
			}
			inputs = append(inputs, input)
			requests = append(requests, map[string]any{"question": input.Input, "evidence": []any{}})
		}
		payload, err := json.Marshal(requests)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, python, script, assets, record.Profile, record.PromptVersion)
		cmd.Env = append(os.Environ(), "PYTHONNOUSERSITE=1", "HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "TOKENIZERS_PARALLELISM=false")
		cmd.Stdin = bytes.NewReader(payload)
		stdout, stderr := &acceptanceBuffer{limit: 128 << 10}, &acceptanceBuffer{limit: 4096}
		cmd.Stdout, cmd.Stderr = stdout, stderr
		if err := cmd.Run(); err != nil {
			t.Fatal("local accounting execution failed", err)
		}
		if uniqueCoverageJSON(json.NewDecoder(bytes.NewReader(stdout.Bytes())), 0) != nil {
			t.Fatal("batch JSON")
		}
		dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
		dec.DisallowUnknownFields()
		var batch acceptanceGenerationBatch
		if dec.Decode(&batch) != nil || dec.Decode(new(any)) != io.EOF || batch.Profile != record.Profile || batch.Revision != record.Revision || batch.PromptVersion != record.PromptVersion || len(batch.Rows) != len(inputs) {
			t.Fatal("batch identity")
		}
		for n, row := range batch.Rows {
			c := dataset.Cases[start+n]
			if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) || !accountingInputCaptured(inputs[n], row) {
				t.Fatal("actual input or budget mismatch", c.ID)
			}
			score, err := ScoreAccounting(c, row.Raw)
			errorText := ""
			if row.AtOutputLimit {
				score.Status = "failed"
				errorText = "output_limit"
			} else if err != nil {
				errorText = err.Error()
			}
			record.Runs = append(record.Runs, AccountingGenerationRun{c.ID, inputs[n], row, score, errorText})
			t.Logf("case=%s status=%s known=%t value=%t unit=%t refs=%t input=%d output=%d truncated=%t", c.ID, score.Status, score.KnownCorrect, score.ValueCorrect, score.UnitCorrect, score.RefsValid, row.InputTokens, row.OutputTokens, row.AtOutputLimit)
		}
	}

}
