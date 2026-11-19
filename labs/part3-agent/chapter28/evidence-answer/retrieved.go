package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type RetrievedRow struct {
	ID       string
	Question string
	Evidence []Evidence
}
type RetrievedBatch struct {
	Protocol, Revision, Space string
	Rows                      []RetrievedRow
}

// ReadRetrieved accepts only the synthetic demo/v1 exchange, not arbitrary user data.
func ReadRetrieved(source io.Reader) (RetrievedBatch, error) {
	raw, err := io.ReadAll(io.LimitReader(source, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 || !utf8.Valid(raw) {
		return RetrievedBatch{}, ErrEvidence
	}
	if uniqueFields(json.NewDecoder(bytes.NewReader(raw)), 0) != nil {
		return RetrievedBatch{}, ErrEvidence
	}
	var shape any
	if json.Unmarshal(raw, &shape) != nil || !unambiguousRetrievedKeys(shape) {
		return RetrievedBatch{}, ErrEvidence
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var batch RetrievedBatch
	if dec.Decode(&batch) != nil || batch.Protocol != "retrieved-evidence-v1" || batch.Revision != "1110a243fdf4706b3f48f1d95db1a4f5529b4d41" || batch.Space != "minilm-l6-v2-onnx-mean-l2-256-v1" || len(batch.Rows) < 1 || len(batch.Rows) > 6 {
		return RetrievedBatch{}, ErrEvidence
	}
	var tail any
	if dec.Decode(&tail) != io.EOF {
		return RetrievedBatch{}, ErrEvidence
	}
	seen := map[string]bool{}
	for _, row := range batch.Rows {
		if row.ID == "" || len(row.ID) > 128 || seen[row.ID] || strings.TrimSpace(row.Question) == "" || len(row.Question) > 4096 || len(row.Evidence) > 1 {
			return RetrievedBatch{}, ErrEvidence
		}
		seen[row.ID] = true
		for _, item := range row.Evidence {
			if item.Tenant != "demo" || item.Version != "v1" || item.ID == "" || item.Source != "fixture:"+item.ID || item.Text == "" || len(item.Text) > 4096 || item.FactKey != "" || item.FactValue != "" {
				return RetrievedBatch{}, ErrEvidence
			}
		}
	}
	return batch, nil
}

func unambiguousRetrievedKeys(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		if len(v) > 16 {
			return false
		}
		seen := []string{}
		for key, child := range v {
			for _, prior := range seen {
				if strings.EqualFold(prior, key) {
					return false
				}
			}
			if !unambiguousRetrievedKeys(child) {
				return false
			}
			seen = append(seen, key)
		}
	case []any:
		for _, child := range v {
			if !unambiguousRetrievedKeys(child) {
				return false
			}
		}
	}
	return true
}

type ChainRow struct {
	Profile, ModelRevision, PromptVersion, DiagnosticDraft string
	ID, Status                                             string
	InputTokens, OutputTokens                              int
	AtOutputLimit                                          bool
	Answer                                                 Answer
}

func RunRetrieved(ctx context.Context, batch RetrievedBatch, python, assets, profile string) ([]ChainRow, error) {
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) || (profile != "smollm" && profile != "qwen3") {
		return nil, ErrDraft
	}
	// Revalidate programmatic inputs with the same registered transport contract.
	raw, err := json.Marshal(batch)
	if err != nil {
		return nil, ErrEvidence
	}
	batch, err = ReadRetrieved(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	rows := []ChainRow{}
	for _, item := range batch.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row := ChainRow{ID: item.ID}
		model := func(callCtx context.Context, question string, evidence []Evidence) (string, error) {
			generated, err := GenerateLocalVariant(callCtx, python, assets, profile, "rag-draft-v4", []DraftRequest{{Question: question, Evidence: evidence}})
			if err != nil {
				return "", err
			}
			record := generated.Rows[0]
			row.Profile, row.ModelRevision, row.PromptVersion, row.DiagnosticDraft = generated.Profile, generated.Revision, generated.PromptVersion, record.Raw
			row.InputTokens, row.OutputTokens, row.AtOutputLimit = record.InputTokens, record.OutputTokens, record.AtOutputLimit
			return LocalDraftModel(record)(callCtx, question, evidence)
		}
		answer, runErr := Run(ctx, "demo", item.Question, item.Evidence, model, ExactSourceJudge)
		row.Answer = answer
		switch {
		case errors.Is(runErr, context.Canceled), errors.Is(runErr, context.DeadlineExceeded):
			return nil, runErr
		case errors.Is(runErr, ErrDraft):
			row.Status = "draft_failed"
		case errors.Is(runErr, ErrEvidence):
			row.Status = "attribution_failed"
		case errors.Is(runErr, ErrReview):
			row.Status = "support_unknown"
		case runErr != nil:
			row.Status = "generation_or_support_failed"
		case answer.Abstained:
			row.Status = "abstained"
		default:
			row.Status = "accepted_extract"
		}
		rows = append(rows, row)
	}
	return rows, nil
}
