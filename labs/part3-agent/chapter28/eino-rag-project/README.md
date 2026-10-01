# 示例： eino rag project

## Goal

Use a single runnable project to connect query embedding, retrieval, context
assembly, and grounded answer generation.

## Concepts

- `Project` is the application boundary.
- `Answer` coordinates retrieval and answer generation.
- `ProjectAnswer.Render` gives the stable golden output shape the tests rely on.

## Run

```bash
gofmt -l labs/part3-agent/chapter28/eino-rag-project
go -C labs vet ./part3-agent/chapter28/eino-rag-project
go -C labs test ./part3-agent/chapter28/eino-rag-project -race -count=1
go -C labs test ./part3-agent/chapter28/eino-rag-project -cover
go build -o /tmp/eino-rag-project ./labs/part3-agent/chapter28/eino-rag-project
go -C labs run ./part3-agent/chapter28/eino-rag-project
```

Expected output:

```text
Question: How does rag use context?
Retrieved context:
1. [labs/part3-agent/chapter28/eino-rag-project] A RAG project assembles retrieved context before generating a grounded answer.
Answer: Use the retrieved context above as the grounding material.
```

## Code Map

- [main.go](main.go): defines the project boundary, local knowledge base,
  retrieval flow, and answer rendering.
- [main_test.go](main_test.go): verifies retrieval presence, grounded output,
  and stable project-shaped demo rendering.

## Related Exercises

- [Chapter 28 Exercises](../../../../book/part3-agent/chapter28/rag.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

This is the capstone for the Eino learning line. After reading this topic, go
back to [chapter26](../../../../book/part3-agent/chapter26.md) and [chapter28](../../../../book/part3-agent/chapter28.md) if you
want to inspect the underlying framework and retrieval layers separately.
