package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const version = "2025-11-25"

var (
	ErrProtocol = errors.New("mcp: protocol failure")
	ErrDenied   = errors.New("mcp: local tool denied")
	ErrTool     = errors.New("mcp: tool failed")
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("mcp: RPC error %d", e.Code) }

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}
type Property struct {
	Type string `json:"type"`
}
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
	Additional *bool               `json:"additionalProperties"`
}
type Tool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  Schema          `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
}
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

func tool() Tool {
	no := false
	return Tool{Name: "read_doc", Description: "Read one fixed teaching document by id.", InputSchema: Schema{Type: "object", Properties: map[string]Property{"id": {Type: "string"}}, Required: []string{"id"}, Additional: &no}}
}
func arguments(args map[string]any) bool {
	value, ok := args["id"].(string)
	return ok && value != "" && len(value) <= 128 && len(args) == 1
}

// Serve exposes a small fixed capability subset; stdout contains only protocol messages.
func Serve(input io.Reader, output io.Writer) error {
	return serveProfile(input, output, "primary")
}

func serveProfile(input io.Reader, output io.Writer, profile string) error {
	document := "Timeout is 30 seconds."
	if profile == "archive" {
		document = "Archive timeout is 45 seconds."
	} else if profile != "primary" && profile != "structured" {
		return ErrProtocol
	}
	scan := bufio.NewScanner(input)
	scan.Buffer(make([]byte, 4096), 64<<10)
	enc := json.NewEncoder(output)
	initialized, ready := false, false
	for scan.Scan() {
		var req message
		if err := json.Unmarshal(scan.Bytes(), &req); err != nil {
			return ErrProtocol
		}
		if len(req.Result) > 0 || req.Error != nil {
			return ErrProtocol
		}
		if req.ID == nil {
			if req.JSONRPC == "2.0" && req.Method == "notifications/initialized" && initialized {
				ready = true
				continue
			}
			return ErrProtocol
		}
		response := message{JSONRPC: "2.0", ID: req.ID}
		var result any
		fail := func(code int, text string) { response.Error = &rpcError{Code: code, Message: text} }
		switch {
		case req.JSONRPC != "2.0":
			fail(-32600, "invalid request")
		case req.Method == "initialize":
			var params struct {
				ProtocolVersion string                     `json:"protocolVersion"`
				Capabilities    map[string]json.RawMessage `json:"capabilities"`
				ClientInfo      struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"clientInfo"`
			}
			if initialized || json.Unmarshal(req.Params, &params) != nil || params.ProtocolVersion != version || params.Capabilities == nil || params.ClientInfo.Name == "" || params.ClientInfo.Version == "" {
				fail(-32602, "unsupported initialization")
			} else {
				initialized = true
				result = map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}, "resources": map[string]any{}, "prompts": map[string]any{}}, "serverInfo": map[string]string{"name": "doc-server", "version": "1.0.0"}}
			}
		case !ready:
			fail(-32000, "not initialized")
		case req.Method == "tools/list":
			definition := tool()
			if profile == "structured" {
				definition.OutputSchema = structuredSchema()
			}
			result = map[string]any{"tools": []Tool{definition}}
		case req.Method == "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if json.Unmarshal(req.Params, &params) != nil || params.Name != "read_doc" || !arguments(params.Arguments) {
				fail(-32602, "invalid tool arguments")
			} else if params.Arguments["id"] != "doc-A" {
				result = ToolResult{IsError: true, Content: []Content{{Type: "text", Text: "document_not_found"}}}
			} else {
				if profile == "structured" {
					result = structuredSuccess()
				} else {
					result = ToolResult{Content: []Content{{Type: "text", Text: document}}}
				}
			}
		default:
			var code int
			var handled bool
			result, code, handled = contextResponse(req.Method, req.Params, document)
			if !handled {
				fail(-32601, "method not found")
			} else if code != 0 {
				fail(code, "invalid or unavailable context")
			}
		}
		if response.Error == nil {
			data, err := json.Marshal(result)
			if err != nil {
				return err
			}
			response.Result = data
		}
		if err := enc.Encode(response); err != nil {
			return err
		}
	}
	return scan.Err()
}

type Client struct {
	input         io.WriteCloser
	output        io.ReadCloser
	scan          *bufio.Scanner
	seq           int
	ready, closed bool
	alias         string
	approved      map[string]bool
	tools         map[string]Tool
	capabilities  map[string]bool
}

func NewClient(input io.WriteCloser, output io.ReadCloser, alias string, approved map[string]bool) (*Client, error) {
	if alias == "" || len(alias) > 64 || strings.Contains(alias, "::") {
		return nil, ErrProtocol
	}
	scan := bufio.NewScanner(output)
	scan.Buffer(make([]byte, 4096), 64<<10)
	permissions := map[string]bool{}
	for name, yes := range approved {
		permissions[name] = yes
	}
	return &Client{input: input, output: output, scan: scan, alias: alias, approved: permissions, tools: map[string]Tool{}}, nil
}
func (c *Client) Close() {
	if !c.closed {
		c.closed = true
		c.ready = false
		c.input.Close()
		c.output.Close()
	}
}
func (c *Client) send(req message) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if len(data) > 16<<10 {
		return ErrProtocol
	}
	_, err = c.input.Write(append(data, '\n'))
	return err
}

