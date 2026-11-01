package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

var ErrContextBudget = errors.New("mcp agent: context input budget exhausted")

type ContextSelection struct {
	URI, Prompt, DocumentID string
	MaxBytes                int
}

// MCPContext保留外部角色标签作为资料，不作为应用政策或工具权限。
type MCPContext struct {
	Server     string
	Resources  []ResourceText
	PromptName string
	Template   []PromptMessage
}
type ContextDecision func(context.Context, MCPContext, []LoopMessage) (LoopReply, error)
type ContextLoopReport struct {
	LoopReport
	ContextRequests int
}

func prepareContext(ctx context.Context, c *ContextCatalog, s ContextSelection) (MCPContext, error) {
	if c == nil || c.client == nil || !c.client.ready || c.client.closed || s.MaxBytes <= 0 || s.MaxBytes > 16<<10 || len(s.URI) > 512 || len(s.Prompt) > 128 || len(s.DocumentID) > 128 || (s.Prompt == "" && s.DocumentID != "") || (s.Prompt != "" && s.DocumentID == "") {
		return MCPContext{}, ErrProtocol
	}
	if err := ctx.Err(); err != nil {
		return MCPContext{}, err
	}
	if s.URI != "" && (!c.approvedResources[s.URI] || c.resources[s.URI].URI != s.URI) {
		return MCPContext{}, ErrDenied
	}
	if s.Prompt != "" && (!c.approvedPrompts[s.Prompt] || c.prompts[s.Prompt].Name != s.Prompt) {
		return MCPContext{}, ErrDenied
	}
	input := MCPContext{Server: c.client.alias}
	if s.URI != "" {
		text, err := c.Read(ctx, s.URI)
		if err != nil {
			return MCPContext{}, err
		}
		input.Resources = []ResourceText{{URI: s.URI, MimeType: "text/plain", Text: text}}
	}
	if s.Prompt != "" {
		messages, err := c.GetPrompt(ctx, s.Prompt, s.DocumentID)
		if err != nil {
			return MCPContext{}, err
		}
		input.PromptName, input.Template = s.Prompt, messages
	}
	return input, ctx.Err()
}

func RunContextLoop(ctx context.Context, catalog *ContextCatalog, s ContextSelection, question string, decide ContextDecision) (report ContextLoopReport, err error) {
	if catalog == nil || catalog.client == nil || decide == nil || strings.TrimSpace(question) == "" || len(question) > 4096 {
		return report, ErrProtocol
	}
	before := catalog.client.seq
	input, err := prepareContext(ctx, catalog, s)
	report.ContextRequests = catalog.client.seq - before
	if err != nil {
		return report, err
	}
	report.LoopReport, err = RunLoop(ctx, catalog.client, question, func(ctx context.Context, history []LoopMessage) (LoopReply, error) {
		view := MCPContext{Server: input.Server, Resources: append([]ResourceText(nil), input.Resources...), PromptName: input.PromptName, Template: append([]PromptMessage(nil), input.Template...)}
		payload, err := json.Marshal(struct {
			Context  MCPContext
			Messages []LoopMessage
		}{view, history})
		if err != nil || len(payload) > s.MaxBytes {
			return LoopReply{}, ErrContextBudget
		}
		return decide(ctx, view, history)
	})
	return report, err
}

func scriptedContextDecision(tool string) ContextDecision {
	return func(ctx context.Context, input MCPContext, history []LoopMessage) (LoopReply, error) {
		if len(input.Resources) != 1 || input.PromptName != "explain_doc" || len(input.Template) != 1 {
			return LoopReply{}, ErrProtocol
		}
		return scriptedDecision(tool)(ctx, history)
	}
}
