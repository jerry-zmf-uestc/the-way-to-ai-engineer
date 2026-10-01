package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func sumToolSet() ToolSet {
	return ToolSet{"add": addTool}
}

func TestRunCompletesWithToolCall(t *testing.T) {
	t.Parallel()

	model := &ScriptedModel{Responses: []Response{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "add", Args: map[string]any{"a": 1.0, "b": 2.0}}}},
		{Final: "1+2=3"},
	}}
	agent := &Agent{Model: model, Tools: sumToolSet(), MaxSteps: 4}
	session := &Session{}

	result, err := agent.Run(context.Background(), session, "1+2=?")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Answer != "1+2=3" || result.Steps != 2 {
		t.Fatalf("Run() = %+v, want answer %q in 2 steps", result, "1+2=3")
	}
	if model.Calls != 2 {
		t.Fatalf("model calls = %d, want 2", model.Calls)
	}

	// 会话历史应完整记录：user → assistant(提议) → tool(observation) → assistant(答案)。
	roles := make([]string, 0, len(session.Messages))
	for _, m := range session.Messages {
		roles = append(roles, m.Role)
	}
	wantRoles := []string{"user", "assistant", "tool", "assistant"}
	if strings.Join(roles, ",") != strings.Join(wantRoles, ",") {
		t.Fatalf("message roles = %v, want %v", roles, wantRoles)
	}
	observation := session.Messages[2]
	if observation.ToolCallID != "c1" || observation.Content != "3" {
		t.Fatalf("observation = %+v, want tool result 3 for call c1", observation)
	}
}

func TestMaxStepsExceeded(t *testing.T) {
	t.Parallel()

	// 模型每轮都提议新工具调用，永远不给最终答案。
	model := &ScriptedModel{Responses: []Response{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "add", Args: map[string]any{"a": 1.0, "b": 1.0}}}},
		{ToolCalls: []ToolCall{{ID: "c2", Name: "add", Args: map[string]any{"a": 2.0, "b": 2.0}}}},
		{ToolCalls: []ToolCall{{ID: "c3", Name: "add", Args: map[string]any{"a": 3.0, "b": 3.0}}}},
	}}
	agent := &Agent{Model: model, Tools: sumToolSet(), MaxSteps: 3}
	session := &Session{}

	_, err := agent.Run(context.Background(), session, "keep going")
	if !errors.Is(err, ErrMaxStepsExceeded) {
		t.Fatalf("Run() error = %v, want ErrMaxStepsExceeded", err)
	}
	if model.Calls != 3 {
		t.Fatalf("model calls = %d, want exactly MaxSteps (3)", model.Calls)
	}
}

func TestRunTimeout(t *testing.T) {
	t.Parallel()

	model := &ScriptedModel{Responses: []Response{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "slow", Args: map[string]any{}}}},
		{Final: "unreachable"},
	}}
	tools := ToolSet{"slow": func(ctx context.Context, _ map[string]any) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Second):
			return "done", nil
		}
	}}
	agent := &Agent{Model: model, Tools: tools, MaxSteps: 8}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := agent.Run(ctx, &Session{}, "slow question")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run() took %v, timeout did not stop the loop", elapsed)
	}
}

func TestRunCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := &ScriptedModel{
		Responses: []Response{
			{ToolCalls: []ToolCall{{ID: "c1", Name: "add", Args: map[string]any{"a": 1.0, "b": 2.0}}}},
			{Final: "unreachable"},
		},
		Hook: func(call int) {
			if call == 1 {
				cancel() // 第一轮模型响应后由外部取消
			}
		},
	}
	agent := &Agent{Model: model, Tools: sumToolSet(), MaxSteps: 8}

	_, err := agent.Run(ctx, &Session{}, "cancel me")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if model.Calls != 1 {
		t.Fatalf("model calls = %d, want 1: loop must stop after cancellation", model.Calls)
	}
}

func TestToolFailureFedBackAsObservation(t *testing.T) {
	t.Parallel()

	model := &ScriptedModel{Responses: []Response{
		{ToolCalls: []ToolCall{
			{ID: "c1", Name: "explode", Args: map[string]any{}},
			{ID: "c2", Name: "missing", Args: map[string]any{}},
		}},
		{Final: "工具失败，已如实报告"},
	}}
	tools := ToolSet{
		"explode": func(_ context.Context, _ map[string]any) (string, error) {
			return "", errors.New("explode: boom")
		},
	}
	agent := &Agent{Model: model, Tools: tools, MaxSteps: 4}
	session := &Session{}

	result, err := agent.Run(context.Background(), session, "试试会失败的工具")
	if err != nil {
		t.Fatalf("Run() error = %v, tool failure must not abort the loop", err)
	}
	if result.Answer != "工具失败，已如实报告" {
		t.Fatalf("Run() answer = %q", result.Answer)
	}

	// 两类失败（handler 报错、未知工具）都要作为 observation 进入历史，
	// 模型在下一轮能看到它们。
	var observations []string
	for _, m := range model.LastHistory {
		if m.Role == "tool" {
			observations = append(observations, m.Content)
		}
	}
	if len(observations) != 2 {
		t.Fatalf("observations = %v, want 2", observations)
	}
	if !strings.Contains(observations[0], "error: explode: boom") {
		t.Fatalf("observation[0] = %q, want handler error fed back", observations[0])
	}
	if !strings.Contains(observations[1], "unknown tool") {
		t.Fatalf("observation[1] = %q, want unknown-tool rejection fed back", observations[1])
	}
}

func TestInvalidMaxStepsRejected(t *testing.T) {
	t.Parallel()

	agent := &Agent{Model: &ScriptedModel{}, Tools: sumToolSet(), MaxSteps: 0}
	if _, err := agent.Run(context.Background(), &Session{}, "hi"); !errors.Is(err, ErrInvalidMaxSteps) {
		t.Fatalf("Run() error = %v, want ErrInvalidMaxSteps", err)
	}
}

func TestSessionHistoryReturnsCopy(t *testing.T) {
	t.Parallel()

	session := &Session{}
	session.Add(Message{Role: "user", Content: "hi"})
	history := session.History()
	history[0].Content = "tampered"
	if session.Messages[0].Content != "hi" {
		t.Fatalf("session mutated through History() copy: %q", session.Messages[0].Content)
	}
}

func TestRunDemoOutput(t *testing.T) {
	t.Parallel()

	out, err := runDemo(context.Background())
	if err != nil {
		t.Fatalf("runDemo() error = %v", err)
	}
	for _, want := range []string{"answer=", "steps=2", "messages=5"} {
		if !strings.Contains(out, want) {
			t.Fatalf("runDemo() missing %q in %q", want, out)
		}
	}
}
