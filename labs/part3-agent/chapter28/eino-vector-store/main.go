package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	embedding "github.com/cloudwego/eino/components/embedding"
)

var (
	ErrEmptyID           = errors.New("eino vector store: id is empty")
	ErrEmptyText         = errors.New("eino vector store: text is empty")
	ErrEmptyVector       = errors.New("eino vector store: vector is empty")
	ErrDimensionMismatch = errors.New("eino vector store: vector dimensions differ")
	ErrLimitInvalid      = errors.New("eino vector store: limit must be positive")
)

type DocInput struct {
	ID       string
	Text     string
	Metadata map[string]string
}

type SearchHit struct {
	ID    string
	Text  string
	Score float64
}

type storedDoc struct {
	ID       string
	Text     string
	Metadata map[string]string
	Vector   []float64
}

type HashEmbedder struct {
	Dimensions int
}

var _ embedding.Embedder = HashEmbedder{}

func (h HashEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	dims := h.Dimensions
	if dims <= 0 {
		dims = 8
	}

	out := make([][]float64, 0, len(texts))
	for _, text := range texts {
		normalized := normalizeText(text)
		if normalized == "" {
			return nil, ErrEmptyText
		}

		vector := make([]float64, dims)
		for _, token := range strings.Fields(normalized) {
			vector[stableBucket(token, dims)]++
		}
		out = append(out, unit(vector))
	}
	return out, nil
}

type MemoryStore struct {
	dimensions int
	docs       map[string]storedDoc
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{docs: make(map[string]storedDoc)}
}

func (s *MemoryStore) Upsert(doc storedDoc) error {
	if strings.TrimSpace(doc.ID) == "" {
		return ErrEmptyID
	}
	if err := s.checkVector(doc.Vector); err != nil {
		return fmt.Errorf("upsert %q: %w", doc.ID, err)
	}

	copied := doc
	copied.Vector = append([]float64(nil), doc.Vector...)
	copied.Metadata = copyMetadata(doc.Metadata)
	s.docs[doc.ID] = copied
	return nil
}

func (s *MemoryStore) Search(query []float64, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		return nil, ErrLimitInvalid
	}
	if err := s.checkVector(query); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	hits := make([]SearchHit, 0, len(s.docs))
	for _, doc := range s.docs {
		score, err := cosine(query, doc.Vector)
		if err != nil {
			return nil, err
		}
		hits = append(hits, SearchHit{ID: doc.ID, Text: doc.Text, Score: score})
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})

	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (s *MemoryStore) checkVector(vector []float64) error {
	if len(vector) == 0 {
		return ErrEmptyVector
	}
	if s.dimensions == 0 {
		s.dimensions = len(vector)
		return nil
	}
	if len(vector) != s.dimensions {
		return ErrDimensionMismatch
	}
	return nil
}

type VectorProject struct {
	embedder HashEmbedder
	store    *MemoryStore
}

func NewVectorProject(embedder HashEmbedder, store *MemoryStore) *VectorProject {
	return &VectorProject{embedder: embedder, store: store}
}

func (p *VectorProject) IndexAndSearch(ctx context.Context, docs []DocInput, query string, topK int) ([]SearchHit, error) {
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Text)
	}

	vectors, err := p.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, err
	}

	for i, doc := range docs {
		if err := p.store.Upsert(storedDoc{
			ID:       doc.ID,
			Text:     doc.Text,
			Metadata: doc.Metadata,
			Vector:   vectors[i],
		}); err != nil {
			return nil, err
		}
	}

	queryVector, err := p.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	return p.store.Search(queryVector[0], topK)
}

func runDemo(ctx context.Context) (string, error) {
	project := NewVectorProject(HashEmbedder{Dimensions: 8}, NewMemoryStore())
	docs := []DocInput{
		{ID: "rag", Text: "rag retrieves relevant context before answering"},
		{ID: "vector", Text: "vector stores keep embedded documents for search"},
		{ID: "http", Text: "http handlers validate requests before calling services"},
	}
	query := "relevant rag context"

	results, err := project.IndexAndSearch(ctx, docs, query, 1)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", errors.New("eino vector store: no results")
	}

	return fmt.Sprintf("indexed=%d query=%q top_hit=%s score=%.2f", len(docs), query, results[0].ID, results[0].Score), nil
}

func normalizeText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

func stableBucket(token string, dimensions int) int {
	var sum int
	for _, r := range token {
		sum += int(r)
	}
	return sum % dimensions
}

func unit(vector []float64) []float64 {
	var squared float64
	for _, v := range vector {
		squared += v * v
	}
	if squared == 0 {
		return vector
	}

	length := math.Sqrt(squared)
	out := make([]float64, len(vector))
	for i, v := range vector {
		out[i] = v / length
	}
	return out
}

func cosine(a, b []float64) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, ErrEmptyVector
	}
	if len(a) != len(b) {
		return 0, ErrDimensionMismatch
	}

	var dot, lenA, lenB float64
	for i := range a {
		dot += a[i] * b[i]
		lenA += a[i] * a[i]
		lenB += b[i] * b[i]
	}
	if lenA == 0 || lenB == 0 {
		return 0, ErrEmptyVector
	}

	return dot / (math.Sqrt(lenA) * math.Sqrt(lenB)), nil
}

func copyMetadata(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}

	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func main() {
	out, err := runDemo(context.Background())
	if err != nil {
		fmt.Println("run demo:", err)
		return
	}

	fmt.Println(out)
}
