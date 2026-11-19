package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHashEmbedderImplementsBatchEmbedding(t *testing.T) {
	t.Parallel()

	embedder := HashEmbedder{Dimensions: 4}
	vectors, err := embedder.EmbedStrings(context.Background(), []string{"go", "eino"})
	if err != nil {
		t.Fatalf("EmbedStrings() error = %v", err)
	}
	if len(vectors) != 2 || len(vectors[0]) != 4 {
		t.Fatalf("unexpected embedding shape: %#v", vectors)
	}
}

func TestIndexDocumentsAndSearch(t *testing.T) {
	t.Parallel()

	docs := []DocInput{
		{ID: "rag", Text: "rag retrieves relevant context"},
		{ID: "http", Text: "http handlers validate requests"},
	}
	project := NewVectorProject(HashEmbedder{Dimensions: 8}, NewMemoryStore())
	results, err := project.IndexAndSearch(context.Background(), docs, "relevant rag", 1)
	if err != nil {
		t.Fatalf("IndexAndSearch() error = %v", err)
	}
	if len(results) != 1 || results[0].ID != "rag" {
		t.Fatalf("top result = %#v, want rag", results)
	}
}

func TestRunDemoIncludesStoreSummary(t *testing.T) {
	t.Parallel()

	out, err := runDemo(context.Background())
	if err != nil {
		t.Fatalf("runDemo() error = %v", err)
	}
	for _, want := range []string{"indexed=", "query=", "top_hit="} {
		if !strings.Contains(out, want) {
			t.Fatalf("runDemo() missing %q in %q", want, out)
		}
	}
}

func TestSearchDistinguishesEmptyResultsFromInvalidRequests(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	results, err := store.Search([]float64{1, 0}, 3)
	if err != nil {
		t.Fatalf("Search() empty store error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search() empty store results = %#v, want empty", results)
	}

	if _, err := store.Search([]float64{1, 0}, 0); !errors.Is(err, ErrLimitInvalid) {
		t.Fatalf("Search() invalid limit error = %v, want ErrLimitInvalid", err)
	}

	if err := store.Upsert(storedDoc{ID: "doc", Vector: []float64{1, 0}}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if _, err := store.Search([]float64{1, 0, 0}, 1); !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("Search() dimension error = %v, want ErrDimensionMismatch", err)
	}
}

func TestQueryEmbeddingFailureDoesNotMeanIndexRollback(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	project := NewVectorProject(HashEmbedder{Dimensions: 8}, store)
	_, err := project.IndexAndSearch(context.Background(), []DocInput{
		{ID: "rag", Text: "rag retrieves context"},
	}, "", 1)
	if !errors.Is(err, ErrEmptyText) {
		t.Fatalf("IndexAndSearch() error = %v, want ErrEmptyText", err)
	}
	if len(store.docs) != 1 {
		t.Fatalf("stored documents = %d, want 1", len(store.docs))
	}
}
