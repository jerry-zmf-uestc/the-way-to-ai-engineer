package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Message 是会话历史的最小单元。
// Role 取值："user"（用户输入）、"assistant"（模型输出，可携带 ToolCalls）、
// "tool"（一次工具执行的结果，ToolCallID 指回对应的调用）。
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// ToolCall 是模型提议的一次工具调用：只有意图，没有任何执行效果。
// Args 来自模型输出，不可信，执行前需要校验（见 tool-calling lab）。
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// Response 是模型一轮的输出：要么是要执行的工具调用，要么是最终答案。
type Response struct {
	ToolCalls []ToolCall
	Final     string
}

// Model 抽象掉具体 Provider：输入完整历史，输出一轮响应。
// 测试用 ScriptedModel 固定输出，避免把概率波动当成回归。
type Model interface {
	Next(ctx context.Context, history []Message) (Response, error)
}

var ErrUnknownTool = errors.New("agent loop: unknown tool")

// ToolFunc 是工具的执行体；本 lab 聚焦 loop，参数形状从简，由 ToolSet 统一查表。
type ToolFunc func(ctx context.Context, args map[string]any) (string, error)

// ToolSet 是最小执行器：权限边界就是这张表，表外的名字一律拒绝。
type ToolSet map[string]ToolFunc

func (t ToolSet) Execute(ctx context.Context, call ToolCall) (string, error) {
	fn, ok := t[call.Name]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownTool, call.Name)
	}
	return fn(ctx, call.Args)
}

// Session 累积一次会话的全部消息。它是 loop 的唯一状态：
// 保存下来（或序列化）就可以在之后的运行中继续。
type Session struct {
	Messages []Message
}

func (s *Session) Add(m Message) {
	s.Messages = append(s.Messages, m)
}

// History 返回副本，外部修改不会写回内部历史。
func (s *Session) History() []Message {
	return append([]Message(nil), s.Messages...)
}

var (
	ErrMaxStepsExceeded = errors.New("agent loop: max steps exceeded")
	ErrEmptyModelReply  = errors.New("agent loop: model returned neither tool calls nor a final answer")
	ErrInvalidMaxSteps  = errors.New("agent loop: max steps must be positive")
)

type Result struct {
	Answer string
	Steps  int
}

type Agent struct {
	Model    Model
	Tools    ToolSet
	MaxSteps int
}

// Run 是最小 agent loop：model → tool → observation → model，
// 直到模型给出最终答案，或触达步数上限、超时、取消。
func (a *Agent) Run(ctx context.Context, session *Session, userInput string) (Result, error) {
	if a.MaxSteps <= 0 {
		return Result{}, ErrInvalidMaxSteps
	}
	session.Add(Message{Role: "user", Content: userInput})

	for step := 1; step <= a.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}

		resp, err := a.Model.Next(ctx, session.History())
		if err != nil {
			return Result{}, fmt.Errorf("agent loop: model: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			if resp.Final == "" {
				return Result{}, ErrEmptyModelReply
			}
			session.Add(Message{Role: "assistant", Content: resp.Final})
			return Result{Answer: resp.Final, Steps: step}, nil
		}

		session.Add(Message{Role: "assistant", ToolCalls: resp.ToolCalls})
		for _, call := range resp.ToolCalls {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			observation, err := a.Tools.Execute(ctx, call)
			content := observation
			if err != nil {
				// 工具失败不中断 loop：错误序列化成 observation 回填，
				// 让模型自己看到失败并决定重试、换工具或放弃。
				content = "error: " + err.Error()
			}
			session.Add(Message{Role: "tool", ToolCallID: call.ID, Content: content})
		}
	}
	return Result{}, fmt.Errorf("%w: %d", ErrMaxStepsExceeded, a.MaxSteps)
}

// ScriptedModel 按脚本化的响应序列回复，Calls 记录被调用的次数，
// Hook 可在每次调用时触发副作用（例如取消 context）。
type ScriptedModel struct {
	Responses   []Response
	Calls       int
	Hook        func(call int)
	LastHistory []Message
}

func (m *ScriptedModel) Next(_ context.Context, history []Message) (Response, error) {
	m.Calls++
	m.LastHistory = history
	if m.Hook != nil {
		m.Hook(m.Calls)
	}
	if m.Calls > len(m.Responses) {
		return Response{}, errors.New("scripted model: script exhausted")
	}
	return m.Responses[m.Calls-1], nil
}

func addTool(_ context.Context, args map[string]any) (string, error) {
	a, okA := args["a"].(float64)
	b, okB := args["b"].(float64)
	if !okA || !okB {
		return "", errors.New("add: arguments a and b must be numbers")
	}
	return strconv.FormatFloat(a+b, 'g', -1, 64), nil
}

func runDemo(ctx context.Context) (string, error) {
	model := &ScriptedModel{Responses: []Response{
		{ToolCalls: []ToolCall{
			{ID: "c1", Name: "add", Args: map[string]any{"a": 6.0, "b": 7.0}},
			{ID: "c2", Name: "get_time", Args: map[string]any{}},
		}},
		{Final: "6+7=13；当前时间 2026-01-01T08:00:00Z"},
	}}
	agent := &Agent{
		Model: model,
		Tools: ToolSet{
			"add": addTool,
			"get_time": func(_ context.Context, _ map[string]any) (string, error) {
				return time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC).Format(time.RFC3339), nil
			},
		},
		MaxSteps: 4,
	}

	session := &Session{}
	result, err := agent.Run(ctx, session, "6+7 等于几？现在几点？")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("answer=%q steps=%d messages=%d", result.Answer, result.Steps, len(session.Messages)), nil
}

func main() {
	out, err := runDemo(context.Background())
	if err != nil {
		fmt.Println("run demo:", err)
		return
	}
	fmt.Println(out)
}
