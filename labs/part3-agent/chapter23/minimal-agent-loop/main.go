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

// Session 保存会话消息，支持后续续聊；它不是中断执行的 checkpoint。
// Messages 由所有者管理，Session 不支持并发读写。
type Session struct {
	Messages []Message
}

func (s *Session) Add(m Message) {
	s.Messages = append(s.Messages, cloneMessage(m))
}

// History 复制消息和 JSON 形状的嵌套参数，不共享 map[string]any 或 []any。
func (s *Session) History() []Message {
	history := make([]Message, len(s.Messages))
	for i, m := range s.Messages {
		history[i] = cloneMessage(m)
	}
	return history
}

// 参数限于 JSON 值：标量、map[string]any、[]any；不承诺复制任意 Go 对象。
func cloneJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		copy := make(map[string]any, len(x))
		for k, v := range x {
			copy[k] = cloneJSON(v)
		}
		return copy
	case []any:
		copy := make([]any, len(x))
		for i, v := range x {
			copy[i] = cloneJSON(v)
		}
		return copy
	default:
		return v
	}
}

func cloneMessage(m Message) Message {
	if m.ToolCalls != nil {
		calls := make([]ToolCall, len(m.ToolCalls))
		for i, call := range m.ToolCalls {
			calls[i] = call
			if call.Args != nil {
				calls[i].Args = cloneJSON(call.Args).(map[string]any)
			}
		}
		m.ToolCalls = calls
	}
	return m
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

// Run 推进决策，工具定义与参数校验由执行层负责。
// 顺序：校验上限 → 记录用户输入 → 检查context → 模型响应分支
// → 记录提议 → 串行执行并配对观察 → 下一轮或终止。
// MaxSteps计模型轮次；一轮可执行多个工具，超限不撤销已完成动作。
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

		if err := ctx.Err(); err != nil {
			return Result{}, err
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
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Result{}, ctxErr
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return Result{}, err
			}
			content := observation
			if err != nil {
				// 非取消类工具失败序列化成 observation 回填，
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
	// 第一轮提出两个调用，第二轮给出预写Final。
	// 历史共五条：user + assistant提议 + 两条tool观察 + assistant答案。
	// 固定Final不证明观察回填正确，测试还须断言下一轮的模型输入。
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
