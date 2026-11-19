package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestActualLocalRecordReplaysWithoutModel(t *testing.T) {
	data, err := os.ReadFile("testdata/local-observed-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != "c94f29138d85c93543c2344c279d42db2f33c4ca9c6a630fd22b6b2fc15caabd" {
		t.Fatal("original actual record changed")
	}
	var record LocalObservedRecord
	if decodeJSON(data, &record, 128<<10) != nil || !record.Complete || record.Profile != "qwen3-1.7b" || record.Revision != localRevision || record.PromptVersion != "rag-joint-v3" || len(record.Generations) != 4 {
		t.Fatal("actual record metadata")
	}
	for n, g := range record.Generations {
		run := record.Archive.Runs[n]
		if len(run.Calls) != 3 || g.InputHash != hash(encoded(g.Observation)) || run.Calls[2].InputHash != g.InputHash || !capturedLocalInput(g.Observation, g.Generation) || g.Generation.AtOutputLimit {
			t.Fatal("actual input/call capture", n)
		}
		m := run.Calls[2].Metrics
		if m == nil || m.Origin != "model" || m.ModelRevision != localRevision || m.UsageSource != "local-tokenizer" || !m.UsageKnown || m.InputTokens != int64(g.Generation.InputTokens) || m.OutputTokens != int64(g.Generation.OutputTokens) {
			t.Fatal("usage association", n)
		}
		answer, adaptErr := adaptLocalAnswer(g.Generation.Raw, g.Observation)
		if run.Calls[2].Error != errorCode(adaptErr) {
			t.Fatal("adaptation error changed", n)
		}
		output := ""
		if adaptErr == nil {
			output = encoded(answer)
		}
		if output != run.Calls[2].Output {
			t.Fatal("raw generation repaired or adaptation changed", n)
		}
	}
	report, err := evaluate(context.Background(), record.Archive)
	if err != nil || report.Total != 4 || report.Passed != 2 || report.ExecutionFailed != 2 || report.CriticalFailed != 2 || report.Gate || !report.UsageKnown || report.CostKnown || report.CallUsage.ModelCalls != 4 || report.CallUsage.KnownInputTokens != 859 || report.CallUsage.KnownOutputTokens != 114 {
		t.Fatal(report, err)
	}
	for _, row := range report.Rows {
		if !row.ReplayVerified {
			t.Fatal("actual failure did not replay", row)
		}
	}
}

func TestCapturedLocalRequestRejectsHiddenLabelsAndWrongEvidence(t *testing.T) {
	o := Observation{Doc: "timeout", Version: "v1", Text: "30 seconds"}
	request := localAnswerRequest(o)
	expected := map[string]any{"question": request["question"], "evidence": []map[string]string{{"evidence_id": o.Doc, "version": o.Version, "text": o.Text}}}
	data, _ := json.Marshal(expected)
	row := LocalGeneration{}
	row.InputMessages = append(row.InputMessages, struct{ Role, Content string }{"system", "instructions"}, struct{ Role, Content string }{"user", string(data)})
	if !capturedLocalInput(o, row) {
		t.Fatal("valid capture rejected")
	}
	expected["Expected"] = "hidden"
	data, _ = json.Marshal(expected)
	row.InputMessages[1].Content = string(data)
	if capturedLocalInput(o, row) {
		t.Fatal("hidden label accepted")
	}
}
