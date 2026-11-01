package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type memoryPromptEvidence struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Text    string `json:"text"`
}
type memoryPromptRequest struct {
	Question string                 `json:"question"`
	Evidence []memoryPromptEvidence `json:"evidence"`
}
type memoryGeneratedRow struct {
	Raw           string `json:"raw"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	AtOutputLimit bool   `json:"at_output_limit"`
	InputMessages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"input_messages"`
}
type memoryGeneratedBatch struct {
	Profile       string               `json:"profile"`
	Revision      string               `json:"revision"`
	PromptVersion string               `json:"prompt_version"`
	Rows          []memoryGeneratedRow `json:"rows"`
}
type memoryGenerationRecord struct {
	Schema          int                   `json:"schema"`
	States          []string              `json:"states"`
	Submitted       []memoryPromptRequest `json:"submitted"`
	InputSHA256     string                `json:"input_sha256"`
	CachedRevision  int64                 `json:"cached_revision"`
	CurrentRevision int64                 `json:"current_revision"`
	Batch           memoryGeneratedBatch  `json:"batch"`
	ExactResults    []bool                `json:"exact_results"`
	InputView       string                `json:"input_view,omitempty"`
	GenerationCalls int                   `json:"generation_calls,omitempty"`
}
type memoryBoundedOutput struct {
	bytes.Buffer
	limit int
}

func (b *memoryBoundedOutput) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}

func memoryRequest(input []MemoryInput) memoryPromptRequest {
	r := memoryPromptRequest{Question: "Return the response-style value verbatim from the evidence. If there is no evidence, report insufficient evidence.", Evidence: []memoryPromptEvidence{}}
	for _, item := range input {
		r.Evidence = append(r.Evidence, memoryPromptEvidence{ID: item.Key, Version: strconv.FormatInt(item.Revision, 10), Text: item.Value})
	}
	return r
}
func memoryRequestView(input []MemoryInput, view string) memoryPromptRequest {
	request := memoryRequest(input)
	if view == "key-value" {
		for n, item := range input {
			request.Evidence[n].Text = item.EvidenceText()
		}
	}
	return request
}
func exactMemoryOutput(raw string, request memoryPromptRequest) bool {
	var expected any
	if len(request.Evidence) == 0 {
		expected = map[string]any{"abstained": true, "reason": "insufficient_evidence", "claims": []any{}}
	} else {
		e := request.Evidence[0]
		value := e.Text
		if e.ID == "response-style" {
			if key, candidate, ok := strings.Cut(e.Text, "="); ok && key == e.ID && (candidate == "concise" || candidate == "detailed") {
				value = candidate
			}
		}
		expected = map[string]any{"claims": []any{map[string]any{"text": value, "citations": []any{map[string]any{"evidence_id": e.ID, "version": e.Version, "quote": e.Text}}}}}
	}
	// Compare the complete value and token count; extra or duplicate fields cannot be normalized away.
	want, _ := json.Marshal(expected)
	var actual map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &actual) != nil {
		return false
	}
	normalized, _ := json.Marshal(actual)
	var object map[string]any
	if json.Unmarshal(normalized, &object) != nil {
		return false
	}
	normalized, _ = json.Marshal(object)
	// A token count check detects duplicate object fields before normalization.
	decoder := json.NewDecoder(strings.NewReader(raw))
	fields := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		if text, ok := token.(string); ok && text != "" {
			fields++
		}
	}
	reference := json.NewDecoder(bytes.NewReader(want))
	expectedFields := 0
	for {
		token, err := reference.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		if text, ok := token.(string); ok && text != "" {
			expectedFields++
		}
	}
	return fields == expectedFields && bytes.Equal(normalized, want)
}

