package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type QuerySelection struct {
	Text, Origin, Reason      string
	GenerationAttempted       bool
	GenerationUsageKnown      bool
	InputTokens, OutputTokens int
}

func SelectExpandedQuery(ctx context.Context, original string, generated *Hypotheses, generationError error) (QuerySelection, error) {
	selection := QuerySelection{Text: original, Origin: "original", Reason: "not_generated", GenerationAttempted: generated != nil || generationError != nil}
	if strings.TrimSpace(original) == "" || len(original) > 4096 || !utf8.ValidString(original) {
		return QuerySelection{}, ErrInput
	}
	if generated != nil && generated.Revision == generatorRevision && generated.PromptVersion == "hypothetical-v1" && len(generated.Rows) == 1 {
		row := generated.Rows[0]
		if row.InputTokens >= 1 && row.InputTokens <= 256 && row.OutputTokens >= 1 && row.OutputTokens <= 64 {
			selection.GenerationUsageKnown = true
			selection.InputTokens, selection.OutputTokens = row.InputTokens, row.OutputTokens
		}
	}
	stopError := ctx.Err()
	if stopError == nil && errors.Is(generationError, context.Canceled) {
		stopError = generationError
	}
	if stopError != nil {
		selection.Text, selection.Origin, selection.Reason = "", "stopped", "canceled"
		if errors.Is(stopError, context.DeadlineExceeded) {
			selection.Reason = "deadline_exceeded"
		}
		return selection, stopError
	}
	if generationError != nil {
		selection.Reason = "generation_failed"
		if errors.Is(generationError, context.DeadlineExceeded) {
			selection.Reason = "generation_deadline"
		}
		return selection, nil
	}
	if generated == nil {
		return selection, nil
	}
	if ValidateHypotheses(*generated, 1) != nil || !utf8.ValidString(generated.Rows[0].Text) {
		selection.Reason = "invalid_hypothesis"
		return selection, nil
	}
	if generated.Rows[0].AtOutputLimit {
		selection.Reason = "output_limit"
		return selection, nil
	}
	selection.Text, selection.Origin, selection.Reason = generated.Rows[0].Text, "hypothetical", "selected"
	return selection, nil
}

func queryFallbackDemo() {
	h := Hypotheses{Revision: generatorRevision, PromptVersion: "hypothetical-v1", Rows: []Hypothesis{{Text: "unverified capped candidate", Trust: "hypothetical", InputTokens: 20, OutputTokens: 64, AtOutputLimit: true}}}
	selected, err := SelectExpandedQuery(context.Background(), "What setting bounds request duration?", &h, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("origin=%s reason=%s generation_usage_known=%t generation_tokens=%d/%d\n", selected.Origin, selected.Reason, selected.GenerationUsageKnown, selected.InputTokens, selected.OutputTokens)
}
