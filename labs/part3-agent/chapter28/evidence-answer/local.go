package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type DraftRequest struct {
	Question string     `json:"question"`
	Evidence []Evidence `json:"evidence"`
}

type PromptEvidence struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Text    string `json:"text"`
}
type PromptRequest struct {
	Question string           `json:"question"`
	Evidence []PromptEvidence `json:"evidence"`
}

func modelPayload(requests []DraftRequest) ([]byte, error) {
	if len(requests) == 0 || len(requests) > 6 {
		return nil, ErrDraft
	}
	input := make([]PromptRequest, 0, len(requests))
	for _, r := range requests {
		if strings.TrimSpace(r.Question) == "" || len(r.Question) > 4096 || len(r.Evidence) > 8 {
			return nil, ErrDraft
		}
		item := PromptRequest{Question: r.Question, Evidence: make([]PromptEvidence, 0, len(r.Evidence))}
		for _, e := range r.Evidence {
			item.Evidence = append(item.Evidence, PromptEvidence{ID: e.ID, Version: e.Version, Text: e.Text})
		}
		input = append(input, item)
	}
	data, err := json.Marshal(input)
	if err != nil || len(data) > 64<<10 {
		return nil, ErrDraft
	}
	return data, nil
}

type GeneratedDraft struct {
	Raw           string             `json:"raw"`
	InputTokens   int                `json:"input_tokens"`
	OutputTokens  int                `json:"output_tokens"`
	AtOutputLimit bool               `json:"at_output_limit"`
	InputMessages []GeneratorMessage `json:"input_messages,omitempty"`
}

type GeneratorMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type GeneratedBatch struct {
	InputSHA256   string           `json:"input_sha256,omitempty"`
	Profile       string           `json:"profile"`
	Revision      string           `json:"revision"`
	PromptVersion string           `json:"prompt_version"`
	Rows          []GeneratedDraft `json:"rows"`
}

// LocalDraftModel is for an offline evaluation record paired with its original request.
// A live adapter must generate inside Run after evidence scope validation.
func LocalDraftModel(row GeneratedDraft) Model {
	return func(ctx context.Context, _ string, _ []Evidence) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if row.AtOutputLimit {
			return "", ErrDraft
		}
		return row.Raw, nil
	}
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, ErrDraft
	}
	return b.Buffer.Write(p)
}
func GenerateLocal(ctx context.Context, python, assets string, requests []DraftRequest) (GeneratedBatch, error) {
	return GenerateLocalProfile(ctx, python, assets, "smollm", requests)
}

func GenerateLocalProfile(ctx context.Context, python, assets, profile string, requests []DraftRequest) (GeneratedBatch, error) {
	return GenerateLocalVariant(ctx, python, assets, profile, "rag-draft-v3", requests)
}

func GenerateLocalVariant(ctx context.Context, python, assets, profile, promptVersion string, requests []DraftRequest) (GeneratedBatch, error) {
	if promptVersion != "rag-draft-v3" && promptVersion != "rag-draft-v4" && promptVersion != "rag-joint-v1" && promptVersion != "rag-joint-v2" && promptVersion != "rag-joint-v3" {
		return GeneratedBatch{}, ErrDraft
	}
	if profile == "qwen3-1.7b" && promptVersion != "rag-joint-v1" && promptVersion != "rag-joint-v2" && promptVersion != "rag-joint-v3" {
		return GeneratedBatch{}, ErrDraft
	}
	revisions := map[string]string{"smollm": "12fd25f77366fa6b3b4b768ec3050bf629380bac", "qwen3": "c1899de289a04d12100db370d81485cdf75e47ca", "qwen3-1.7b": "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e"}
	expected, ok := revisions[profile]
	if !ok {
		return GeneratedBatch{}, ErrDraft
	}
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) || len(requests) == 0 || len(requests) > 6 {
		return GeneratedBatch{}, ErrDraft
	}
	payload, err := modelPayload(requests)
	if err != nil {
		return GeneratedBatch{}, ErrDraft
	}
	_, file, _, _ := runtime.Caller(0)
	child, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(child, python, filepath.Join(filepath.Dir(file), "local_generator.py"), assets, profile, promptVersion)
	cmd.Stdin = bytes.NewReader(payload)
	stdout := &boundedBuffer{limit: 128 << 10}
	stderr := &boundedBuffer{limit: 4096}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err = cmd.Run(); err != nil {
		if child.Err() != nil {
			return GeneratedBatch{}, child.Err()
		}
		return GeneratedBatch{}, errors.New("rag: local generation failed")
	}
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	dec.DisallowUnknownFields()
	var batch GeneratedBatch
	if err = dec.Decode(&batch); err != nil {
		return GeneratedBatch{}, ErrDraft
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || batch.Profile != profile || batch.Revision != expected || batch.PromptVersion != promptVersion || len(batch.Rows) != len(requests) {
		return GeneratedBatch{}, ErrDraft
	}
	for n, r := range batch.Rows {
		if len(r.Raw) > 16384 || r.InputTokens < 1 || r.InputTokens > 1024 || r.OutputTokens < 1 || r.OutputTokens > 192 || r.AtOutputLimit != (r.OutputTokens == 192) {
			return GeneratedBatch{}, ErrDraft
		}
		if promptVersion == "rag-joint-v3" {
			if validateCitationInput(r, requests[n]) != nil {
				return GeneratedBatch{}, ErrDraft
			}
		} else if len(r.InputMessages) != 0 {
			return GeneratedBatch{}, ErrDraft
		}
	}
	digest := sha256.Sum256(payload)
	batch.InputSHA256 = hex.EncodeToString(digest[:])
	return batch, nil
}
