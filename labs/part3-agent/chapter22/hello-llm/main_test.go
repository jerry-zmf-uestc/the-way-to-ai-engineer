package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFakeProviderDeterministic(t *testing.T) {
	t.Parallel()

	messages := []Message{
		{Role: RoleSystem, Content: "You are a concise assistant."},
		{Role: RoleUser, Content: "ping"},
	}
	provider := FakeProvider{}

	first, err := provider.Generate(context.Background(), messages)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	second, err := provider.Generate(context.Background(), messages)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if first != second {
		t.Fatalf("Generate() not deterministic: %+v vs %+v", first, second)
	}
	if first.Text != "pong" {
		t.Fatalf("Text = %q, want %q", first.Text, "pong")
	}
	if first.Usage.PromptTokens == 0 || first.Usage.CompletionTokens == 0 {
		t.Fatalf("Usage = %+v, want both token counts recorded", first.Usage)
	}
}

func TestFakeProviderRequiresUserMessage(t *testing.T) {
	t.Parallel()

	_, err := (FakeProvider{}).Generate(context.Background(), []Message{
		{Role: RoleSystem, Content: "rules only"},
	})
	if err == nil || !strings.Contains(err.Error(), "no user message") {
		t.Fatalf("Generate() error = %v, want no-user-message error", err)
	}
}

func TestOpenAICompatibleProviderOK(t *testing.T) {
	t.Parallel()

	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "hello from server"}}],
			"usage": {"prompt_tokens": 11, "completion_tokens": 7}
		}`))
	}))
	defer server.Close()

	provider := OpenAICompatibleProvider{
		BaseURL: server.URL,
		APIKey:  "test-key-not-a-secret",
		Model:   "fake-model",
	}
	resp, err := provider.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text != "hello from server" {
		t.Fatalf("Text = %q", resp.Text)
	}
	if resp.Usage.PromptTokens != 11 || resp.Usage.CompletionTokens != 7 {
		t.Fatalf("Usage = %+v, want {11 7}", resp.Usage)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("request path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer test-key-not-a-secret" {
		t.Fatalf("Authorization = %q, want bearer header", gotAuth)
	}
}

func TestOpenAICompatibleProviderHTTP500(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "model overloaded"},
		})
	}))
	defer server.Close()

	provider := OpenAICompatibleProvider{BaseURL: server.URL, Model: "fake-model"}
	_, err := provider.Generate(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("Generate() error = %v, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want 500", httpErr.StatusCode)
	}
	if !strings.Contains(httpErr.Message, "model overloaded") {
		t.Fatalf("Message = %q, want provider error body", httpErr.Message)
	}
}

func TestOpenAICompatibleProviderTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond) // 慢于客户端超时
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := OpenAICompatibleProvider{
		BaseURL: server.URL,
		Model:   "fake-model",
		Client:  &http.Client{Timeout: 50 * time.Millisecond},
	}
	_, err := provider.Generate(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	if err == nil {
		t.Fatal("Generate() error = nil, want timeout error")
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		t.Fatalf("Generate() error = %v, timeout must not surface as HTTPError", err)
	}
}

// TestE2ELiveProvider 是唯一访问真实 Provider 的测试，默认跳过。
// 需要同时显式设置 E2E_LIVE=1、LLM_BASE_URL 和 LLM_API_KEY；
// 它不进默认测试与 CI，避免意外产生真实费用。
func TestE2ELiveProvider(t *testing.T) {
	t.Parallel()

	if os.Getenv("E2E_LIVE") != "1" {
		t.Skip("set E2E_LIVE=1 with LLM_BASE_URL and LLM_API_KEY to run against a real provider")
	}
	baseURL := os.Getenv("LLM_BASE_URL")
	apiKey := os.Getenv("LLM_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Fatal("E2E_LIVE set but LLM_BASE_URL or LLM_API_KEY missing")
	}

	provider := OpenAICompatibleProvider{BaseURL: baseURL, APIKey: apiKey, Model: os.Getenv("LLM_MODEL")}
	resp, err := provider.Generate(context.Background(), []Message{
		{Role: RoleUser, Content: "Say the word ok."},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp.Text == "" {
		t.Fatal("Generate() returned empty text")
	}
}

func TestProviderResponseBoundaries(t *testing.T) {
	for _, c := range []struct {
		name       string
		status     int
		body       string
		limit      int64
		httpStatus int
	}{
		{"non JSON rate limit", 429, "slow down", 0, 429},
		{"non JSON gateway", 502, "<html>gateway error</html>", 0, 502},
		{"invalid success JSON", 200, "not json", 0, 0},
		{"no choices", 200, `{"choices":[]}`, 0, 0},
		{"truncated", 200, `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`, 0, 0},
		{"empty text", 200, `{"choices":[{"message":{"content":" "}}]}`, 0, 0},
		{"oversize", 200, `{"choices":[{"message":{"content":"large"}}]}`, 4, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(c.status); fmt.Fprint(w, c.body) }))
			defer server.Close()
			_, err := (OpenAICompatibleProvider{BaseURL: server.URL, Model: "fake", MaxBytes: c.limit}).Generate(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
			if err == nil {
				t.Fatal("bad response accepted")
			}
			var he *HTTPError
			if c.httpStatus != 0 {
				if !errors.As(err, &he) || he.StatusCode != c.httpStatus {
					t.Fatalf("lost HTTP status: %v", err)
				}
			} else if errors.As(err, &he) {
				t.Fatalf("protocol error became HTTP error: %v", err)
			}
		})
	}
}
func TestFakeHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (FakeProvider{}).Generate(ctx, []Message{{Role: RoleUser, Content: "ping"}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}
