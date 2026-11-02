package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"
)

func TestActualProtocolRepairRecordPreservesEachFailedAttempt(t *testing.T) {
	data, err := os.ReadFile("testdata/local-observed-repair-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != "f354a7d87c84b7a7fd2b7d9cb42104a305755b53cd647df67d992b35362382aa" {
		t.Fatal("raw repair record changed")
	}
	var record LocalObservedRecord
	if decodeJSON(data, &record, 128<<10) != nil || !record.Complete || record.Mode != "protocol-repair" || record.Revision != localRevision || record.PromptVersion != "rag-joint-v3" || len(record.Generations) != 6 {
		t.Fatal("repair record identity")
	}
	previous := map[string]string{}
	attempts := map[string]int{}
	index := 0
	for _, run := range record.Archive.Runs {
		if run.FinalLimit != 2 {
			t.Fatal("missing archived final limit", run)
		}
		for _, call := range run.Calls {
			if call.Kind != "model-final" {
				continue
			}
			g := record.Generations[index]
			index++
			attempts[g.InputHash]++
			if g.Attempt != attempts[g.InputHash] || g.InputHash != call.InputHash || g.InputHash != hash(encoded(g.Observation)) || g.PreviousRaw != previous[g.InputHash] || !capturedLocalFeedbackInput(g.Observation, g.Generation, g.PreviousRaw) {
				t.Fatal("feedback/call binding", g)
			}
			answer, err := adaptLocalAnswer(g.Generation.Raw, g.Observation)
			if err != nil {
				err = errors.Join(ErrAnswerProtocol, err)
			}
			output := ""
			if err == nil {
				output = encoded(answer)
			}
			if output != call.Output || errorCode(err) != call.Error || call.Metrics.InputTokens != int64(g.Generation.InputTokens) || call.Metrics.OutputTokens != int64(g.Generation.OutputTokens) {
				t.Fatal("raw repair or erased metrics", call, g)
			}
			previous[g.InputHash] = g.Generation.Raw
		}
	}
	r, err := evaluate(context.Background(), record.Archive)
	if err != nil || r.Gate || r.Passed != 2 || r.ExecutionFailed != 2 || r.CriticalFailed != 2 || r.CallUsage.ModelCalls != 6 || r.CallUsage.KnownInputTokens != 1405 || r.CallUsage.KnownOutputTokens != 152 {
		t.Fatal(r, err)
	}
	for _, row := range r.Rows {
		if !row.ReplayVerified {
			t.Fatal("failed attempt did not replay", row)
		}
	}
}
