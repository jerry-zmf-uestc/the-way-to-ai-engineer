package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func readyRouter(t *testing.T, archiveAllowed bool) (*ToolRouter, *Client, *Client) {
	t.Helper()
	primary := processForServer(t, "primary", "primary", map[string]bool{"read_doc": true})
	archive := processForServer(t, "archive", "archive", map[string]bool{"read_doc": archiveAllowed})
	discoverServer(t, primary)
	discoverServer(t, archive)
	router, err := NewToolRouter(primary, archive)
	if err != nil {
		t.Fatal(err)
	}
	return router, primary, archive
}

func TestRoutedLoopUsesTwoActualSourcesAndCallIDs(t *testing.T) {
	router, primary, archive := readyRouter(t, true)
	a, b := primary.seq, archive.seq
	r, err := RunRoutedLoop(context.Background(), router, "compare", scriptedRouteComparison(router.Names()))
	if err != nil || r.Rounds != 3 || r.ToolCalls != 2 || r.ToolFailures != 0 || primary.seq != a+1 || archive.seq != b+1 || len(r.History) != 6 {
		t.Fatal(r, err)
	}
	for i, name := range []string{"archive::read_doc", "primary::read_doc"} {
		proposal, result := r.History[1+i*2], r.History[2+i*2]
		if proposal.Tool != name || result.Tool != name || proposal.CallID != result.CallID || result.Role != "tool" || !strings.Contains(r.Final, name+": "+result.Content) {
			t.Fatal("source/call association lost", r.History)
		}
	}
	if r.History[2].Content != "Archive timeout is 45 seconds." || r.History[4].Content != "Timeout is 30 seconds." {
		t.Fatal("actual sources mixed", r)
	}
}

func TestRoutedLoopSharesToolBudgetAcrossServers(t *testing.T) {
	router, primary, archive := readyRouter(t, true)
	a, b := primary.seq, archive.seq
	proposals := 0
	r, err := RunRoutedLoop(context.Background(), router, "question", func(context.Context, []LoopMessage) (LoopReply, error) {
		proposals++
		name := "primary::read_doc"
		if proposals%2 == 0 {
			name = "archive::read_doc"
		}
		return LoopReply{Call: LoopCall{ID: fmt.Sprint(proposals), Tool: name, DocumentID: "doc-A"}}, nil
	})
	if !errors.Is(err, ErrLoopBudget) || r.ToolCalls != 2 || proposals != 3 || primary.seq != a+1 || archive.seq != b+1 || r.Final != "" {
		t.Fatal("server switching reset budget", r, err)
	}
}

func TestRoutedLoopLocalDenialSendsNoRequest(t *testing.T) {
	router, primary, archive := readyRouter(t, false)
	a, b := primary.seq, archive.seq
	r, err := RunRoutedLoop(context.Background(), router, "question", scriptedDecision("archive::read_doc"))
	if err != nil || r.ToolFailures != 1 || primary.seq != a || archive.seq != b || r.History[2].Tool != "archive::read_doc" || r.History[2].Content != "not_authorized" {
		t.Fatal("route shared other server permission", r, err)
	}
}

func TestRoutedLoopCancellationRejectsProposalBeforeEitherServer(t *testing.T) {
	router, primary, archive := readyRouter(t, true)
	a, b := primary.seq, archive.seq
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := RunRoutedLoop(ctx, router, "question", func(context.Context, []LoopMessage) (LoopReply, error) {
		cancel()
		return LoopReply{Call: LoopCall{ID: "late", Tool: "archive::read_doc", DocumentID: "doc-A"}}, nil
	})
	if !errors.Is(err, context.Canceled) || r.ToolCalls != 0 || primary.seq != a || archive.seq != b || len(r.History) != 1 || r.Final != "" {
		t.Fatal("canceled proposal reached router", r, err)
	}
}

func TestRoutedLoopFailureDoesNotAutomaticallyChangeSource(t *testing.T) {
	for _, transport := range []bool{false, true} {
		t.Run(fmt.Sprint(transport), func(t *testing.T) {
			router, primary, archive := readyRouter(t, true)
			before := primary.seq
			id := "missing"
			if transport {
				archive.input.Close()
				id = "doc-A"
			}
			decisions := 0
			r, err := RunRoutedLoop(context.Background(), router, "question", func(_ context.Context, history []LoopMessage) (LoopReply, error) {
				decisions++
				if decisions == 1 {
					return LoopReply{Call: LoopCall{ID: "archive-1", Tool: "archive::read_doc", DocumentID: id}}, nil
				}
				if !history[2].Failed || history[2].Tool != "archive::read_doc" || history[2].CallID != "archive-1" {
					t.Fatal(history)
				}
				return LoopReply{Final: "Selected source failed."}, nil
			})
			if primary.seq != before || r.ToolCalls != 1 {
				t.Fatal("failure silently used primary", r, err)
			}
			if transport {
				if err == nil || decisions != 1 || r.Final != "" {
					t.Fatal("transport failure treated as normal observation", r, err)
				}
			} else if err != nil || decisions != 2 || r.ToolFailures != 1 {
				t.Fatal("business failure not preserved", r, err)
			}
		})
	}
}
