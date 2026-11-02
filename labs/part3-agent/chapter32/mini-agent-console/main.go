// Command mini-agent-console is a self-contained streaming agent demo: a web
// page sends a question, a deterministic offline fake model optionally asks
// for one tool call (calculator, echo or research), and the answer streams back to the
// page token by token over SSE. It depends only on the standard library.
package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	eventToken      = "token"
	eventToolCall   = "tool_call"
	eventToolResult = "tool_result"
	eventAgentError = "agent_error"
	eventDone       = "done"

	maxQuestionBytes = 4096
)

var (
	ErrEmptyExpression = errors.New("calculator: empty expression")
	ErrUnexpectedToken = errors.New("calculator: unexpected token")
	ErrDivisionByZero  = errors.New("calculator: division by zero")
	ErrEmptyEcho       = errors.New("echo: empty input")
	ErrTokenBudget     = errors.New("agent: token budget exceeded")
)

// streamEvent is one named SSE event with a pre-encoded single-line JSON
// payload.
type streamEvent struct {
	name string
	data string
}

func newEvent(name string, payload any) streamEvent {
	data, err := json.Marshal(payload)
	if err != nil {
		// Payloads are flat string maps; a marshal failure is a programmer bug.
		panic(err)
	}
	return streamEvent{name: name, data: string(data)}
}

// wireEvent separates transport identity/order from the typed application payload.
// IDs are diagnostic only: this Lab does not store/replay events on reconnect.
type wireEvent struct {
	RunID   string          `json:"run_id"`
	Seq     uint64          `json:"seq"`
	Payload json.RawMessage `json:"payload"`
}

func writeSSE(w http.ResponseWriter, runID string, seq uint64, ev streamEvent) error {
	data, err := json.Marshal(wireEvent{RunID: runID, Seq: seq, Payload: json.RawMessage(ev.data)})
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "id: %s:%d\nevent: %s\ndata: %s\n\n", runID, seq, ev.name, data); err != nil {
		return err
	}
	return http.NewResponseController(w).Flush()
}

func doneEvent(reason string) streamEvent {
	return newEvent(eventDone, map[string]string{"finish_reason": reason})
}

// tool is a read-only capability admitted by the application registry. Tool
// permission is decided by this registry, never by the prompt.
type tool func(args string) (string, error)

func defaultRegistry() map[string]tool {
	return map[string]tool{
		"calculator": calculatorTool,
		"echo":       echoTool,
		"research":   researchTool,
	}
}

// --- deterministic offline fake model ---

type modelDecision struct {
	toolName string
	toolArgs string
	reply    string
}

// decide maps a question to either a single tool-call proposal or a direct
// reply. It is deliberately rule based so tests and demos are reproducible.
func decide(question string) modelDecision {
	q := strings.TrimSpace(question)
	switch {
	case q == "research" || strings.HasPrefix(q, "research "):
		return modelDecision{toolName: "research", toolArgs: strings.TrimSpace(strings.TrimPrefix(q, "research"))}
	case strings.HasPrefix(q, "calc "):
		return modelDecision{toolName: "calculator", toolArgs: strings.TrimSpace(q[len("calc "):])}
	case strings.HasPrefix(q, "echo "):
		return modelDecision{toolName: "echo", toolArgs: strings.TrimSpace(q[len("echo "):])}
	case strings.HasPrefix(q, "tool "):
		// Escape hatch: let the "model" propose an arbitrary tool name so the
		// registry rejection path is observable from the UI.
		rest := strings.TrimSpace(q[len("tool "):])
		name, args, _ := strings.Cut(rest, " ")
		return modelDecision{toolName: name, toolArgs: args}
	default:
		return modelDecision{reply: "You asked: " + q + ". This offline fake model answers directly and never calls a tool."}
	}
}

func replyAfterTool(name, result string) string {
	return fmt.Sprintf("The %s tool returned %q. This concludes the fake agent loop.", name, result)
}

// --- agent loop ---

type agentConfig struct {
	tokenDelay time.Duration
	maxTokens  int
}

// runAgent executes the fake agent loop in a new goroutine and returns a
// channel of SSE events. The goroutine always closes the channel and exits
// when the loop finishes or ctx is cancelled, so a disconnected client never
// leaks it. Registered tools must also return promptly; cancellation cannot
// forcibly interrupt a non-cooperating tool.
func runAgent(ctx context.Context, registry map[string]tool, question string, cfg agentConfig) <-chan streamEvent {
	out := make(chan streamEvent)
	go func() {
		defer close(out)

		send := func(ev streamEvent) bool {
			if ctx.Err() != nil {
				return false
			}
			select {
			case out <- ev:
				return true
			case <-ctx.Done():
				return false
			}
		}

		streamReply := func(text string) bool {
			sent := 0
			for _, token := range strings.Fields(text) {
				if sent >= cfg.maxTokens {
					send(newEvent(eventAgentError, map[string]string{"message": ErrTokenBudget.Error()}))
					send(doneEvent("error"))
					return false
				}
				if cfg.tokenDelay > 0 {
					select {
					case <-ctx.Done():
						return false
					case <-time.After(cfg.tokenDelay):
					}
				}
				if !send(newEvent(eventToken, map[string]string{"token": token + " "})) {
					return false
				}
				sent++
			}
			return true
		}

		decision := decide(question)
		if decision.toolName == "" {
			if streamReply(decision.reply) {
				send(doneEvent("stop"))
			}
			return
		}

		// The fake model may propose at most one tool call per question.
		if !send(newEvent(eventToolCall, map[string]string{"name": decision.toolName, "args": decision.toolArgs})) {
			return
		}
		fn, ok := registry[decision.toolName]
		if !ok {
			send(newEvent(eventAgentError, map[string]string{
				"message": fmt.Sprintf("unknown tool %q: call rejected by the application registry", decision.toolName),
			}))
			send(doneEvent("error"))
			return
		}
		if ctx.Err() != nil {
			return
		}
		result, err := fn(decision.toolArgs)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			send(newEvent(eventAgentError, map[string]string{"message": err.Error()}))
			send(doneEvent("error"))
			return
		}
		if !send(newEvent(eventToolResult, map[string]string{"name": decision.toolName, "result": result})) {
			return
		}
		reply := replyAfterTool(decision.toolName, result)
		if decision.toolName == "research" {
			reply, err = researchReply(result)
			if err != nil {
				send(newEvent(eventAgentError, map[string]string{"message": err.Error()}))
				send(doneEvent("error"))
				return
			}
		}
		if streamReply(reply) {
			send(doneEvent("stop"))
		}
	}()
	return out
}

