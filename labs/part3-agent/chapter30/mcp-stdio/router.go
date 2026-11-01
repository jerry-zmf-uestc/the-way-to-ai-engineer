package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
)

// ToolRouter owns a fixed Host routing snapshot, not the Clients or their processes.
// One caller uses it serially; per-session budgets and cancellation remain in Client.
type ToolRouter struct {
	routes map[string]*Client
}

func NewToolRouter(clients ...*Client) (*ToolRouter, error) {
	if len(clients) < 1 || len(clients) > 4 {
		return nil, ErrProtocol
	}
	routes := map[string]*Client{}
	aliases := map[string]bool{}
	for _, c := range clients {
		if c == nil || !c.ready || c.closed || aliases[c.alias] {
			return nil, ErrProtocol
		}
		aliases[c.alias] = true
		for name := range c.tools {
			if c.approved[name] {
				routes[c.alias+"::"+name] = c
			}
		}
	}
	if len(routes) > 32 {
		return nil, ErrProtocol
	}
	return &ToolRouter{routes: routes}, nil
}

func (r *ToolRouter) Names() []string {
	names := make([]string, 0, len(r.routes))
	for name := range r.routes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *ToolRouter) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	c, ok := r.routes[name]
	if !ok {
		return "", ErrDenied
	}
	return c.Call(ctx, name, args)
}

// Both commands are this program with fixed profiles. A tool name never selects an executable.
func multiServerMode(ctx context.Context, output io.Writer, agent, actualModel bool) (resultErr error) {
	clients := []*Client{}
	for _, profile := range []string{"primary", "archive"} {
		cmd := exec.CommandContext(ctx, os.Args[0], "-server", "-profile", profile)
		cmd.Stderr = os.Stderr
		input, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		response, err := cmd.StdoutPipe()
		if err != nil {
			input.Close()
			return err
		}
		if err = cmd.Start(); err != nil {
			input.Close()
			response.Close()
			return err
		}
		c, err := NewClient(input, response, profile, map[string]bool{"read_doc": true})
		if err != nil {
			input.Close()
			response.Close()
			return errors.Join(err, cmd.Wait())
		}
		defer func() {
			c.Close()
			resultErr = errors.Join(resultErr, cmd.Wait())
		}()
		if err = c.Initialize(ctx); err != nil {
			return err
		}
		if _, err = c.Discover(ctx); err != nil {
			return err
		}
		clients = append(clients, c)
	}
	router, err := NewToolRouter(clients...)
	if err != nil {
		return err
	}
	if actualModel {
		record := ModelRun{Dataset: "mcp-routing-development-v1", Prompt: "mcp-routing-v1", ModelRevision: "c1899de289a04d12100db370d81485cdf75e47ca", SemanticQuality: "unreviewed"}
		if os.Getenv("MCP_MODEL_CONTRACT") == "v2" {
			record.Prompt = "mcp-routing-v2"
		}
		record.Run, err = RunRoutedLoop(ctx, router, "Read doc-A from primary and archive and compare their timeout.", modelDecision(router.Names(), &record.Calls))
		record.Complete = true
		record.CoveragePass = routingCoverage(record.Run, err)
		if err != nil {
			record.Error = "run_failed"
		}
		return errors.Join(err, json.NewEncoder(output).Encode(record))
	}
	if agent {
		report, err := RunRoutedLoop(ctx, router, "Compare the two document versions.", scriptedRouteComparison(router.Names()))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "agent_rounds=%d tool_calls=%d tool_failures=%d\nanswer=%q\n", report.Rounds, report.ToolCalls, report.ToolFailures, report.Final)
		return err
	}
	for _, name := range router.Names() {
		text, err := router.Call(ctx, name, map[string]any{"id": "doc-A"})
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(output, "namespace=%s text=%q\n", name, text); err != nil {
			return err
		}
	}
	return nil
}

func scriptedRouteComparison(names []string) Decision {
	names = append([]string(nil), names...)
	return func(ctx context.Context, history []LoopMessage) (LoopReply, error) {
		if len(names) != 2 {
			return LoopReply{}, ErrProtocol
		}
		observed := []LoopMessage{}
		for _, m := range history {
			if m.Role == "tool" {
				if m.Failed {
					return LoopReply{Final: "The selected source did not provide usable evidence."}, nil
				}
				observed = append(observed, m)
			}
		}
		if len(observed) < len(names) {
			return LoopReply{Call: LoopCall{ID: fmt.Sprintf("read-%d", len(observed)+1), Tool: names[len(observed)], DocumentID: "doc-A"}}, nil
		}
		return LoopReply{Final: fmt.Sprintf("%s: %s | %s: %s", observed[0].Tool, observed[0].Content, observed[1].Tool, observed[1].Content)}, nil
	}
}