func TestActualMemoryInputGeneration(t *testing.T) {
	if os.Getenv("MEMORY_INPUT_GENERATION_E2E") != "1" {
		t.Skip("explicit isolated storage and prepared generation only")
	}
	for _, name := range []string{"MEMORY_E2E", "MEMORY_TEST_DATABASE", "MEMORY_REDIS_E2E", "RAG_GENERATION_E2E"} {
		if os.Getenv(name) != "1" {
			t.Fatal("required gate absent", name)
		}
	}
	view := os.Getenv("MEMORY_INPUT_VIEW")
	if view != "" && view != "key-value" {
		t.Fatal("registered input view required")
	}
	python, assets, path := os.Getenv("GENERATION_PYTHON"), os.Getenv("GENERATION_ASSETS"), os.Getenv("MEMORY_INPUT_RECORD_PATH")
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) || !filepath.IsAbs(path) {
		t.Fatal("trusted absolute runtime/assets/new record required")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unused exclusive record path required")
	}
	generator, err := filepath.Abs("../../chapter28/evidence-answer/local_generator.py")
	if err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(filepath.Dir(generator), "generation-requirements.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	runtimeCheck := `import sys,importlib.metadata as m;from pathlib import Path;assert sys.version_info[:3]==(3,12,14);[None if m.version(n)==v else (_ for _ in ()).throw(ValueError(n)) for n,v in [line.split('==') for line in Path(sys.argv[1]).read_text().splitlines() if line and not line.startswith('#')]]`
	if err := exec.CommandContext(ctx, python, "-c", runtimeCheck, lock).Run(); err != nil {
		t.Fatal("locked generation runtime required")
	}
	db, err := sql.Open("pgx", os.Getenv("MEMORY_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	var database string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database); err != nil || database != "memory_lab" {
		t.Fatal("isolated memory_lab required")
	}
	store := SQLStore{db}
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	command, _ := isolatedRedis(t)
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	scope := Scope{"generation-" + t.Name(), "reader"}
	memory := Memory{store, func() time.Time { return now }}
	c := Consent{Scope: scope, Key: "response-style", Operation: "write", Value: "concise", Allowed: true, ExpiresAt: now.Add(time.Hour)}
	first, err := memory.Write(ctx, scope, c.Key, c.Value, "confirmation-1", 0, time.Hour, c)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(first)
	key := redisCacheKey(scope, c.Key)
	if result, err := command(ctx, "SET", key, string(payload), "PX", "300000"); err != nil || result != "OK" {
		t.Fatal("cache setup failed")
	}
	c.Value = "detailed"
	current, err := memory.Write(ctx, scope, c.Key, c.Value, "confirmation-2", first.Revision, time.Hour, c)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := command(ctx, "GET", key)
	var cached Record
	if err != nil || json.Unmarshal([]byte(raw), &cached) != nil || cached.Value != "concise" || cached.Revision != 1 {
		t.Fatal("stale cache not observed")
	}
	input, err := memory.Input(ctx, scope, []string{cached.Key})
	if err != nil || len(input) != 1 || input[0].Value != "detailed" || input[0].Revision != 2 {
		t.Fatal("current input missing")
	}
	generate := func(request memoryPromptRequest) memoryGeneratedBatch {
		data, err := json.Marshal([]memoryPromptRequest{request})
		if err != nil || len(data) > 64<<10 {
			t.Fatal("payload bound")
		}
		output := &memoryBoundedOutput{limit: 128 << 10}
		stderr := &memoryBoundedOutput{limit: 4096}
		cmd := exec.CommandContext(ctx, python, generator, assets, "qwen3-1.7b", "rag-joint-v3")
		cmd.Stdin = bytes.NewReader(data)
		cmd.Stdout = output
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			t.Fatal("local generation failed", err)
		}
		var batch memoryGeneratedBatch
		decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&batch) != nil || batch.Profile != "qwen3-1.7b" || batch.PromptVersion != "rag-joint-v3" || batch.Revision != "70d244cc86ccca08cf5af4e1e306ecf908b1ad5e" || len(batch.Rows) != 1 {
			t.Fatal("generation protocol identity")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			t.Fatal("trailing output")
		}
		return batch
	}
	requests := []memoryPromptRequest{memoryRequestView(input, view)}
	batch := generate(requests[0])
	// Check another user while the original user's current fact still exists.
	input, err = memory.Input(ctx, Scope{scope.Tenant, "other"}, []string{cached.Key})
	if err != nil || len(input) != 0 {
		t.Fatal("foreign input returned")
	}
	requests = append(requests, memoryRequestView(input, view))
	batch.Rows = append(batch.Rows, generate(requests[1]).Rows...)
	c.Operation = "delete"
	if _, err := memory.Delete(ctx, scope, c.Key, current.Revision, c); err != nil {
		t.Fatal(err)
	}
	input, err = memory.Input(ctx, scope, []string{cached.Key})
	if err != nil || len(input) != 0 {
		t.Fatal("deleted input returned")
	}
	requests = append(requests, memoryRequestView(input, view))
	batch.Rows = append(batch.Rows, generate(requests[2]).Rows...)
	payload, err = json.Marshal(requests)
	if err != nil {
		t.Fatal(err)
	}
	results := make([]bool, 3)
	for n, row := range batch.Rows {
		if row.InputTokens < 1 || row.InputTokens > 1024 || row.OutputTokens < 1 || row.OutputTokens > 192 || row.AtOutputLimit != (row.OutputTokens == 192) || len(row.Raw) > 16384 || len(row.InputMessages) != 2 || row.InputMessages[0].Role != "system" || row.InputMessages[1].Role != "user" {
			t.Fatal("generation row bound")
		}
		var sent struct {
			Question string `json:"question"`
			Evidence []struct {
				ID      string `json:"evidence_id"`
				Version string `json:"version"`
				Text    string `json:"text"`
			} `json:"evidence"`
		}
		if json.Unmarshal([]byte(row.InputMessages[1].Content), &sent) != nil || sent.Question != requests[n].Question || len(sent.Evidence) != len(requests[n].Evidence) {
			t.Fatal("submitted input differs")
		}
		for k, e := range sent.Evidence {
			original := requests[n].Evidence[k]
			if e.ID != original.ID || e.Version != original.Version || e.Text != original.Text {
				t.Fatal("evidence input differs")
			}
		}
		results[n] = !row.AtOutputLimit && exactMemoryOutput(row.Raw, requests[n])
	}
	digest := sha256.Sum256(payload)
	schema := 2
	if view == "key-value" {
		schema = 3
	}
	record := memoryGenerationRecord{Schema: schema, InputView: view, GenerationCalls: 3, States: []string{"updated-stale-cache", "foreign-user", "deleted"}, Submitted: requests, InputSHA256: hex.EncodeToString(digest[:]), CachedRevision: cached.Revision, CurrentRevision: current.Revision, Batch: batch, ExactResults: results}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal("new exclusive record required")
	}
	_, err = file.Write(append(data, '\n'))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal("record write failed")
	}
	t.Logf("actual generation recorded: rows=3 exact_results=%v; infrastructure pass is not model quality", results)
}

