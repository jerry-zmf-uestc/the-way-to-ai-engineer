// Command hello-llm 演示最小 LLM 调用链路：消息模型、Provider 接口、
// 离线 FakeProvider、OpenAI 兼容 HTTP 适配器，以及 token 与成本记录。
// 默认运行不访问网络；真实 Provider 只在测试显式设置 E2E_LIVE 时启用。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// Role 是消息角色。system 承担稳定规则，user 承担本次请求参数，
// assistant 承载模型的历史回复（多轮对话与重试反馈时回传）。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message 是 Provider 之间交换的最小消息单位。
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Usage 记录一次调用的 token 用量。真实 Provider 由服务端返回，
// fake 只能本地估算——估算值用于观察量级，不能当账单。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// Response 是一次生成的结果：文本加上 token 用量。
type Response struct {
	Text  string
	Usage Usage
}

// Provider 是模型调用的抽象边界。先写 fake 再写 HTTP 适配器，
// 调用方代码在两种实现之间不需要改变。
type Provider interface {
	Generate(ctx context.Context, messages []Message) (Response, error)
}

// estimateTokens 用「约 4 个字符 1 个 token」的经验法则粗估，
// 只用于演示成本记录，真实计费以 Provider 返回的 usage 为准。
func estimateTokens(s string) int {
	return utf8.RuneCountInString(s)/4 + 1
}

// FakeProvider 是确定性的离线实现：按输入规则回复，并估算 token 用量。
// 相同输入永远产生相同输出，测试可以断言具体内容。
type FakeProvider struct{}

var _ Provider = FakeProvider{}

func (FakeProvider) Generate(_ context.Context, messages []Message) (Response, error) {
	promptTokens := 0
	lastUser := ""
	for _, m := range messages {
		promptTokens += estimateTokens(m.Content)
		if m.Role == RoleUser {
			lastUser = m.Content // 取最后一条 user 消息作为本次请求参数
		}
	}
	if lastUser == "" {
		return Response{}, errors.New("hello-llm: no user message")
	}

	reply := "fake reply: " + lastUser
	if strings.Contains(lastUser, "ping") {
		reply = "pong"
	}
	return Response{
		Text: reply,
		Usage: Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: estimateTokens(reply),
		},
	}, nil
}

// 示例价格（美元 / 1M token），仅为演示成本记录而硬编码，
// 不代表任何真实 Provider 的报价。
const (
	examplePromptPricePerMillion     = 0.15
	exampleCompletionPricePerMillion = 0.60
)

// estimateCost 按示例价格表把 token 用量折算成美元。
func estimateCost(u Usage) float64 {
	return float64(u.PromptTokens)/1e6*examplePromptPricePerMillion +
		float64(u.CompletionTokens)/1e6*exampleCompletionPricePerMillion
}

// HTTPError 表示 Provider 返回的非 200 状态。调用方可以按 StatusCode
// 区分限流（429）、服务端故障（5xx）等语义，决定重试还是放弃。
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("hello-llm: provider returned HTTP %d: %s", e.StatusCode, e.Message)
}

// OpenAICompatibleProvider 走 OpenAI 风格的 POST {BaseURL}/chat/completions。
// BaseURL 由部署方配置（allowlist），本例不接受运行时任意 URL 之外的校验。
type OpenAICompatibleProvider struct {
	BaseURL  string
	APIKey   string // 只从环境变量注入，不写入日志与错误信息
	Model    string
	Client   *http.Client // nil 时使用带 30s 超时的默认客户端
	MaxBytes int64        // 响应体上限，<= 0 时默认 1 MiB
}

var _ Provider = OpenAICompatibleProvider{}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p OpenAICompatibleProvider) Generate(ctx context.Context, messages []Message) (Response, error) {
	if len(messages) == 0 {
		return Response{}, errors.New("hello-llm: empty messages")
	}

	body, err := json.Marshal(chatRequest{Model: p.Model, Messages: messages})
	if err != nil {
		return Response{}, fmt.Errorf("hello-llm: encode request: %w", err)
	}

	url := strings.TrimSuffix(p.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("hello-llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("hello-llm: request: %w", err)
	}
	defer resp.Body.Close()

	maxBytes := p.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return Response{}, fmt.Errorf("hello-llm: read response: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return Response{}, fmt.Errorf("hello-llm: response exceeds %d bytes", maxBytes)
	}

	var parsed chatResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return Response{}, fmt.Errorf("hello-llm: decode response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := http.StatusText(resp.StatusCode)
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		return Response{}, &HTTPError{StatusCode: resp.StatusCode, Message: msg}
	}
	if len(parsed.Choices) == 0 {
		return Response{}, errors.New("hello-llm: response has no choices")
	}

	return Response{
		Text: parsed.Choices[0].Message.Content,
		Usage: Usage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
		},
	}, nil
}

func main() {
	messages := []Message{
		{Role: RoleSystem, Content: "You are a concise assistant."},
		{Role: RoleUser, Content: "ping"},
	}

	resp, err := (FakeProvider{}).Generate(context.Background(), messages)
	if err != nil {
		fmt.Println("generate:", err)
		return
	}

	fmt.Println("reply:", resp.Text)
	fmt.Printf("tokens: prompt=%d completion=%d\n", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	fmt.Printf("estimated cost: $%.8f (example price, not a real bill)\n", estimateCost(resp.Usage))
}