// --- HTTP server ---

//go:embed web/index.html
var indexHTML []byte

type server struct {
	registry map[string]tool
	agent    agentConfig
	timeout  time.Duration
}

func newServer() *server {
	return &server{
		registry: defaultRegistry(),
		agent:    agentConfig{tokenDelay: 60 * time.Millisecond, maxTokens: 256},
		timeout:  30 * time.Second,
	}
}

func (s *server) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.indexHandler)
	mux.HandleFunc("GET /chat", s.chatHandler)
	return mux
}

func (s *server) indexHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

// chatHandler streams one agent session as SSE. EventSource only issues GET
// requests, so the question travels in the q query parameter. Cancelling the
// client connection (the stop button calls EventSource.close) aborts the loop
// via the request context.
func (s *server) chatHandler(w http.ResponseWriter, r *http.Request) {
	question := strings.TrimSpace(r.URL.Query().Get("q"))
	if question == "" {
		http.Error(w, "missing q query parameter", http.StatusBadRequest)
		return
	}
	if len(question) > maxQuestionBytes {
		http.Error(w, "question too long", http.StatusRequestEntityTooLarge)
		return
	}
	_, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()

	runID := rand.Text()
	var seq uint64
	write := func(ev streamEvent) bool {
		seq++
		return writeSSE(w, runID, seq, ev) == nil
	}
	finishCanceled := func() {
		reason := "cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = "timeout"
		}
		write(doneEvent(reason)) // Best effort; a disconnected viewer may never receive this terminal.
	}
	events := runAgent(ctx, s.registry, question, s.agent)
	for {
		if ctx.Err() != nil {
			finishCanceled()
			return
		}
		select {
		case ev, ok := <-events:
			if ctx.Err() != nil {
				finishCanceled()
				return
			}
			if !ok {
				return
			}
			if !write(ev) {
				return
			} // defer cancel stops a producer blocked on the next send.
			if ev.name == eventDone {
				return
			}
		case <-ctx.Done():
			finishCanceled()
			return
		}
	}

}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()

	srv := newServer()
	log.Printf("mini agent console listening on http://%s (Ctrl+C to stop)", *addr)
	httpServer := &http.Server{Addr: *addr, Handler: srv.mux(), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 35 * time.Second}
	log.Fatal(httpServer.ListenAndServe())
}

// --- tools ---

func echoTool(args string) (string, error) {
	if strings.TrimSpace(args) == "" {
		return "", ErrEmptyEcho
	}
	return args, nil
}

func calculatorTool(args string) (string, error) {
	value, err := evalExpression(args)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(value, 'g', -1, 64), nil
}

// evalExpression evaluates a tiny arithmetic grammar: numbers, + - * /,
// parentheses, and unary minus. It never touches the network, the file
// system, or user-controlled code paths.
func evalExpression(input string) (float64, error) {
	p := &exprParser{s: input}
	p.skipSpace()
	if p.pos >= len(p.s) {
		return 0, ErrEmptyExpression
	}
	value, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos != len(p.s) {
		return 0, fmt.Errorf("%w %q at position %d", ErrUnexpectedToken, p.s[p.pos:], p.pos)
	}
	return value, nil
}

type exprParser struct {
	s   string
	pos int
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *exprParser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || (p.s[p.pos] != '+' && p.s[p.pos] != '-') {
			return left, nil
		}
		op := p.s[p.pos]
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
}

func (p *exprParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || (p.s[p.pos] != '*' && p.s[p.pos] != '/') {
			return left, nil
		}
		op := p.s[p.pos]
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, ErrDivisionByZero
			}
			left /= right
		}
	}
}

func (p *exprParser) parseFactor() (float64, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return 0, fmt.Errorf("%w: missing operand", ErrUnexpectedToken)
	}
	switch p.s[p.pos] {
	case '-':
		p.pos++
		value, err := p.parseFactor()
		return -value, err
	case '(':
		p.pos++
		value, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			return 0, fmt.Errorf("%w: missing closing parenthesis", ErrUnexpectedToken)
		}
		p.pos++
		return value, nil
	}

	start := p.pos
	for p.pos < len(p.s) && (p.s[p.pos] >= '0' && p.s[p.pos] <= '9' || p.s[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("%w %q at position %d", ErrUnexpectedToken, p.s[p.pos:], start)
	}
	value, err := strconv.ParseFloat(p.s[start:p.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrUnexpectedToken, err)
	}
	return value, nil
}
