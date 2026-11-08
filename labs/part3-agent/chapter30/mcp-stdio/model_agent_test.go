package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestModelProposalPreservesRawAndRejectsTruncation(t *testing.T) {
	raw := `{"Call":{"ID":"r1","Tool":"archive::read_doc","DocumentID":"doc-A"}}`
	g := GeneratedReply{Raw: raw, InputTokens: 10, OutputTokens: 20}
	r, err := parseGenerated(g)
	if err != nil || r.Call.Tool != "archive::read_doc" || g.Raw != raw {
		t.Fatal(r, g, err)
	}
	for _, bad := range []GeneratedReply{
		{Raw: raw, InputTokens: 10, OutputTokens: 192},
		{Raw: raw, InputTokens: 10, OutputTokens: 20, AtOutputLimit: true},
		{Raw: "```json\n" + raw + "\n```", InputTokens: 10, OutputTokens: 20},
		{Raw: `{"Final":"one","Final":"two"}`, InputTokens: 10, OutputTokens: 20},
		{Raw: raw, InputTokens: 1025, OutputTokens: 20},
	} {
		if _, err := parseGenerated(bad); !errors.Is(err, ErrModel) {
			t.Fatal("bad model boundary accepted", bad, err)
		}
	}
}

func TestModelGateDoesNotStartProcess(t *testing.T) {
	t.Setenv("MCP_MODEL_E2E", "")
	var records []ModelRecord
	_, err := modelDecision([]string{"primary::read_doc"}, &records)(context.Background(), []LoopMessage{{Role: "user", Content: "question"}})
	if !errors.Is(err, ErrModel) || len(records) != 1 || records[0].Stage != "model_process_failed" || len(records[0].View.Tools) != 1 {
		t.Fatal(records, err)
	}
}

func TestApplicationBindsOnlyValidLocalToolProposals(t *testing.T) {
	g := GeneratedReply{Raw: `{"Call":{"Tool":"archive::read_doc","DocumentID":"doc-A"}}`, InputTokens: 10, OutputTokens: 20}
	reply, err := bindGenerated(g, "host-1")
	if err != nil || reply.Call.ID != "host-1" || reply.Call.Tool != "archive::read_doc" || g.Raw != `{"Call":{"Tool":"archive::read_doc","DocumentID":"doc-A"}}` {
		t.Fatal(reply, g, err)
	}
	for _, raw := range []string{
		`{"Call":{"ID":"forged","Tool":"archive::read_doc","DocumentID":"doc-A"}}`,
		`{"Final":"x","Call":{"Tool":"archive::read_doc","DocumentID":"doc-A"}}`,
		`{"Call":{"Tool":"archive::read_doc"}}`,
		`{}`,
	} {
		g.Raw = raw
		if _, err := bindGenerated(g, "host-1"); !errors.Is(err, ErrModel) {
			t.Fatal("invalid proposal bound", raw, err)
		}
	}
	g.Raw = `{"Final":"observed answer"}`
	if reply, err := bindGenerated(g, "host-1"); err != nil || reply.Call.ID != "" {
		t.Fatal(reply, err)
	}
}

func TestRoutingCoverageRequiresActualBothSourceObservations(t *testing.T) {
	r := LoopReport{Final: "30 and 45", History: []LoopMessage{{Role: "tool", Tool: "primary::read_doc"}, {Role: "tool", Tool: "archive::read_doc"}}}
	if !routingCoverage(r, nil) {
		t.Fatal("closed fixture coverage lost")
	}
	r.History = r.History[:1]
	if routingCoverage(r, nil) {
		t.Fatal("guessed answer counted without both sources")
	}
	if routingCoverage(r, ErrModel) {
		t.Fatal("failed run counted")
	}
}

func TestActualRoutingRecordReplaysWithoutModel(t *testing.T) {
	raw, err := os.ReadFile("testdata/model-routing-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var saved ModelRun
	if json.Unmarshal(raw, &saved) != nil || !saved.Complete || saved.CoveragePass || len(saved.Calls) != 2 {
		t.Fatal("invalid saved diagnostic")
	}
	router, primary, archive := readyRouter(t, true)
	a, b := primary.seq, archive.seq
	n := 0
	r, err := RunRoutedLoop(context.Background(), router, saved.Run.History[0].Content, func(_ context.Context, h []LoopMessage) (LoopReply, error) {
		if n >= len(saved.Calls) || !reflect.DeepEqual(h, saved.Calls[n].View.History) || !reflect.DeepEqual(router.Names(), saved.Calls[n].View.Tools) {
			t.Fatal("record input drift")
		}
		g := saved.Calls[n].GeneratedReply
		n++
		return parseGenerated(g)
	})
	if !errors.Is(err, ErrProtocol) || n != 2 || !reflect.DeepEqual(r, saved.Run) || primary.seq != a || archive.seq != b+1 {
		t.Fatal(r, n, err)
	}
}

func TestActualBoundRoutingRecordStillRejectsRepeatedSourceBudget(t *testing.T) {
	raw, err := os.ReadFile("testdata/model-routing-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var saved ModelRun
	if json.Unmarshal(raw, &saved) != nil || !saved.Complete || saved.CoveragePass || len(saved.Calls) != 3 {
		t.Fatal("invalid bound diagnostic")
	}
	router, primary, archive := readyRouter(t, true)
	a, b := primary.seq, archive.seq
	n := 0
	r, err := RunRoutedLoop(context.Background(), router, saved.Run.History[0].Content, func(_ context.Context, h []LoopMessage) (LoopReply, error) {
		if n >= len(saved.Calls) || !reflect.DeepEqual(h, saved.Calls[n].View.History) || !reflect.DeepEqual(router.Names(), saved.Calls[n].View.Tools) {
			t.Fatal("bound record input drift")
		}
		call := saved.Calls[n]
		n++
		if call.View.CallID != fmt.Sprintf("model-%d", n) || call.BoundCallID != call.View.CallID {
			t.Fatal("association ownership drift", call)
		}
		return bindGenerated(call.GeneratedReply, call.View.CallID)
	})
	if !errors.Is(err, ErrLoopBudget) || n != 3 || !reflect.DeepEqual(r, saved.Run) || primary.seq != a || archive.seq != b+2 {
		t.Fatal(r, n, err)
	}
}