func TestMemoryGenerationProjectionContract(t *testing.T) {
	request := memoryRequest([]MemoryInput{{Key: "response-style", Value: "detailed", Revision: 2}})
	good := `{"claims":[{"text":"detailed","citations":[{"evidence_id":"response-style","version":"2","quote":"detailed"}]}]}`
	if !exactMemoryOutput(good, request) {
		t.Fatal("exact current value rejected")
	}
	for _, bad := range []string{strings.Replace(good, "detailed", "concise", 1), strings.Replace(good, `"version":"2"`, `"version":"1"`, 1), strings.Replace(good, `"text":"detailed"`, `"text":"detailed","text":"detailed"`, 1), good + ">"} {
		if exactMemoryOutput(bad, request) {
			t.Fatal("invalid answer accepted")
		}
	}
	empty := memoryRequest(nil)
	if !exactMemoryOutput(`{"abstained":true,"reason":"insufficient_evidence","claims":[]}`, empty) || exactMemoryOutput(good, empty) {
		t.Fatal("missing evidence contract")
	}
	data, err := json.Marshal(request)
	if err != nil || bytes.Contains(data, []byte("Tenant")) || bytes.Contains(data, []byte("Source")) {
		t.Fatal("storage fields reached generation")
	}
}

func TestMemoryKeyValueViewKeepsMeaningAndOriginalValue(t *testing.T) {
	input := []MemoryInput{{Key: "response-style", Value: "detailed", Revision: 2}}
	plain := memoryRequest(input)
	view := memoryRequestView(input, "key-value")
	if plain.Question != view.Question || plain.Evidence[0].Text != "detailed" || view.Evidence[0].Text != "response-style=detailed" || view.Evidence[0].Version != "2" || input[0].Value != "detailed" {
		t.Fatal("view changed task or original fact")
	}
	raw := `{"claims":[{"text":"detailed","citations":[{"evidence_id":"response-style","version":"2","quote":"response-style=detailed"}]}]}`
	if !exactMemoryOutput(raw, view) {
		t.Fatal("correct original value rejected")
	}
	for _, bad := range []string{strings.Replace(raw, `"text":"detailed"`, `"text":"concise"`, 1), strings.Replace(raw, `"version":"2"`, `"version":"1"`, 1), strings.Replace(raw, `"quote":"response-style=detailed"`, `"quote":"response-style=concise"`, 1)} {
		if exactMemoryOutput(bad, view) {
			t.Fatal("wrong value or citation accepted")
		}
	}
}
