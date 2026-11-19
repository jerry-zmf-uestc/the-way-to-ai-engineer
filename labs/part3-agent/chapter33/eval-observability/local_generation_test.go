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
	"reflect"
	"runtime"
	"testing"
	"time"
)

const localRevision = "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e"

type LocalGeneration struct {
	Raw           string                           `json:"raw"`
	InputTokens   int                              `json:"input_tokens"`
	OutputTokens  int                              `json:"output_tokens"`
	AtOutputLimit bool                             `json:"at_output_limit"`
	InputMessages []struct{ Role, Content string } `json:"input_messages"`
}
type LocalObservedCall struct {
	InputHash   string
	Observation Observation
	Generation  LocalGeneration
	Attempt     int    `json:",omitempty"`
	PreviousRaw string `json:",omitempty"`
}
type LocalObservedRecord struct {
	Complete                         bool
	Profile, Revision, PromptVersion string
	Archive                          Archive
	Generations                      []LocalObservedCall
	Mode                             string `json:",omitempty"`
}
type localBoundBuffer struct {
	bytes.Buffer
	limit int
}

func (b *localBoundBuffer) Write(data []byte) (int, error) {
	if b.Len()+len(data) > b.limit {
		return 0, ErrRecord
	}
	return b.Buffer.Write(data)
}

func capturedLocalInput(observation Observation, row LocalGeneration) bool {
	return capturedLocalFeedbackInput(observation, row, "")
}

func capturedLocalFeedbackInput(observation Observation, row LocalGeneration, previous string) bool {
	if len(row.InputMessages) != 2 || row.InputMessages[0].Role != "system" || row.InputMessages[1].Role != "user" {
		return false
	}
	request := localAnswerRequest(observation)
	if previous != "" {
		request["question"] = protocolFeedbackQuestion(request["question"].(string), previous)
	}
	evidence := []map[string]string{}
	for _, item := range request["evidence"].([]map[string]string) {
		evidence = append(evidence, map[string]string{"evidence_id": item["id"], "version": item["version"], "text": item["text"]})
	}
	var actual, expected any
	payload, _ := json.Marshal(map[string]any{"question": request["question"], "evidence": evidence})
	if uniqueJSON(json.NewDecoder(bytes.NewBufferString(row.InputMessages[1].Content)), 0) != nil || json.Unmarshal([]byte(row.InputMessages[1].Content), &actual) != nil || json.Unmarshal(payload, &expected) != nil {
		return false
	}
	return reflect.DeepEqual(actual, expected)
}

