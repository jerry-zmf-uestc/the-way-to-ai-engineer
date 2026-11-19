package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

var ErrInput = errors.New("invalid neural retrieval contract")

const revision = "1110a243fdf4706b3f48f1d95db1a4f5529b4d41"
const space = "minilm-l6-v2-onnx-mean-l2-256-v1"

type Batch struct {
	Revision, Space string
	Dimension       int
	Truncated       []bool
	Vectors         [][]float64
}
type Document struct{ ID, Tenant, Version, Text string }
type Question struct{ ID, Text string }
type Label struct {
	QuestionID string
	Relevant   []string
}
type Hit struct {
	ID    string
	Score float64
}
type CaseResult struct {
	ID                         string
	Lexical, Dense             []Hit
	LexicalRecall, DenseRecall float64
	RecallKnown, Truncated     bool
}

// A child result buffer is bounded even if the local program misbehaves.
type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, ErrInput
	}
	return b.Buffer.Write(p)
}
func Validate(batch Batch, count int) error {
	if count < 1 || count > 32 || batch.Revision != revision || batch.Space != space || batch.Dimension != 384 || len(batch.Vectors) != count || len(batch.Truncated) != count {
		return ErrInput
	}
	for _, vec := range batch.Vectors {
		if len(vec) != 384 {
			return ErrInput
		}
		norm := 0.0
		for _, v := range vec {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return ErrInput
			}
			norm += v * v
		}
		if math.Abs(norm-1) > .001 {
			return ErrInput
		}
	}
	return nil
}
func Encode(ctx context.Context, python, assets string, texts []string) (Batch, error) {
	if !filepath.IsAbs(python) || !filepath.IsAbs(assets) || len(texts) == 0 || len(texts) > 32 {
		return Batch{}, ErrInput
	}
	for _, text := range texts {
		if strings.TrimSpace(text) == "" || len(text) > 4096 {
			return Batch{}, ErrInput
		}
	}
	_, file, _, _ := runtime.Caller(0)
	data, err := json.Marshal(map[string]any{"texts": texts})
	if err != nil {
		return Batch{}, err
	}
	child, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(child, python, filepath.Join(filepath.Dir(file), "encoder.py"), assets)
	cmd.Stdin = bytes.NewReader(data)
	output := &cappedBuffer{limit: 2 << 20}
	cmd.Stdout = output
	stderr := &cappedBuffer{limit: 4096}
	cmd.Stderr = stderr
	if err = cmd.Run(); err != nil {
		if child.Err() != nil {
			return Batch{}, child.Err()
		}
		return Batch{}, errors.New("local encoder execution failed")
	}
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	decoder.DisallowUnknownFields()
	var batch Batch
	if err = decoder.Decode(&batch); err != nil {
		return Batch{}, ErrInput
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return Batch{}, ErrInput
	}
	if err = Validate(batch, len(texts)); err != nil {
		return Batch{}, err
	}
	return batch, nil
}
func eligible(d Document, tenant, version string) bool {
	return d.Tenant == tenant && d.Version == version
}
func Lexical(docs []Document, q, tenant, version string, k int) []Hit {
	words := strings.Fields(strings.ToLower(q))
	hits := []Hit{}
	for _, doc := range docs {
		if !eligible(doc, tenant, version) {
			continue
		}
		tokens := strings.Fields(strings.ToLower(doc.Text))
		score := 0.0
		for _, w := range words {
			for _, t := range tokens {
				if strings.Trim(t, ".,;:!?-") == strings.Trim(w, ".,;:!?-") {
					score++
					break
				}
			}
		}
		hits = append(hits, Hit{ID: doc.ID, Score: score})
	}
	return rank(hits, k)
}
func Dense(docs []Document, vectors [][]float64, query []float64, tenant, version string, k int) ([]Hit, error) {
	if len(vectors) != len(docs) || len(query) != 384 || k < 1 || k > len(docs) {
		return nil, ErrInput
	}
	hits := []Hit{}
	for n, doc := range docs {
		if !eligible(doc, tenant, version) {
			continue
		}
		if len(vectors[n]) != len(query) {
			return nil, ErrInput
		}
		score := 0.0
		for dim, v := range vectors[n] {
			score += v * query[dim]
		}
		hits = append(hits, Hit{ID: doc.ID, Score: score})
	}
	return rank(hits, k), nil
}
func rank(hits []Hit, k int) []Hit {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	return hits[:min(k, len(hits))]
}
func Recall(label Label, hits []Hit) (float64, bool, error) {
	if len(label.Relevant) == 0 {
		return 0, false, nil
	}
	ids := map[string]bool{}
	for _, id := range label.Relevant {
		if id == "" || ids[id] {
			return 0, false, ErrInput
		}
		ids[id] = true
	}
	seen := map[string]bool{}
	got := 0
	for _, hit := range hits {
		if seen[hit.ID] {
			return 0, false, ErrInput
		}
		seen[hit.ID] = true
		if ids[hit.ID] {
			got++
		}
	}
	return float64(got) / float64(len(ids)), true, nil
}
func fixtures() ([]Document, []Question, []Label) {
	docs := []Document{
		{ID: "timeout", Tenant: "demo", Version: "v1", Text: "The request timeout is the maximum duration a network operation may wait before it is canceled."},
		{ID: "retry", Tenant: "demo", Version: "v1", Text: "Retry attempts repeat a failed call with exponential backoff and a bounded attempt limit."},
		{ID: "cancel", Tenant: "demo", Version: "v1", Text: "A cancellation signal must be passed to downstream operations so that they can stop cooperatively."},
		{ID: "cache", Tenant: "demo", Version: "v1", Text: "Cache entries store reusable results and expire after a time to live. Stale data must be invalidated."},
		{ID: "citation", Tenant: "demo", Version: "v1", Text: "A citation identifies the source document. Check that quoted material supports the answer's claim."},
		{ID: "weather", Tenant: "demo", Version: "v1", Text: "Weather reports describe temperature, wind, rain, and cloud cover."},
		{ID: "old-timeout", Tenant: "demo", Version: "old", Text: "The request timeout is the maximum duration a network operation may wait before it is canceled."},
		{ID: "other-user", Tenant: "other", Version: "v1", Text: "Retry attempts repeat a failed call with exponential backoff and a bounded attempt limit."},
	}
	questions := []Question{
		{ID: "duration", Text: "What setting controls how long an operation is allowed to wait?"},
		{ID: "retry", Text: "How can another attempt be made after a transient failure without looping forever?"},
		{ID: "cancel", Text: "How does a downstream operation learn that it should halt?"},
		{ID: "cache", Text: "When should a previously saved result no longer be reused?"},
		{ID: "citation", Text: "How can I check whether an answer is backed by its referenced material?"},
		{ID: "unsupported", Text: "Can this assistant buy a flight ticket and charge a credit card?"},
	}
	labels := []Label{{"duration", []string{"timeout"}}, {"retry", []string{"retry"}}, {"cancel", []string{"cancel"}}, {"cache", []string{"cache"}}, {"citation", []string{"citation"}}, {"unsupported", nil}}
	return docs, questions, labels
}
func Evaluate(docs []Document, questions []Question, labels []Label, batch Batch) ([]CaseResult, error) {
	if len(questions) != len(labels) || len(docs)+len(questions) > 32 {
		return nil, ErrInput
	}
	if err := Validate(batch, len(docs)+len(questions)); err != nil {
		return nil, err
	}
	rows := []CaseResult{}
	for n, q := range questions {
		if labels[n].QuestionID != q.ID {
			return nil, ErrInput
		}
		lex := Lexical(docs, q.Text, "demo", "v1", 1)
		dense, err := Dense(docs, batch.Vectors[:len(docs)], batch.Vectors[len(docs)+n], "demo", "v1", 1)
		if err != nil {
			return nil, err
		}
		lr, known, err := Recall(labels[n], lex)
		if err != nil {
			return nil, err
		}
		dr, _, err := Recall(labels[n], dense)
		if err != nil {
			return nil, err
		}
		rows = append(rows, CaseResult{ID: q.ID, Lexical: lex, Dense: dense, LexicalRecall: lr, DenseRecall: dr, RecallKnown: known, Truncated: batch.Truncated[len(docs)+n]})
	}
	return rows, nil
}
func main() {
	export := flag.Bool("export-evidence", false, "export top1 original candidates as JSON")
	fallback := flag.Bool("query-fallback", false, "show fixed query expansion fallback")
	flag.Parse()
	if *fallback {
		if *export || flag.NArg() != 0 {
			panic(ErrInput)
		}
		queryFallbackDemo()
		return
	}
	docs, questions, labels := fixtures()
	if os.Getenv("EMBEDDING_E2E") != "1" {
		if *export {
			panic("export requires EMBEDDING_E2E=1")
		}
		fmt.Printf("documents=%d questions=%d pretrained_embedding=not_run\n", len(docs), len(questions))
		return
	}
	if *export {
		filtered := []Document{}
		for _, d := range docs {
			if eligible(d, "demo", "v1") {
				filtered = append(filtered, d)
			}
		}
		docs = filtered
	}
	texts := []string{}
	for _, d := range docs {
		texts = append(texts, d.Text)
	}
	for _, q := range questions {
		texts = append(texts, q.Text)
	}
	batch, err := Encode(context.Background(), os.Getenv("EMBEDDING_PYTHON"), os.Getenv("EMBEDDING_ASSETS"), texts)
	if err != nil {
		panic(err)
	}
	if *export {
		payload, err := ExportCandidates(context.Background(), docs, questions, batch)
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(payload); err != nil {
			panic(err)
		}
		return
	}
	rows, err := Evaluate(docs, questions, labels, batch)
	if err != nil {
		panic(err)
	}
	for _, row := range rows {
		fmt.Printf("case=%s lexical=%s dense=%s recall_known=%t lexical_recall=%.0f dense_recall=%.0f truncated=%t\n", row.ID, row.Lexical[0].ID, row.Dense[0].ID, row.RecallKnown, row.LexicalRecall, row.DenseRecall, row.Truncated)
	}
	fmt.Printf("revision=%s dimension=%d space=%s\n", batch.Revision, batch.Dimension, batch.Space)
}
