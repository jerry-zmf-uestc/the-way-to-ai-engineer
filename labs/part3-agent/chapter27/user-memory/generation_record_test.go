package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestMemoryInputGenerationRecordsPreserveActualFailures(t *testing.T) {
	for _, item := range []struct {
		path, hash    string
		schema, calls int
		states        []string
	}{
		{"testdata/input-generation-v1.json", "c97663dc8dc3db2b0f2855bf5ad6aff883db9f0d40acf077788333a4b1c22699", 1, 0, []string{"updated-stale-cache", "deleted", "foreign-user"}},
		{"testdata/input-generation-v2.json", "0c8ed030a02a08bd3f2983537d4a8bf2ed127434da04d12f1e8f7c24b08ddea4", 2, 3, []string{"updated-stale-cache", "foreign-user", "deleted"}},
	} {
		t.Run(item.path, func(t *testing.T) {
			data, err := os.ReadFile(item.path)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(data)
			if hex.EncodeToString(digest[:]) != item.hash {
				t.Fatal("original record changed")
			}
			var r memoryGenerationRecord
			if json.Unmarshal(data, &r) != nil || r.Schema != item.schema || r.GenerationCalls != item.calls || !reflect.DeepEqual(r.States, item.states) || r.CachedRevision != 1 || r.CurrentRevision != 2 || len(r.Submitted) != 3 || len(r.Batch.Rows) != 3 {
				t.Fatal("record identity")
			}
			if r.Batch.Profile != "qwen3-1.7b" || r.Batch.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" || r.Batch.PromptVersion != "rag-joint-v3" {
				t.Fatal("model identity")
			}
			submitted, _ := json.Marshal(r.Submitted)
			inputHash := sha256.Sum256(submitted)
			if hex.EncodeToString(inputHash[:]) != r.InputSHA256 {
				t.Fatal("submitted input changed")
			}
			expected := []memoryPromptRequest{memoryRequest([]MemoryInput{{Key: "response-style", Value: "detailed", Revision: 2}}), memoryRequest(nil), memoryRequest(nil)}
			if !reflect.DeepEqual(r.Submitted, expected) {
				t.Fatal("stale or storage data entered submitted input")
			}
			results := make([]bool, 3)
			in, out := 0, 0
			for n, row := range r.Batch.Rows {
				if row.AtOutputLimit || len(row.InputMessages) != 2 || row.InputMessages[1].Role != "user" {
					t.Fatal("incomplete original generation")
				}
				var sent struct {
					Question string `json:"question"`
					Evidence []struct {
						ID      string `json:"evidence_id"`
						Version string `json:"version"`
						Text    string `json:"text"`
					} `json:"evidence"`
				}
				if json.Unmarshal([]byte(row.InputMessages[1].Content), &sent) != nil || sent.Question != expected[n].Question || len(sent.Evidence) != len(expected[n].Evidence) {
					t.Fatal("actual messages changed")
				}
				for k, e := range sent.Evidence {
					v := expected[n].Evidence[k]
					if e.ID != v.ID || e.Version != v.Version || e.Text != v.Text {
						t.Fatal("actual evidence changed")
					}
				}
				results[n] = exactMemoryOutput(row.Raw, r.Submitted[n])
				in += row.InputTokens
				out += row.OutputTokens
			}
			if !reflect.DeepEqual(results, []bool{false, true, true}) || !reflect.DeepEqual(results, r.ExactResults) || in != 579 || out != 54 {
				t.Fatal("original failures or usage hidden", results, in, out)
			}
		})
	}
}

func TestMemoryKeyValueRecordDoesNotHideValueAndProtocolFailure(t *testing.T) {
	data, err := os.ReadFile("testdata/input-generation-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != "89a62f4a5f85bb7c053195c16da73c41610659955cfe409882e32d4000389fb1" {
		t.Fatal("original view record changed")
	}
	var r memoryGenerationRecord
	if json.Unmarshal(data, &r) != nil || r.Schema != 3 || r.InputView != "key-value" || r.GenerationCalls != 3 || len(r.Batch.Rows) != 3 || r.Batch.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" || r.Batch.PromptVersion != "rag-joint-v3" {
		t.Fatal("view record identity")
	}
	want := []memoryPromptRequest{memoryRequestView([]MemoryInput{{Key: "response-style", Value: "detailed", Revision: 2}}, "key-value"), memoryRequest(nil), memoryRequest(nil)}
	if !reflect.DeepEqual(r.Submitted, want) {
		t.Fatal("view task changed")
	}
	payload, _ := json.Marshal(r.Submitted)
	inputHash := sha256.Sum256(payload)
	if hex.EncodeToString(inputHash[:]) != r.InputSHA256 {
		t.Fatal("submitted digest changed")
	}
	results := make([]bool, 3)
	in, out := 0, 0
	for n, row := range r.Batch.Rows {
		if row.AtOutputLimit || len(row.InputMessages) != 2 || row.InputMessages[1].Role != "user" {
			t.Fatal("generation incomplete")
		}
		var sent struct {
			Question string `json:"question"`
			Evidence []struct {
				ID      string `json:"evidence_id"`
				Version string `json:"version"`
				Text    string `json:"text"`
			} `json:"evidence"`
		}
		if json.Unmarshal([]byte(row.InputMessages[1].Content), &sent) != nil || sent.Question != want[n].Question || len(sent.Evidence) != len(want[n].Evidence) {
			t.Fatal("actual input drift")
		}
		for k, e := range sent.Evidence {
			original := want[n].Evidence[k]
			if e.ID != original.ID || e.Version != original.Version || e.Text != original.Text {
				t.Fatal("actual evidence drift")
			}
		}
		results[n] = exactMemoryOutput(row.Raw, r.Submitted[n])
		in += row.InputTokens
		out += row.OutputTokens
	}
	if !reflect.DeepEqual(results, []bool{false, true, true}) || !reflect.DeepEqual(results, r.ExactResults) || in != 581 || out != 77 {
		t.Fatal("failure or usage hidden", results, in, out)
	}
}