func (c *Client) notify(ctx context.Context, req message) error {
	if c.closed {
		return ErrProtocol
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- c.send(req) }()
	select {
	case err := <-done:
		if stop := ctx.Err(); stop != nil {
			c.Close()
			return stop
		}
		return err
	case <-ctx.Done():
		c.Close()
		<-done
		return ctx.Err()
	}
}

// exchange只支持单个未完成请求；超时关闭整个stdio会话，不假称服务端动作已撤销。
func (c *Client) exchange(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if c.closed || c.seq >= 32 {
		return nil, ErrProtocol
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	c.seq++
	id := c.seq

	type received struct {
		data []byte
		err  error
	}
	done := make(chan received, 1)
	go func() {
		if err := c.send(message{JSONRPC: "2.0", ID: &id, Method: method, Params: data}); err != nil {
			done <- received{err: err}
			return
		}
		if !c.scan.Scan() {
			err := c.scan.Err()
			if err == nil {
				err = io.EOF
			}
			done <- received{err: err}
			return
		}
		done <- received{data: append([]byte(nil), c.scan.Bytes()...)}
	}()
	var packet received
	select {
	case packet = <-done:
	case <-ctx.Done():
		c.Close()
		<-done
		return nil, ctx.Err()
	}
	if stop := ctx.Err(); stop != nil {
		c.Close()
		return nil, stop
	}
	if packet.err != nil {
		c.Close()
		return nil, packet.err
	}
	var response message
	if json.Unmarshal(packet.data, &response) != nil || response.JSONRPC != "2.0" || response.ID == nil || *response.ID != id || response.Method != "" || len(response.Params) > 0 || (response.Error == nil) == (len(response.Result) == 0) {
		c.Close()
		return nil, ErrProtocol
	}
	if response.Error != nil {
		return nil, response.Error
	}
	return response.Result, nil
}
func (c *Client) Initialize(ctx context.Context) error {
	if c.ready {
		return ErrProtocol
	}
	result, err := c.exchange(ctx, "initialize", map[string]any{"protocolVersion": version, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "course-client", "version": "1.0.0"}})
	if err != nil {
		return err
	}
	var init struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
	}
	if json.Unmarshal(result, &init) != nil || init.ProtocolVersion != version {
		c.Close()
		return ErrProtocol
	}
	if _, ok := init.Capabilities["tools"]; !ok {
		c.Close()
		return ErrProtocol
	}
	c.capabilities = map[string]bool{}
	for name, raw := range init.Capabilities {
		var capability map[string]json.RawMessage
		if json.Unmarshal(raw, &capability) != nil || capability == nil {
			c.Close()
			return ErrProtocol
		}
		c.capabilities[name] = true
	}
	if err := c.notify(ctx, message{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		c.Close()
		return err
	}
	c.ready = true
	return nil
}
func (c *Client) Discover(ctx context.Context) ([]string, error) {
	if !c.ready {
		return nil, ErrProtocol
	}
	result, err := c.exchange(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var list struct {
		Tools      []Tool `json:"tools"`
		NextCursor string `json:"nextCursor"`
	}
	if json.Unmarshal(result, &list) != nil || len(list.Tools) > 8 || list.NextCursor != "" {
		return nil, ErrProtocol
	}
	candidate := map[string]Tool{}
	names := []string{}
	for _, t := range list.Tools {
		if len(t.OutputSchema) > 0 && validateStructuredSchema(t.OutputSchema) != nil {
			return nil, ErrProtocol
		}
		schema := t.InputSchema
		if t.Name == "" || len(t.Name) > 128 || strings.Contains(t.Name, "::") || len(t.Description) > 1024 || schema.Type != "object" || len(schema.Properties) != 1 || schema.Properties["id"].Type != "string" || len(schema.Required) != 1 || schema.Required[0] != "id" || schema.Additional == nil || *schema.Additional {
			return nil, ErrProtocol
		}
		if _, exists := candidate[t.Name]; exists {
			return nil, ErrProtocol
		}
		candidate[t.Name] = t
		if c.approved[t.Name] {
			names = append(names, c.alias+"::"+t.Name)
		}
	}
	c.tools = candidate
	return names, nil
}
func (c *Client) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	local := strings.TrimPrefix(name, c.alias+"::")
	if !c.ready || name == local || !c.approved[local] {
		return "", ErrDenied
	}
	if _, ok := c.tools[local]; !ok {
		return "", ErrDenied
	}
	if !arguments(args) {
		return "", ErrProtocol
	}
	data, err := c.exchange(ctx, "tools/call", map[string]any{"name": local, "arguments": args})
	if err != nil {
		return "", err
	}
	var result ToolResult
	if json.Unmarshal(data, &result) != nil || len(result.Content) == 0 || len(result.Content) > 16 {
		return "", ErrProtocol
	}
	text := ""
	for _, part := range result.Content {
		if part.Type != "text" || len(text)+len(part.Text) > 4096 {
			return "", ErrProtocol
		}
		text += part.Text
	}
	if result.IsError {
		return "", ErrTool
	}
	return text, nil
}
func main() {
	server := flag.Bool("server", false, "serve trusted local stdio session")
	agent := flag.Bool("agent", false, "run fixed decision loop over real stdio tools")
	contextDemo := flag.Bool("context", false, "read an approved resource and obtain an approved prompt")
	contextAgent := flag.Bool("context-agent", false, "select approved context and pass it to the fixed decision loop")
	multi := flag.Bool("multi-server", false, "route same-name tools to two trusted local processes")
	routedAgent := flag.Bool("routed-agent", false, "run one fixed decision loop over two server routes")
	routedModel := flag.Bool("routed-model", false, "explicit local model generation over two trusted server routes")
	profile := flag.String("profile", "primary", "fixed teaching server profile")
	structured := flag.Bool("structured", false, "validate fixed structured output and project observation")
	flag.Parse()
	modes := 0
	for _, chosen := range []bool{*agent, *contextDemo, *multi, *contextAgent, *routedAgent, *routedModel, *structured} {
		if chosen {
			modes++
		}
	}
	if modes > 1 {
		panic("choose one demonstration mode")
	}
	if *server {
		if err := serveProfile(os.Stdin, os.Stdout, *profile); err != nil {
			fmt.Fprintln(os.Stderr, "server:", err)
			os.Exit(1)
		}
		return
	}
	timeout := 3 * time.Second
	if *routedModel {
		if os.Getenv("MCP_MODEL_E2E") != "1" {
			panic("explicit MCP_MODEL_E2E=1 required")
		}
		timeout = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if *multi || *routedAgent || *routedModel {
		if err := multiServerMode(ctx, os.Stdout, *routedAgent, *routedModel); err != nil {
			panic(err)
		}
		return
	}
	cmd := exec.CommandContext(ctx, os.Args[0], "-server")
	if *structured {
		cmd = exec.CommandContext(ctx, os.Args[0], "-server", "-profile", "structured")
	}
	cmd.Stderr = os.Stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err = cmd.Start(); err != nil {
		panic(err)
	}
	client, err := NewClient(input, output, "docs", map[string]bool{"read_doc": true})
	if err != nil {
		panic(err)
	}
	if err = client.Initialize(ctx); err != nil {
		client.Close()
		cmd.Wait()
		panic(err)
	}
	names, err := client.Discover(ctx)
	if err != nil {
		client.Close()
		cmd.Wait()
		panic(err)
	}
	var text string
	var report LoopReport
	if *structured {
		var value StructuredDocument
		value, err = client.CallStructured(ctx, names[0], map[string]any{"id": "doc-A"}, "4")
		if err == nil {
			text = value.Observation()
		}
	} else if *contextDemo || *contextAgent {
		catalog, createErr := NewContextCatalog(client, map[string]bool{docURI: true}, map[string]bool{"explain_doc": true})
		if createErr != nil {
			err = createErr
		} else {
			resources, prompts, discoverErr := catalog.Discover(ctx)
			err = discoverErr
			if err == nil && (len(resources) != 1 || len(prompts) != 1) {
				err = ErrProtocol
			}
			if err == nil && *contextAgent {
				prepared, loopErr := RunContextLoop(ctx, catalog, ContextSelection{URI: docURI, Prompt: "explain_doc", DocumentID: "doc-A", MaxBytes: 8192}, "What is timeout?", scriptedContextDecision(names[0]))
				err = loopErr
				report, text = prepared.LoopReport, prepared.Final
				if err == nil {
					fmt.Printf("context_requests=%d agent_rounds=%d tool_calls=%d\n", prepared.ContextRequests, prepared.Rounds, prepared.ToolCalls)
				}
			} else if err == nil {
				text, err = catalog.Read(ctx, resources[0].URI)
				if err == nil {
					messages, promptErr := catalog.GetPrompt(ctx, prompts[0].Name, "doc-A")
					err = promptErr
					if err == nil {
						fmt.Printf("resources=%d prompts=%d prompt_messages=%d automatic_model_injection=false\n", len(resources), len(prompts), len(messages))
					}
				}
			}
		}
	} else if *agent {
		report, err = RunLoop(ctx, client, "What is timeout?", scriptedDecision(names[0]))
		text = report.Final
	} else {
		text, err = client.Call(ctx, names[0], map[string]any{"id": "doc-A"})
	}

	client.Close()
	waitErr := cmd.Wait()
	if err != nil {
		panic(err)
	}
	if waitErr != nil {
		panic(waitErr)
	}
	if *agent {
		fmt.Printf("agent_rounds=%d tool_calls=%d tool_failures=%d\n", report.Rounds, report.ToolCalls, report.ToolFailures)
	}
	if *contextDemo {
		fmt.Printf("resource_uri=%s text=%q\n", docURI, text)
	} else {
		fmt.Printf("namespace=%s text=%q\n", names[0], text)
	}
}
