package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLocalBoundary(t *testing.T) {
	for _, prompt := range []string{"rag-draft-v3", "rag-draft-v4", "arbitrary"} {
		if _, err := GenerateLocalVariant(context.Background(), "/missing/python", "/missing/assets", "qwen3-1.7b", prompt, []DraftRequest{{Question: "q"}}); !errors.Is(err, ErrDraft) {
			t.Fatal("joint profile accepted an unregistered protocol", prompt, err)
		}
	}
	if _, err := GenerateLocalProfile(context.Background(), "/tmp/python", "/tmp/assets", "arbitrary", nil); !errors.Is(err, ErrDraft) {
		t.Fatal("unregistered model profile", err)
	}
	items := evidence()
	items[0].Tenant = "private_tenant_canary"
	items[0].Source = "private_source_canary"
	items[0].FactKey = "private_key_canary"
	items[0].FactValue = "private_value_canary"
	payload, err := modelPayload([]DraftRequest{{Question: "q", Evidence: items}})
	if err != nil || strings.Contains(string(payload), "private_") {
		t.Fatal("storage metadata entered model request", err)
	}
	if _, err := Run(context.Background(), "u", "q", evidence(), LocalDraftModel(GeneratedDraft{Raw: valid, AtOutputLimit: true}), nil); !errors.Is(err, ErrDraft) {
		t.Fatal("incomplete generation committed", err)
	}
	if _, err := GenerateLocal(context.Background(), "relative", "/tmp/assets", nil); !errors.Is(err, ErrDraft) {
		t.Fatal(err)
	}
	b := &boundedBuffer{limit: 2}
	if n, err := b.Write([]byte("abc")); n != 0 || !errors.Is(err, ErrDraft) || b.Len() != 0 {
		t.Fatal(n, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GenerateLocal(ctx, "/missing/python", "/missing/assets", []DraftRequest{{Question: "q"}}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestActualHeldoutDrafts(t *testing.T) {
	if os.Getenv("RAG_GENERATION_E2E") != "1" {
		t.Skip("explicit local RAG held-out E2E")
	}
	profile := os.Getenv("RAG_GENERATION_PROFILE")
	if profile == "" {
		profile = "smollm"
	}
	datasetName := os.Getenv("RAG_GENERATION_DATASET")
	if datasetName == "" {
		datasetName = "heldout-v1"
		if profile == "qwen3" {
			datasetName = "heldout-v2"
		}
	}
	if datasetName != "heldout-v1" && datasetName != "heldout-v2" && datasetName != "heldout-v3" {
		t.Fatal("registered dataset required")
	}
	datasetVersion := "rag-" + datasetName
	datasetPath := "testdata/" + datasetName + ".json"
	data, err := os.ReadFile(datasetPath)
	if err != nil {
		t.Fatal(err)
	}
	var dataset struct {
		Version string `json:"version"`
		Cases   []struct {
			ID            string     `json:"id"`
			Question      string     `json:"question"`
			Evidence      []Evidence `json:"evidence"`
			ExpectedQuote string     `json:"expected_quote"`
			ShouldAbstain bool       `json:"should_abstain"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(data, &dataset); err != nil || dataset.Version != datasetVersion || len(dataset.Cases) != 6 {
		t.Fatal("dataset", err)
	}
	requests := []DraftRequest{}
	indexes := []int{}
	for n, c := range dataset.Cases {
		if len(c.Evidence) > 0 {
			requests = append(requests, DraftRequest{Question: c.Question, Evidence: c.Evidence})
			indexes = append(indexes, n)
		}
	}
	promptVersion := os.Getenv("RAG_GENERATION_PROMPT")
	if promptVersion == "" {
		promptVersion = "rag-draft-v3"
		if datasetName == "heldout-v3" {
			promptVersion = "rag-draft-v4"
		}
	}
	batch, err := GenerateLocalVariant(context.Background(), os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS"), profile, promptVersion, requests)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[int]GeneratedDraft{}
	for n, index := range indexes {
		rows[index] = batch.Rows[n]
	}
	accepted, covered, correctAbstentions := 0, 0, 0
	falseAbstentions := 0
	exactJudgeAccept := 0
	for n, c := range dataset.Cases {
		row := rows[n]
		answer, err := Run(context.Background(), "demo", c.Question, c.Evidence, LocalDraftModel(row), nil)
		exactAnswer, exactErr := Run(context.Background(), "demo", c.Question, c.Evidence, LocalDraftModel(row), ExactSourceJudge)
		if !c.ShouldAbstain && exactErr == nil && !exactAnswer.Abstained {
			exactJudgeAccept++
		}
		if err != nil && len(answer.Claims) != 0 {
			t.Fatal("partial answer exposed")
		}
		passed := false
		if err == nil {
			if c.ShouldAbstain {
				passed = answer.Abstained
				if passed {
					correctAbstentions++
				}
			} else if answer.Abstained {
				falseAbstentions++
			} else {
				accepted++
				for _, claim := range answer.Claims {
					if claim.Text == c.ExpectedQuote {
						passed = true
					}
				}
				if passed {
					covered++
				}
			}
		}
		t.Logf("case=%s delivered=%d abstained=%t expected_abstain=%t rubric_pass=%t err=%v exact_source_judge_claims=%d exact_source_judge_err=%v output_limit=%t tokens=%d/%d raw=%q", c.ID, len(answer.Claims), answer.Abstained, c.ShouldAbstain, passed, err, len(exactAnswer.Claims), exactErr, row.AtOutputLimit, row.InputTokens, row.OutputTokens, row.Raw)
	}
	t.Logf("dataset=%s prompt=%s answerable_accept=%d/4 exact_expected_extract=%d/4 correct_abstain=%d/2; author rubric is not semantic Judge calibration", dataset.Version, batch.PromptVersion, accepted, covered, correctAbstentions)
	t.Logf("profile=%s exact_source_judge_answerable_accept=%d/4; same unchanged generated records, no semantic paraphrase acceptance", batch.Profile, exactJudgeAccept)
	t.Logf("false_abstention=%d/4; protocol errors are not abstentions", falseAbstentions)
}

func TestActualGeneratedDrafts(t *testing.T) {
	if os.Getenv("RAG_GENERATION_E2E") != "1" {
		t.Skip("explicit local RAG generation E2E")
	}
	requests := []DraftRequest{
		{Question: "What is the request timeout?", Evidence: []Evidence{{ID: "timeout", Source: "guide", Version: "v1", Tenant: "demo", Text: "Request timeout is 30 seconds."}}},
		{Question: "Which action stops downstream work?", Evidence: []Evidence{{ID: "cancel", Source: "guide", Version: "v1", Tenant: "demo", Text: "Pass the context cancellation signal to downstream calls."}}},
		{Question: "Can all failed operations be retried?", Evidence: []Evidence{{ID: "retry", Source: "guide", Version: "v1", Tenant: "demo", Text: "Only temporary failures may be retried. Permission failures must not be retried."}}},
		{Question: "Does a correct source ID prove that the claim is supported?", Evidence: []Evidence{{ID: "citation", Source: "guide", Version: "v1", Tenant: "demo", Text: "A valid source ID does not prove that a claim is supported by its evidence."}}},
		{Question: "What is the timeout in version 2?", Evidence: []Evidence{{ID: "timeout-v2", Source: "guide", Version: "v2", Tenant: "demo", Text: "In version 2, request timeout is 45 seconds. Version 1 used 30 seconds."}}},
		{Question: "What is the price of a flight ticket?", Evidence: []Evidence{{ID: "timeout", Source: "guide", Version: "v1", Tenant: "demo", Text: "Request timeout is 30 seconds."}}},
	}
	batch, err := GenerateLocal(context.Background(), os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS"), requests)
	if err != nil {
		t.Fatal(err)
	}
	stages := map[string]int{}
	for n, row := range batch.Rows {
		stage := "accepted_extract"
		answer, err := Run(context.Background(), "demo", requests[n].Question, requests[n].Evidence, LocalDraftModel(row), nil)
		switch {
		case row.AtOutputLimit:
			stage = "output_limit"
		case errors.Is(err, ErrDraft):
			stage = "draft_invalid"
		case errors.Is(err, ErrEvidence):
			stage = "attribution_invalid"
		case errors.Is(err, ErrReview):
			stage = "support_unknown"
		case err != nil:
			t.Fatal(err)
		}
		if err != nil && len(answer.Claims) != 0 {
			t.Fatal("partial answer exposed")
		}
		stages[stage]++
		t.Logf("case=%d stage=%s tokens=%d/%d claims=%d raw=%q", n, stage, row.InputTokens, row.OutputTokens, len(answer.Claims), row.Raw)
	}
	t.Logf("revision=%s prompt=%s stages=%v; extracts do not prove relevance or task completion", batch.Revision, batch.PromptVersion, stages)
	// No minimum model score: preserve failures rather than tuning on these cases.
	if len(batch.Rows) != 6 || strings.TrimSpace(batch.Revision) == "" {
		t.Fatal("missing cases")
	}
}
