package main

import (
	"context"
	"errors"
	"strings"
)

var ErrLoopBudget = errors.New("mcp agent: loop budget exhausted")

type LoopCall struct{ ID, Tool, DocumentID string }
type LoopReply struct {
	Final string
	Call  LoopCall
}
type LoopMessage struct {
	Role, Content, CallID, Tool, DocumentID string
	Failed                                  bool
}
type LoopReport struct {
	Final                           string
	Rounds, ToolCalls, ToolFailures int
	History                         []LoopMessage
}
type Decision func(context.Context, []LoopMessage) (LoopReply, error)

// RunLoop binds a trusted initialized/discovered Client to the decision loop.
// The model callback sees value snapshots, never Client permissions or process configuration.
func RunLoop(ctx context.Context, c *Client, question string, decide Decision) (LoopReport, error) {
	if c == nil || !c.ready || c.closed {
		return LoopReport{}, ErrProtocol
	}
	return runLoop(ctx, question, decide, func(ctx context.Context, call LoopCall) (string, error) {
		return c.Call(ctx, call.Tool, map[string]any{"id": call.DocumentID})
	})
}

func RunRoutedLoop(ctx context.Context, router *ToolRouter, question string, decide Decision) (LoopReport, error) {
	if router == nil || router.routes == nil {
		return LoopReport{}, ErrProtocol
	}
	return runLoop(ctx, question, decide, func(ctx context.Context, call LoopCall) (string, error) {
		return router.Call(ctx, call.Tool, map[string]any{"id": call.DocumentID})
	})
}

func runLoop(ctx context.Context, question string, decide Decision, execute func(context.Context, LoopCall) (string, error)) (LoopReport, error) {
	r := LoopReport{}
	if question == "" || len(question) > 4096 || decide == nil {
		return r, ErrProtocol
	}
	r.History = []LoopMessage{{Role: "user", Content: question}}
	seen := map[string]bool{}
	for r.Rounds < 4 {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		r.Rounds++
		reply, err := decide(ctx, append([]LoopMessage(nil), r.History...))
		if stop := ctx.Err(); stop != nil {
			return r, stop
		}
		if err != nil {
			return r, err
		}
		call := reply.Call
		if call.ID == "" {
			if call.Tool != "" || call.DocumentID != "" || strings.TrimSpace(reply.Final) == "" || len(reply.Final) > 4096 {
				return r, ErrProtocol
			}
			r.History = append(r.History, LoopMessage{Role: "assistant", Content: reply.Final})
			r.Final = reply.Final
			return r, nil
		}
		if reply.Final != "" || len(call.ID) > 128 || call.Tool == "" || len(call.Tool) > 256 || call.DocumentID == "" || len(call.DocumentID) > 128 || seen[call.ID] {
			return r, ErrProtocol
		}
		if r.ToolCalls >= 2 {
			return r, ErrLoopBudget
		}
		seen[call.ID] = true
		r.History = append(r.History, LoopMessage{Role: "assistant", CallID: call.ID, Tool: call.Tool, DocumentID: call.DocumentID})
		r.ToolCalls++
		text, callErr := execute(ctx, call)
		if stop := ctx.Err(); stop != nil {
			return r, stop
		}
		observation := LoopMessage{Role: "tool", CallID: call.ID, Tool: call.Tool}
		switch {
		case callErr == nil:
			observation.Content = text
		case errors.Is(callErr, ErrTool):
			observation.Failed = true
			observation.Content = "tool_failed"
		case errors.Is(callErr, ErrDenied):
			observation.Failed = true
			observation.Content = "not_authorized"
		default:
			return r, callErr // Protocol, transport and cancellation are not normal evidence.
		}
		if observation.Failed {
			r.ToolFailures++
		}
		r.History = append(r.History, observation)
	}
	return r, ErrLoopBudget
}
func scriptedDecision(toolName string) Decision {
	return func(ctx context.Context, history []LoopMessage) (LoopReply, error) {
		if len(history) == 1 {
			return LoopReply{Call: LoopCall{ID: "read-1", Tool: toolName, DocumentID: "doc-A"}}, nil
		}
		last := history[len(history)-1]
		if last.Role != "tool" || last.CallID != "read-1" {
			return LoopReply{}, ErrProtocol
		}
		if last.Failed {
			return LoopReply{Final: "No usable document evidence."}, nil
		}
		return LoopReply{Final: last.Content}, nil
	}
}
