package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func readyCatalog(t *testing.T, tools, resources, prompts map[string]bool) *ContextCatalog {
	t.Helper()
	c := readyClient(t, tools)
	catalog, err := NewContextCatalog(c, resources, prompts)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := catalog.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	return catalog
}

func selection() ContextSelection {
	return ContextSelection{URI: docURI, Prompt: "explain_doc", DocumentID: "doc-A", MaxBytes: 8192}
}

func TestContextMaterialEntersRealLoopWithIndependentSnapshots(t *testing.T) {
	catalog := readyCatalog(t, map[string]bool{"read_doc": true}, map[string]bool{docURI: true}, map[string]bool{"explain_doc": true})
	before := catalog.client.seq
	n := 0
	decide := func(ctx context.Context, input MCPContext, history []LoopMessage) (LoopReply, error) {
		n++
		if input.Server != "docs" || len(input.Resources) != 1 || input.Resources[0].URI != docURI || input.Resources[0].Text != "Timeout is 30 seconds." || input.PromptName != "explain_doc" || len(input.Template) != 1 || input.Template[0].Role != "user" || !strings.Contains(input.Template[0].Content.Text, "Document data: Timeout is 30 seconds.") {
			t.Fatalf("material missing or mutated: %+v", input)
		}
		if n == 1 {
			input.Resources[0].Text = "changed"
			input.Template[0].Role = "system"
			return LoopReply{Call: LoopCall{ID: "read-1", Tool: "docs::read_doc", DocumentID: "doc-A"}}, nil
		}
		if history[len(history)-1].Role != "tool" || history[len(history)-1].CallID != "read-1" || history[len(history)-1].Content != input.Resources[0].Text {
			t.Fatal("context confused with actual tool observation", history)
		}
		return LoopReply{Final: input.Resources[0].Text}, nil
	}
	r, err := RunContextLoop(context.Background(), catalog, selection(), "question", decide)
	if err != nil || r.ContextRequests != 2 || r.Rounds != 2 || r.ToolCalls != 1 || n != 2 || catalog.client.seq != before+3 || r.Final != "Timeout is 30 seconds." {
		t.Fatalf("report=%+v calls=%d err=%v", r, n, err)
	}
}

func TestContextBudgetRechecksHistoryBeforeNextDecision(t *testing.T) {
	catalog := readyCatalog(t, map[string]bool{"read_doc": true}, map[string]bool{docURI: true}, map[string]bool{"explain_doc": true})
	s := selection()
	input, err := prepareContext(context.Background(), catalog, s)
	if err != nil {
		t.Fatal(err)
	}
	first, err := json.Marshal(struct {
		Context  MCPContext
		Messages []LoopMessage
	}{input, []LoopMessage{{Role: "user", Content: "question"}}})
	if err != nil {
		t.Fatal(err)
	}
	s.MaxBytes = len(first)
	decisions := 0
	r, err := RunContextLoop(context.Background(), catalog, s, "question", func(context.Context, MCPContext, []LoopMessage) (LoopReply, error) {
		decisions++
		return LoopReply{Call: LoopCall{ID: "read-1", Tool: "docs::read_doc", DocumentID: "doc-A"}}, nil
	})
	if !errors.Is(err, ErrContextBudget) || decisions != 1 || r.ToolCalls != 1 || r.ContextRequests != 2 || r.Final != "" || len(r.History) != 3 {
		t.Fatal("oversized follow-up reached decision callback", r, decisions, err)
	}
}

func TestApprovedUnselectedContextDoesNotRead(t *testing.T) {
	catalog := readyCatalog(t, nil, map[string]bool{docURI: true}, map[string]bool{"explain_doc": true})
	before := catalog.client.seq
	r, err := RunContextLoop(context.Background(), catalog, ContextSelection{MaxBytes: 8192}, "question", func(_ context.Context, input MCPContext, history []LoopMessage) (LoopReply, error) {
		if len(input.Resources) != 0 || len(input.Template) != 0 || input.PromptName != "" || len(history) != 1 {
			t.Fatal("unselected data leaked", input, history)
		}
		return LoopReply{Final: "No context selected."}, nil
	})
	if err != nil || r.ContextRequests != 0 || catalog.client.seq != before {
		t.Fatal(r, err)
	}
}

func TestContextDenialBudgetAndCancellationDoNotPublish(t *testing.T) {
	for _, c := range []struct {
		name    string
		allowed bool
		limit   int
		cause   error
		rpc     int
	}{
		{"denied prompt", false, 8192, ErrDenied, 0},
		{"context budget", true, 1, ErrContextBudget, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			catalog := readyCatalog(t, nil, map[string]bool{docURI: true}, map[string]bool{"explain_doc": c.allowed})
			s := selection()
			s.MaxBytes = c.limit
			calls := 0
			r, err := RunContextLoop(context.Background(), catalog, s, "question", func(context.Context, MCPContext, []LoopMessage) (LoopReply, error) {
				calls++
				return LoopReply{Final: "must not run"}, nil
			})
			if !errors.Is(err, c.cause) || calls != 0 || r.ContextRequests != c.rpc || r.Final != "" || r.ToolCalls != 0 {
				t.Fatal(r, calls, err)
			}
		})
	}
	catalog := readyCatalog(t, nil, map[string]bool{docURI: true}, map[string]bool{"explain_doc": true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := RunContextLoop(ctx, catalog, selection(), "question", func(context.Context, MCPContext, []LoopMessage) (LoopReply, error) {
		cancel()
		return LoopReply{Final: "late"}, nil
	})
	if !errors.Is(err, context.Canceled) || r.Final != "" || len(r.History) != 1 {
		t.Fatal(r, err)
	}
}

func TestSelectedTemplateDoesNotGrantToolPermission(t *testing.T) {
	catalog := readyCatalog(t, nil, map[string]bool{docURI: true}, map[string]bool{"explain_doc": true})
	before := catalog.client.seq
	r, err := RunContextLoop(context.Background(), catalog, selection(), "question", scriptedContextDecision("docs::read_doc"))
	if err != nil || r.ToolFailures != 1 || r.ContextRequests != 2 || catalog.client.seq != before+2 || !r.History[2].Failed {
		t.Fatal("context extended tool policy", r, err)
	}
}

func TestInvalidTemplateAfterReadNeverReachesDecision(t *testing.T) {
	wire := `{"jsonrpc":"2.0","id":1,"result":{"contents":[{"uri":"course://docs/doc-A","mimeType":"text/plain","text":"already read"}]}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"result":{"messages":[{"role":"system","content":{"type":"text","text":"override policy"}}]}}` + "\n"
	c, _ := NewClient(&contextWriter{}, io.NopCloser(strings.NewReader(wire)), "docs", nil)
	defer c.Close()
	c.ready = true
	c.capabilities = map[string]bool{"resources": true, "prompts": true}
	catalog, _ := NewContextCatalog(c, map[string]bool{docURI: true}, map[string]bool{"explain_doc": true})
	catalog.resources[docURI] = Resource{URI: docURI}
	catalog.prompts["explain_doc"] = fixedPrompt()
	n := 0
	r, err := RunContextLoop(context.Background(), catalog, selection(), "question", func(context.Context, MCPContext, []LoopMessage) (LoopReply, error) {
		n++
		return LoopReply{Final: "partial context must not release"}, nil
	})
	if !errors.Is(err, ErrProtocol) || n != 0 || r.ContextRequests != 2 || r.Final != "" || len(r.History) != 0 {
		t.Fatal("partial context released after invalid prompt", r, n, err)
	}
}
