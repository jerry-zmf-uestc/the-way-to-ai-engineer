package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const validJSON = `{"answer": "循环变量每轮新建", "confidence": 0.9, "sentiment": "positive", "citations": ["go.dev/doc/go1.22"]}`

func TestParseAnswerValid(t *testing.T) {
	t.Parallel()

	answer, err := parseAnswer(validJSON)
	if err != nil {
		t.Fatalf("parseAnswer() error = %v", err)
	}
	if answer.Answer != "循环变量每轮新建" || answer.Confidence != 0.9 || answer.Sentiment != "positive" {
		t.Fatalf("parseAnswer() = %+v", answer)
	}
	if len(answer.Citations) != 1 || answer.Citations[0] != "go.dev/doc/go1.22" {
		t.Fatalf("Citations = %v", answer.Citations)
	}
}

func TestParseAnswerBadJSON(t *testing.T) {
	t.Parallel()

	_, err := parseAnswer(`{"answer": "oops"`) // 语法错误：未闭合
	var serr *StructuredError
	if !errors.As(err, &serr) {
		t.Fatalf("parseAnswer() error = %v, want *StructuredError", err)
	}
	if serr.Kind != KindSyntax {
		t.Fatalf("Kind = %q, want %q", serr.Kind, KindSyntax)
	}
	if serr.Raw == "" {
		t.Fatal("Raw must keep the original bad output for debugging")
	}
}

func TestParseAnswerSchemaViolations(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"missing field":       `{"answer": "a", "confidence": 0.5, "sentiment": "neutral"}`,
		"wrong type":          `{"answer": "a", "confidence": "high", "sentiment": "neutral", "citations": []}`,
		"confidence range":    `{"answer": "a", "confidence": 1.5, "sentiment": "neutral", "citations": []}`,
		"sentiment enum":      `{"answer": "a", "confidence": 0.5, "sentiment": "happy", "citations": []}`,
		"citation not string": `{"answer": "a", "confidence": 0.5, "sentiment": "neutral", "citations": [42]}`,
		"unknown field":       `{"answer": "a", "confidence": 0.5, "sentiment": "neutral", "citations": [], "mood": "x"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := parseAnswer(raw)
			var serr *StructuredError
			if !errors.As(err, &serr) {
				t.Fatalf("parseAnswer() error = %v, want *StructuredError", err)
			}
			if serr.Kind != KindSchema {
				t.Fatalf("Kind = %q, want %q", serr.Kind, KindSchema)
			}
			if len(serr.Issues) == 0 {
				t.Fatal("Issues must list each violation, not fail silently")
			}
		})
	}
}

func TestGenerateAnswerFirstTry(t *testing.T) {
	t.Parallel()

	provider := &FakeProvider{Outputs: []string{validJSON}}
	result, err := GenerateAnswer(context.Background(), provider, "q")
	if err != nil {
		t.Fatalf("GenerateAnswer() error = %v", err)
	}
	if result.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", result.Attempts)
	}
	if len(provider.Requests) != 1 {
		t.Fatalf("provider called %d times, want 1", len(provider.Requests))
	}
}

func TestGenerateAnswerRetrySucceeds(t *testing.T) {
	t.Parallel()

	provider := &FakeProvider{Outputs: []string{
		`{"answer": "a", "confidence": "high"}`, // schema 违例
		validJSON,
	}}
	result, err := GenerateAnswer(context.Background(), provider, "q")
	if err != nil {
		t.Fatalf("GenerateAnswer() error = %v", err)
	}
	if result.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2", result.Attempts)
	}
	if len(provider.Requests) != 2 {
		t.Fatalf("provider called %d times, want 2", len(provider.Requests))
	}

	// 重试必须带反馈：第二轮消息 = 原始 2 条 + 坏输出 + 问题清单
	retry := provider.Requests[1]
	if len(retry) != 4 {
		t.Fatalf("retry request has %d messages, want 4", len(retry))
	}
	if retry[2].Role != "assistant" || !strings.Contains(retry[2].Content, `"confidence": "high"`) {
		t.Fatalf("retry[2] = %+v, want the bad output echoed back", retry[2])
	}
	if retry[3].Role != "user" || !strings.Contains(retry[3].Content, "confidence") {
		t.Fatalf("retry[3] = %+v, want validation feedback", retry[3])
	}
}

func TestGenerateAnswerGivesUp(t *testing.T) {
	t.Parallel()

	provider := &FakeProvider{Outputs: []string{
		`not json at all`,
		`{"answer": ""}`,
	}}
	_, err := GenerateAnswer(context.Background(), provider, "q")
	if err == nil {
		t.Fatal("GenerateAnswer() error = nil, want visible failure after retries")
	}
	var serr *StructuredError
	if !errors.As(err, &serr) {
		t.Fatalf("GenerateAnswer() error = %v, want wrapped *StructuredError", err)
	}
	if !strings.Contains(err.Error(), "giving up after 2 attempts") {
		t.Fatalf("error = %q, want attempt count visible", err)
	}
	if len(provider.Requests) != 2 {
		t.Fatalf("provider called %d times, want 2 (no unbounded retry)", len(provider.Requests))
	}
}

type failingProvider struct{}

func (failingProvider) Generate(_ context.Context, _ []Message) (string, error) {
	return "", errors.New("structured-output: provider unavailable")
}

func TestGenerateAnswerProviderErrorNotRetried(t *testing.T) {
	t.Parallel()

	_, err := GenerateAnswer(context.Background(), failingProvider{}, "q")
	if err == nil || !strings.Contains(err.Error(), "provider unavailable") {
		t.Fatalf("GenerateAnswer() error = %v, want provider error propagated", err)
	}
}

type providerFunc func(context.Context, []Message) (string, error)

func (f providerFunc) Generate(ctx context.Context, m []Message) (string, error) { return f(ctx, m) }
func TestCanceledAndOversizeOutputRejected(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := GenerateAnswer(ctx, providerFunc(func(context.Context, []Message) (string, error) {
		cancel()
		return `{"answer":"late","confidence":1,"sentiment":"neutral","citations":[]}`, nil
	}), "question")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("late response accepted: %v", err)
	}
	calls := 0
	_, err = GenerateAnswer(context.Background(), providerFunc(func(context.Context, []Message) (string, error) { calls++; return strings.Repeat("x", (64<<10)+1), nil }), "question")
	if err == nil || calls != 1 {
		t.Fatalf("oversize output retried: calls=%d err=%v", calls, err)
	}
}
