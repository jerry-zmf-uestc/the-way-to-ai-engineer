# 示例： eino vector store

## Goal

Understand how an Eino embedder and a vector store cooperate to build the
retrieval half of a RAG system.

## Concepts

- `HashEmbedder` demonstrates the Eino embedding interface without remote APIs.
- `MemoryStore` owns dimension checks and result ordering.
- `VectorProject` is the application boundary that coordinates indexing and
  search.

## Run

```bash
gofmt -l labs/part3-agent/chapter28/eino-vector-store
go -C labs vet ./part3-agent/chapter28/eino-vector-store
go -C labs test ./part3-agent/chapter28/eino-vector-store -race -count=1
go -C labs test ./part3-agent/chapter28/eino-vector-store -cover
go build -o /tmp/eino-vector-store ./labs/part3-agent/chapter28/eino-vector-store
go -C labs run ./part3-agent/chapter28/eino-vector-store
```

Expected output:

```text
indexed=3 query="relevant rag context" top_hit=rag score=0.82
```

## Code Map

- [main.go](main.go): defines a deterministic Eino-shaped embedder, the in-memory
  vector store, and the project boundary that coordinates indexing and search.
- [main_test.go](main_test.go): verifies embedding shape, ranking behavior,
  empty-result versus invalid-request semantics, partial state after a query
  embedding failure, and the bounded demo summary.

## Related Exercises

- [Chapter 28 Exercises](../../../../book/part3-agent/chapter28.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

This chapter stops at retrieval infrastructure. The next chapter turns these
boundaries into a project-shaped RAG demo:
[eino-rag-project](../eino-rag-project).
