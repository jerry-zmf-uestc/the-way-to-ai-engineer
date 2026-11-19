package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer() *server {
	srv := newServer()
	srv.agent.tokenDelay = 0
	srv.timeout = 5 * time.Second
	return srv
}

func serveChat(t *testing.T, srv *server, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	srv.mux().ServeHTTP(rec, req)
	return rec
}

func TestChatFullSessionStreamsEvents(t *testing.T) {
	rec := serveChat(t, newTestServer(), "/chat?q=calc+2%2B3*4")

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"event: tool_call",
		`"name":"calculator"`,
		`"args":"2+3*4"`,
		"event: tool_result",
		`"result":"14"`,
		"event: token",
		"event: done",
		`"finish_reason":"stop"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stream missing %q\nfull body:\n%s", want, body)
		}
	}
}

func TestChatDirectAnswerSkipsTools(t *testing.T) {
	rec := serveChat(t, newTestServer(), "/chat?q=hello+there")
	body := rec.Body.String()

	if strings.Contains(body, "event: tool_call") {
		t.Errorf("direct answer must not call tools\nfull body:\n%s", body)
	}
	if !strings.Contains(body, "event: token") || !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Errorf("direct answer must stream tokens and finish with stop\nfull body:\n%s", body)
	}
}

func TestChatUnknownToolRejected(t *testing.T) {
	rec := serveChat(t, newTestServer(), "/chat?q=tool+rocket+launch")
	body := rec.Body.String()

	if strings.Contains(body, "event: tool_result") {
		t.Errorf("unknown tool must never execute\nfull body:\n%s", body)
	}
	for _, want := range []string{
		"event: agent_error",
		`unknown tool \"rocket\"`,
		`"finish_reason":"error"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stream missing %q\nfull body:\n%s", want, body)
		}
	}
}

func TestChatCancelStopsSession(t *testing.T) {
	srv := newServer()
	srv.agent.tokenDelay = 25 * time.Millisecond
	srv.timeout = 30 * time.Second

	req := httptest.NewRequest(http.MethodGet, "/chat?q=hello+there", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		srv.mux().ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond) // let a few tokens stream
	cancel()                           // client pressed the stop button
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after client cancel")
	}
}

func TestRunAgentExitsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := runAgent(ctx, defaultRegistry(), "hello there", agentConfig{tokenDelay: 20 * time.Millisecond, maxTokens: 256})

	if _, ok := <-events; !ok {
		t.Fatal("channel closed before the first event")
	}
	cancel()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // agent goroutine closed the channel and exited
			}
		case <-deadline:
			t.Fatal("agent goroutine did not exit after cancel")
		}
	}
}

func TestChatTimeout(t *testing.T) {
	srv := newServer()
	srv.agent.tokenDelay = 30 * time.Millisecond
	srv.timeout = 100 * time.Millisecond

	rec := serveChat(t, srv, "/chat?q=hello+there")
	body := rec.Body.String()
	if !strings.Contains(body, "event: done") || !strings.Contains(body, `"finish_reason":"timeout"`) {
		t.Errorf("timeout must end the stream with finish_reason=timeout\nfull body:\n%s", body)
	}
}

func TestChatEmptyQuestionRejected(t *testing.T) {
	rec := serveChat(t, newTestServer(), "/chat?q=+")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCalculatorTool(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"2+3*4", "14"},
		{"(2+3)*4", "20"},
		{"10/4", "2.5"},
		{"-3+1", "-2"},
	}
	for _, c := range cases {
		got, err := calculatorTool(c.expr)
		if err != nil {
			t.Errorf("calculatorTool(%q): unexpected error %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("calculatorTool(%q) = %q, want %q", c.expr, got, c.want)
		}
	}

	for _, expr := range []string{"", "2+", "1/0", "abc", "(1+2"} {
		if got, err := calculatorTool(expr); err == nil {
			t.Errorf("calculatorTool(%q) = %q, want error", expr, got)
		}
	}
}

func TestEchoToolRejectsEmptyInput(t *testing.T) {
	if got, err := echoTool("  "); err == nil {
		t.Errorf("echoTool(blank) = %q, want error", got)
	}
}

func decodeWire(t *testing.T, body string) []wireEvent {
	t.Helper()
	events := []wireEvent{}
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev wireEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatal(err)
		}
		events = append(events, ev)
	}
	return events
}
func TestWireIdentitySequenceAndSingleTerminal(t *testing.T) {
	srv := newTestServer()
	first := serveChat(t, srv, "/chat?q=calc+2%2B3")
	second := serveChat(t, srv, "/chat?q=echo+hello")
	firstEvents := decodeWire(t, first.Body.String())
	secondEvents := decodeWire(t, second.Body.String())
	if len(firstEvents) == 0 || len(secondEvents) == 0 || firstEvents[0].RunID == secondEvents[0].RunID {
		t.Fatal("missing/distinct run identity")
	}
	for _, rec := range []*httptest.ResponseRecorder{first, second} {
		events := decodeWire(t, rec.Body.String())
		for i, e := range events {
			if e.RunID != events[0].RunID || e.Seq != uint64(i+1) {
				t.Fatal(e)
			}
			if !strings.Contains(rec.Body.String(), fmt.Sprintf("id: %s:%d\n", e.RunID, e.Seq)) {
				t.Fatal("SSE id mismatch")
			}
		}
		if strings.Count(rec.Body.String(), "event: done\n") != 1 {
			t.Fatal("terminal count")
		}
	}
}

type brokenWriter struct {
	header       http.Header
	writes       int
	flushFailure bool
}

func (w *brokenWriter) Header() http.Header { return w.header }
func (w *brokenWriter) WriteHeader(int)     {}
func (w *brokenWriter) Write(p []byte) (int, error) {
	w.writes++
	if !w.flushFailure {
		return 0, errors.New("disconnected")
	}
	return len(p), nil
}
func (w *brokenWriter) Flush()            {}
func (w *brokenWriter) FlushError() error { return errors.New("flush failed") }
func TestWriteAndFlushFailureStopsHandler(t *testing.T) {
	for _, flush := range []bool{false, true} {
		w := &brokenWriter{header: http.Header{}, flushFailure: flush}
		done := make(chan struct{})
		go func() { newTestServer().chatHandler(w, httptest.NewRequest("GET", "/chat?q=hello", nil)); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("failed writer blocked handler")
		}
		if w.writes != 1 {
			t.Fatal("continued after write failure", w.writes)
		}
	}
}
func TestLateToolResultAfterCancelIsNotStreamed(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry := map[string]tool{"echo": func(string) (string, error) { close(entered); <-release; return "late", nil }}
	events := runAgent(ctx, registry, "echo hello", agentConfig{maxTokens: 256})
	first := <-events
	if first.name != eventToolCall {
		t.Fatal(first)
	}
	<-entered
	cancel()
	close(release)
	for ev := range events {
		t.Fatal("late event", ev)
	}
}
func TestOutputBudgetHasOneFailedTerminal(t *testing.T) {
	srv := newTestServer()
	srv.agent.maxTokens = 1
	rec := serveChat(t, srv, "/chat?q=hello")
	body := rec.Body.String()
	if strings.Count(body, "event: token\n") != 1 || strings.Count(body, "event: done\n") != 1 || !strings.Contains(body, `"finish_reason":"error"`) {
		t.Fatal(body)
	}
}
