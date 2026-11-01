package main

import (
	"context"
	"errors"
	"testing"
)

func readyClient(t *testing.T, approved map[string]bool) *Client {
	t.Helper()
	c := process(t, approved)
	if err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c
}
func TestLoopUsesRealStdioResultAndCallAssociation(t *testing.T) {
	c := readyClient(t, map[string]bool{"read_doc": true})
	before := c.seq
	report, err := RunLoop(context.Background(), c, "What is timeout?", scriptedDecision("docs::read_doc"))
	if err != nil || report.Rounds != 2 || report.ToolCalls != 1 || report.ToolFailures != 0 || report.Final != "Timeout is 30 seconds." || c.seq != before+1 {
		t.Fatal(report, err)
	}
	if len(report.History) != 4 || report.History[1].CallID != "read-1" || report.History[2].CallID != "read-1" || report.History[2].Role != "tool" {
		t.Fatal(report.History)
	}
}
func TestLoopFeedsToolFailureBeforeBoundedRecovery(t *testing.T) {
	c := readyClient(t, map[string]bool{"read_doc": true})
	decisions := 0
	decide := func(ctx context.Context, history []LoopMessage) (LoopReply, error) {
		decisions++
		switch decisions {
		case 1:
			return LoopReply{Call: LoopCall{ID: "first", Tool: "docs::read_doc", DocumentID: "missing"}}, nil
		case 2:
			if len(history) != 3 || !history[2].Failed || history[2].Content != "tool_failed" {
				t.Fatal(history)
			}
			history[0].Content = "attempted mutation"
			return LoopReply{Call: LoopCall{ID: "retry", Tool: "docs::read_doc", DocumentID: "doc-A"}}, nil
		default:
			if history[0].Content != "question" || history[4].Failed || history[4].CallID != "retry" {
				t.Fatal(history)
			}
			return LoopReply{Final: history[4].Content}, nil
		}
	}
	report, err := RunLoop(context.Background(), c, "question", decide)
	if err != nil || report.ToolFailures != 1 || report.ToolCalls != 2 || report.Rounds != 3 || report.Final != "Timeout is 30 seconds." {
		t.Fatal(report, err)
	}
}
func TestLoopDeniedCapabilityDoesNotSendRPC(t *testing.T) {
	c := readyClient(t, map[string]bool{})
	before := c.seq
	report, err := RunLoop(context.Background(), c, "question", scriptedDecision("docs::read_doc"))
	if err != nil || report.ToolFailures != 1 || c.seq != before || report.History[2].Content != "not_authorized" {
		t.Fatal(report, err)
	}
	// A graceful Final after refusal is still not proof that the requested document was read.
	if report.History[2].Failed != true {
		t.Fatal("denial hidden by Final")
	}
}
func TestLoopCancellationAndMalformedReplyStop(t *testing.T) {
	c := readyClient(t, map[string]bool{"read_doc": true})
	ctx, cancel := context.WithCancel(context.Background())
	report, err := RunLoop(ctx, c, "question", func(context.Context, []LoopMessage) (LoopReply, error) {
		cancel()
		return LoopReply{Final: "late"}, nil
	})
	if !errors.Is(err, context.Canceled) || report.Final != "" || len(report.History) != 1 {
		t.Fatal(report, err)
	}
	_, err = RunLoop(context.Background(), c, "question", func(context.Context, []LoopMessage) (LoopReply, error) {
		return LoopReply{Final: "answer", Call: LoopCall{ID: "x", Tool: "docs::read_doc", DocumentID: "doc-A"}}, nil
	})
	if !errors.Is(err, ErrProtocol) {
		t.Fatal(err)
	}
}
func TestLoopBudgetsAndDuplicateIDs(t *testing.T) {
	for _, duplicate := range []bool{true, false} {
		c := readyClient(t, map[string]bool{"read_doc": true})
		n := 0
		_, err := RunLoop(context.Background(), c, "question", func(context.Context, []LoopMessage) (LoopReply, error) {
			n++
			id := "same"
			if !duplicate {
				id = string(rune('a' + n))
			}
			return LoopReply{Call: LoopCall{ID: id, Tool: "docs::read_doc", DocumentID: "doc-A"}}, nil
		})
		expected := ErrLoopBudget
		if duplicate {
			expected = ErrProtocol
		}
		if !errors.Is(err, expected) {
			t.Fatal(err)
		}
	}
}