func TestActualObservedLocalGeneration(t *testing.T) {
	if os.Getenv("OBSERVABILITY_GENERATION_E2E") != "1" {
		t.Skip("explicit local observed generation")
	}
	python, assets := os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS")
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) {
		t.Fatal("locked absolute paths required")
	}
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "..", "..", "chapter28", "evidence-answer", "local_generator.py")
	ctx, cancel := context.WithTimeout(context.Background(), 420*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, python, "-c", "import sys; from pathlib import Path; from importlib.metadata import version; assert sys.version_info[:3]==(3,12,14); pairs=[s.split('==') for s in Path(sys.argv[1]).read_text().splitlines() if s and not s.startswith('#')]; assert all(version(n)==v for n,v in pairs)", filepath.Join(filepath.Dir(script), "generation-requirements.lock"))
	check.Env = append(os.Environ(), "PYTHONNOUSERSITE=1")
	if err := check.Run(); err != nil {
		t.Fatal("generation runtime lock", err)
	}
	record := LocalObservedRecord{Profile: "qwen3-1.7b", Revision: localRevision, PromptVersion: "rag-joint-v3"}
	mode := os.Getenv("OBSERVABILITY_GENERATION_MODE")
	if mode != "" && mode != "protocol-repair" {
		t.Fatal("registered generation mode")
	}
	config, maximum := "baseline", 4
	if mode == "protocol-repair" {
		record.Mode, config, maximum = mode, mode, 8
	}
	attempts, previous := map[string]int{}, map[string]string{}
	defer func() {
		if path := os.Getenv("OBSERVABILITY_RECORD_PATH"); path != "" {
			if !filepath.IsAbs(path) {
				t.Fatal("absolute new record path")
			}
			data, err := json.MarshalIndent(record, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.Write(data)
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatal("record save", err, closeErr)
			}
		}
	}()
	a, err := recordObserved(ctx, config, func(ctx context.Context, kind, input string) (ObservedReply, error) {
		if kind != "model-final" {
			output, err := decision(ctx, "baseline", kind, input)
			origin := "application"
			if kind == "tool-read" {
				origin = "tool"
			}
			return ObservedReply{output, CallMetrics{Origin: origin}}, err
		}
		if len(record.Generations) >= maximum {
			t.Fatal("generation call bound")
		}
		var observation Observation
		if decodeJSON([]byte(input), &observation, 4096) != nil {
			t.Fatal("observation shape")
		}
		key := hash(input)
		attempts[key]++
		if attempts[key] > 2 {
			t.Fatal("case final budget")
		}
		prior := ""
		request := localAnswerRequest(observation)
		if mode == "protocol-repair" && attempts[key] > 1 {
			prior = previous[key]
			request["question"] = protocolFeedbackQuestion(request["question"].(string), prior)
		}
		payload, _ := json.Marshal([]map[string]any{request})
		cmd := exec.CommandContext(ctx, python, script, assets, record.Profile, record.PromptVersion)
		cmd.Env = append(os.Environ(), "PYTHONNOUSERSITE=1", "HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "TOKENIZERS_PARALLELISM=false")
		cmd.Stdin = bytes.NewReader(payload)
		out, stderr := &localBoundBuffer{limit: 64 << 10}, &localBoundBuffer{limit: 4096}
		cmd.Stdout, cmd.Stderr = out, stderr
		if err := cmd.Run(); err != nil {
			t.Fatal("locked generation process failed", err)
		}
		var batch struct {
			Profile, Revision string
			PromptVersion     string            `json:"prompt_version"`
			Rows              []LocalGeneration `json:"rows"`
		}
		dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
		dec.DisallowUnknownFields()
		if uniqueJSON(json.NewDecoder(bytes.NewReader(out.Bytes())), 0) != nil || dec.Decode(&batch) != nil || dec.Decode(new(any)) != io.EOF || batch.Profile != record.Profile || batch.Revision != record.Revision || batch.PromptVersion != record.PromptVersion || len(batch.Rows) != 1 {
			t.Fatal("generation batch identity")
		}
		row := batch.Rows[0]
		if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) || !capturedLocalFeedbackInput(observation, row, prior) {
			t.Fatal("captured input or generation budget")
		}
		capture := LocalObservedCall{InputHash: key, Observation: observation, Generation: row}
		if mode == "protocol-repair" {
			capture.Attempt, capture.PreviousRaw = attempts[key], prior
		}
		record.Generations = append(record.Generations, capture)
		previous[key] = row.Raw
		metrics := CallMetrics{Origin: "model", ModelRevision: localRevision, UsageSource: "local-tokenizer", UsageKnown: true, InputTokens: int64(row.InputTokens), OutputTokens: int64(row.OutputTokens)}
		if row.AtOutputLimit {
			return ObservedReply{Metrics: metrics}, ErrRecord
		}
		answer, err := adaptLocalAnswer(row.Raw, observation)
		if err != nil {
			if mode == "protocol-repair" {
				err = errors.Join(ErrAnswerProtocol, err)
			}
			return ObservedReply{Metrics: metrics}, err
		}
		return ObservedReply{encoded(answer), metrics}, nil
	})
	record.Archive = a
	if err != nil {
		t.Fatal("actual recording", err)
	}
	report, err := evaluate(context.Background(), a)
	if err != nil || report.CallUsage == nil || report.CallUsage.ModelCalls != len(record.Generations) || report.CallUsage.ModelCalls < 4 || report.CallUsage.ModelCalls > maximum || report.CallUsage.ApplicationCalls != 4 || report.CallUsage.ToolCalls != 4 || !report.UsageKnown || report.CostKnown {
		t.Fatal(report, err)
	}
	record.Complete = true
	t.Logf("actual-final-calls=%d passed=%d/%d execution_failed=%d gate=%t input=%d output=%d", report.CallUsage.ModelCalls, report.Passed, report.Total, report.ExecutionFailed, report.Gate, report.CallUsage.KnownInputTokens, report.CallUsage.KnownOutputTokens)
}
