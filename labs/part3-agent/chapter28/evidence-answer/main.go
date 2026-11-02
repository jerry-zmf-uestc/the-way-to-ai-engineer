package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

var (
	ErrDraft       = errors.New("rag: invalid draft")
	ErrEvidence    = errors.New("rag: invalid evidence")
	ErrUnsupported = errors.New("rag: unsupported claim")
	ErrReview      = errors.New("rag: support needs review")
)

type Evidence struct{ ID, Source, Version, Tenant, Text, FactKey, FactValue string }
type Claim struct {
	Text       string `json:"text"`
	EvidenceID string `json:"evidence_id"`
	Version    string `json:"version"`
	Quote      string `json:"quote"`
}
type Draft struct {
	Claims    []Claim `json:"claims"`
	Abstained bool    `json:"abstained,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}
type Answer struct {
	Claims    []Claim
	Abstained bool
	Reason    string
}
type Verdict string

const (
	Supported   Verdict = "supported"
	Unsupported Verdict = "unsupported"
	Unknown     Verdict = "unknown"
)

// Model与Judge由应用注入；模型不得自报自己的支持标签。
type Model func(context.Context, string, []Evidence) (string, error)
type Judge func(context.Context, Claim, Evidence) (Verdict, error)

func Parse(raw string) (Draft, error) {
	if len(raw) > 32<<10 || !utf8.ValidString(raw) {
		return Draft{}, ErrDraft
	}
	if err := uniqueFields(json.NewDecoder(strings.NewReader(raw)), 0); err != nil {
		return Draft{}, ErrDraft
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var draft Draft
	if err := dec.Decode(&draft); err != nil {
		return Draft{}, fmt.Errorf("%w: %w", ErrDraft, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Draft{}, ErrDraft
	}
	if draft.Abstained {
		if len(draft.Claims) != 0 || draft.Reason != "insufficient_evidence" {
			return Draft{}, ErrDraft
		}
		return draft, nil
	}
	if draft.Reason != "" || len(draft.Claims) == 0 || len(draft.Claims) > 16 {
		return Draft{}, ErrDraft
	}
	for _, c := range draft.Claims {
		if strings.TrimSpace(c.Text) == "" || c.EvidenceID == "" || c.Version == "" || strings.TrimSpace(c.Quote) == "" {
			return Draft{}, ErrDraft
		}
	}
	return draft, nil
}

// encoding/json accepts duplicate object keys; reject ambiguity before decoding a draft.
func uniqueFields(dec *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrDraft
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			name = strings.ToLower(name)
			if !ok || seen[name] {
				return ErrDraft
			}
			seen[name] = true
			if err := uniqueFields(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := uniqueFields(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return ErrDraft
	}
	_, err = dec.Token()
	return err
}

// Validate先证明引用归属，再检查断言支持；失败不能公开部分答案。
func Validate(ctx context.Context, draft Draft, evidence map[string]Evidence, judge Judge) error {
	if draft.Abstained || len(draft.Claims) == 0 {
		return ErrDraft
	}
	for _, claim := range draft.Claims {
		if err := ctx.Err(); err != nil {
			return err
		}
		item, ok := evidence[claim.EvidenceID]
		if !ok || claim.Version != item.Version || !strings.Contains(item.Text, claim.Quote) {
			return ErrEvidence
		}
		// 完整原文身份可确定性核对；部分摘录可能删除否定或条件，仍需支持判断。
		if claim.Text == claim.Quote && claim.Quote == item.Text {
			continue
		}
		if judge == nil {
			return ErrReview
		}
		verdict, err := judge(ctx, claim, item)
		if err != nil {
			return fmt.Errorf("rag: judge: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		switch verdict {
		case Supported:
		case Unsupported:
			return ErrUnsupported
		default:
			return ErrReview
		}
	}
	return ctx.Err()
}
func prepareEvidence(tenant string, items []Evidence) (map[string]Evidence, string, error) {
	registry := map[string]Evidence{}
	facts := map[string]string{}
	bytes := 0
	for _, item := range items {
		bytes += len(item.Text) + len(item.Source) + len(item.ID) + len(item.Version) + len(item.FactKey) + len(item.FactValue)
		if bytes > 16<<10 || item.Tenant != tenant || item.ID == "" || item.Source == "" || item.Version == "" || item.Text == "" {
			return nil, "", ErrEvidence
		}
		if _, ok := registry[item.ID]; ok {
			return nil, "", ErrEvidence
		}
		registry[item.ID] = item
		// 结构化教学事实元信息检测同键冲突，不冒充自然语言矛盾检测。
		if item.FactKey != "" {
			if old, ok := facts[item.FactKey]; ok && old != item.FactValue {
				return nil, "conflicting_evidence", nil
			}
			facts[item.FactKey] = item.FactValue
		}
	}
	return registry, "", nil
}

func Run(ctx context.Context, tenant, question string, items []Evidence, model Model, judge Judge) (Answer, error) {
	if err := ctx.Err(); err != nil {
		return Answer{}, err
	}
	if tenant == "" || strings.TrimSpace(question) == "" || len(question) > 4096 || len(items) > 8 {
		return Answer{}, ErrEvidence
	}
	if len(items) == 0 {
		return Answer{Abstained: true, Reason: "no_evidence"}, nil
	}
	registry, reason, err := prepareEvidence(tenant, items)
	if err != nil {
		return Answer{}, err
	}
	if reason != "" {
		return Answer{Abstained: true, Reason: reason}, nil
	}
	if model == nil {
		return Answer{}, ErrDraft
	}
	raw, err := model(ctx, question, append([]Evidence(nil), items...))
	if err != nil {
		return Answer{}, fmt.Errorf("rag: model: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Answer{}, err
	}
	draft, err := Parse(raw)
	if err != nil {
		return Answer{}, err
	}
	if draft.Abstained {
		if err := ctx.Err(); err != nil {
			return Answer{}, err
		}
		return Answer{Abstained: true, Reason: "model_insufficient_evidence"}, nil
	}
	if err := Validate(ctx, draft, registry, judge); err != nil {
		return Answer{}, err
	}
	return Answer{Claims: append([]Claim(nil), draft.Claims...)}, nil
}
func main() {
	retrieved := flag.Bool("retrieved", false, "read registered synthetic retrieval export from stdin")
	flag.Parse()
	if *retrieved {
		if os.Getenv("RAG_GENERATION_E2E") != "1" || os.Getenv("EMBEDDING_E2E") != "1" {
			panic("retrieved chain requires both E2E gates")
		}
		batch, err := ReadRetrieved(os.Stdin)
		if err != nil {
			panic(err)
		}
		rows, err := RunRetrieved(context.Background(), batch, os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS"), os.Getenv("RAG_GENERATION_PROFILE"))
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(rows); err != nil {
			panic(err)
		}
		return
	}
	items := []Evidence{{ID: "timeout:0", Source: "guide", Version: "v1", Tenant: "demo", Text: "Request timeout is 30 seconds.", FactKey: "timeout", FactValue: "30"}}
	// 允许构造释义草稿；固定教学标签验证Judge接入，不证明真实自然语言判断质量。
	model := func(context.Context, string, []Evidence) (string, error) {
		return `{"claims":[{"text":"Requests time out after 30 seconds.","evidence_id":"timeout:0","version":"v1","quote":"Request timeout is 30 seconds."}]}`, nil
	}
	judge := func(_ context.Context, c Claim, e Evidence) (Verdict, error) {
		if c.Text == "Requests time out after 30 seconds." && e.ID == "timeout:0" && e.Text == "Request timeout is 30 seconds." {
			return Supported, nil
		}
		return Unknown, nil
	}
	answer, err := Run(context.Background(), "demo", "What is the timeout?", items, model, judge)
	if err != nil {
		panic(err)
	}
	fmt.Printf("abstained=%t claims=%d text=%q source=%s@%s\n", answer.Abstained, len(answer.Claims), answer.Claims[0].Text, answer.Claims[0].EvidenceID, answer.Claims[0].Version)
}
