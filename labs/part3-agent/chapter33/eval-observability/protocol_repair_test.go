package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestProtocolRepairBoundIsArchivedAndReplayed(t *testing.T) {
	finals := map[string]int{}
	a, err := recordObserved(context.Background(), "protocol-repair", func(ctx context.Context, kind, input string) (ObservedReply, error) {
		r, err := syntheticObservedDriver(ctx, kind, input)
		if kind == "model-final" {
			finals[input]++
			if finals[input] == 1 {
				r.Output = ""
				err = ErrAnswerProtocol
			}
		}
		return r, err
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := evaluate(context.Background(), a)
	if err != nil || !r.Gate || r.CallUsage.SyntheticCalls != 8 || r.CallUsage.SyntheticOutputTokens != 40 {
		t.Fatal(r, err)
	}
	for _, run := range a.Runs {
		if run.FinalLimit != 2 || len(run.Calls) != 4 || run.Calls[2].Error != "answer_protocol" {
			t.Fatal(run)
		}
	}
	a.Runs[0].FinalLimit = 1
	if _, err = evaluate(context.Background(), a); !errors.Is(err, ErrRecord) {
		t.Fatal("changed retry bound accepted", err)
	}
}

func TestProtocolRepairDoesNotRetryOtherFailuresOrInventFeedbackFacts(t *testing.T) {
	for _, failure := range []error{ErrRecord, context.Canceled, context.DeadlineExceeded} {
		calls := 0
		_, err := executeLimited(context.Background(), cases()[0].Input, func(ctx context.Context, kind, input string) (string, error) {
			if kind == "model-final" {
				calls++
				return "", failure
			}
			return decision(ctx, "baseline", kind, input)
		}, 1, 2)
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatal("unrelated failure retried", err, calls)
		}
	}
	question := "Only use these sources."
	raw := "{\"abstained\":true}>"
	feedback := protocolFeedbackQuestion(question, raw)
	if !strings.HasPrefix(feedback, question) || !strings.Contains(feedback, "answer_protocol") || strings.Contains(feedback, "Expected") {
		t.Fatal(feedback)
	}
}

func TestStoppedExecutionNeverEntersInitialDriver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	answer, err := executeLimited(ctx, cases()[0].Input, func(context.Context, string, string) (string, error) { calls++; return "", nil }, 1, 2)
	if !errors.Is(err, context.Canceled) || calls != 0 || answer != (Answer{}) {
		t.Fatal("stopped request entered driver", answer, err, calls)
	}
}
