// Command structured-output 演示结构化输出的完整闭环：要求模型返回 JSON、
// 用最小 schema 校验（字段存在、类型、枚举、取值范围）、坏输出产生可见的
// 结构化错误，并把错误反馈给 Provider 重试一次。全程离线，不需要模型密钥。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Message 与 Provider 是本 lab 自包含的最小消息模型，
// 与 hello-llm 形状一致但只保留文本，便于专注结构化输出本身。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Provider interface {
	Generate(ctx context.Context, messages []Message) (string, error)
}

// Answer 是期望模型返回的结构：答案、置信度、情感枚举与引用列表。
type Answer struct {
	Answer     string   `json:"answer"`
	Confidence float64  `json:"confidence"`
	Sentiment  string   `json:"sentiment"`
	Citations  []string `json:"citations"`
}

var allowedSentiments = map[string]bool{
	"positive": true,
	"neutral":  true,
	"negative": true,
}

// ErrorKind 区分两类坏输出：JSON 语法错误与 schema 违例。
// 两者的恢复策略不同，必须分开，不能都揉成「解析失败」。
type ErrorKind string

const (
	KindSyntax ErrorKind = "invalid_json"
	KindSchema ErrorKind = "schema_violation"
)

// StructuredError 是可见的结构化失败：保留原始输出与逐项问题，
// 调用方可以记录、展示或反馈给模型重试，而不是吞掉返回零值。
type StructuredError struct {
	Kind   ErrorKind
	Issues []string
	Raw    string
}

func (e *StructuredError) Error() string {
	return fmt.Sprintf("structured-output: %s: %s", e.Kind, strings.Join(e.Issues, "; "))
}

// parseAnswer 先验证 JSON 语法，再逐项校验 schema。
// 故意先解析进 map[string]any：直接 Unmarshal 进结构体时，
// 「类型不对」和「字段缺失」很难分开报告，而这里需要逐项问题清单。
func parseAnswer(raw string) (Answer, error) {
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return Answer{}, &StructuredError{
			Kind:   KindSyntax,
			Issues: []string{err.Error()},
			Raw:    raw,
		}
	}

	var issues []string
	var out Answer

	answer, ok := fields["answer"].(string)
	if !ok || strings.TrimSpace(answer) == "" {
		issues = append(issues, "field \"answer\": required non-empty string")
	} else {
		out.Answer = answer
	}

	confidence, ok := fields["confidence"].(float64)
	if !ok {
		issues = append(issues, "field \"confidence\": required number")
	} else if confidence < 0 || confidence > 1 {
		issues = append(issues, "field \"confidence\": must be in [0, 1]")
	} else {
		out.Confidence = confidence
	}

	sentiment, ok := fields["sentiment"].(string)
	if !ok || !allowedSentiments[sentiment] {
		issues = append(issues, "field \"sentiment\": must be one of positive|neutral|negative")
	} else {
		out.Sentiment = sentiment
	}

	citations, ok := fields["citations"].([]any)
	if !ok {
		issues = append(issues, "field \"citations\": required array of strings")
	} else {
		for i, c := range citations {
			s, ok := c.(string)
			if !ok {
				issues = append(issues, fmt.Sprintf("field \"citations\"[%d]: must be a string", i))
				continue
			}
			out.Citations = append(out.Citations, s)
		}
	}

	for key := range fields {
		switch key {
		case "answer", "confidence", "sentiment", "citations":
		default:
			issues = append(issues, fmt.Sprintf("unknown field %q", key))
		}
	}

	if len(issues) > 0 {
		return Answer{}, &StructuredError{Kind: KindSchema, Issues: issues, Raw: raw}
	}
	return out, nil
}

const systemPrompt = `You answer questions with JSON only, no prose. Schema:
{"answer": string, "confidence": number in [0,1], "sentiment": "positive"|"neutral"|"negative", "citations": [string]}`

// Result 记录被接受的答案与实际尝试次数，重试必须可观察。
type Result struct {
	Answer   Answer
	Attempts int
}

// FakeProvider 按脚本返回预置输出：Outputs 依次消费，耗尽后重复最后一条。
// 测试用它分别覆盖「一次成功」「坏输出后重试成功」「重试仍失败」三条路径。
type FakeProvider struct {
	Outputs  []string
	Requests [][]Message // 记录每次收到的消息，供测试断言重试反馈
}

var _ Provider = (*FakeProvider)(nil)

func (f *FakeProvider) Generate(_ context.Context, messages []Message) (string, error) {
	if len(f.Outputs) == 0 {
		return "", errors.New("structured-output: fake provider has no scripted output")
	}
	copied := make([]Message, len(messages))
	copy(copied, messages)
	f.Requests = append(f.Requests, copied)

	out := f.Outputs[0]
	if len(f.Outputs) > 1 {
		f.Outputs = f.Outputs[1:]
	}
	return out, nil
}

// GenerateAnswer 调用 Provider 并校验输出。校验失败时把原始输出与
// 问题清单作为新消息回传，最多重试一次；Provider 自身的错误（网络、
// 配额）直接上抛，不参与重试——那是另一层失败语义。
func GenerateAnswer(ctx context.Context, p Provider, question string) (Result, error) {
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}

	const maxAttempts = 2 // 首次调用 + 一次带反馈的重试
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		raw, err := p.Generate(ctx, messages)
		if err != nil {
			return Result{}, fmt.Errorf("structured-output: provider call: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if len(raw) > 64<<10 {
			return Result{}, errors.New("structured-output: response exceeds 64 KiB")
		}
		answer, err := parseAnswer(raw)
		if err == nil {
			return Result{Answer: answer, Attempts: attempt}, nil
		}
		var serr *StructuredError
		if !errors.As(err, &serr) {
			return Result{}, err
		}
		lastErr = serr
		messages = append(messages,
			Message{Role: "assistant", Content: raw},
			Message{Role: "user", Content: "Your output failed validation: " +
				strings.Join(serr.Issues, "; ") + ". Return only valid JSON matching the schema."},
		)
	}
	return Result{}, fmt.Errorf("structured-output: giving up after %d attempts: %w", maxAttempts, lastErr)
}

func main() {
	provider := &FakeProvider{Outputs: []string{
		`{"answer": "循环变量每轮新建", "confidence": "high"}`, // 坏输出：confidence 类型错误
		`{"answer": "循环变量每轮新建", "confidence": 0.9, "sentiment": "positive", "citations": ["go.dev/doc/go1.22"]}`,
	}}

	result, err := GenerateAnswer(context.Background(), provider, "Go 1.22 的循环变量语义是什么？")
	if err != nil {
		fmt.Println("generate answer:", err)
		return
	}

	fmt.Printf("accepted after %d attempt(s)\n", result.Attempts)
	fmt.Println("answer:", result.Answer.Answer)
	fmt.Printf("confidence: %.2f sentiment: %s\n", result.Answer.Confidence, result.Answer.Sentiment)
	fmt.Println("citations:", strings.Join(result.Answer.Citations, ", "))
}
