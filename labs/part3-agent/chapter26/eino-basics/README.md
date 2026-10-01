# 示例： eino basics

## Goal

Understand how Eino introduces component boundaries in Go: prompt input,
message flow, context propagation, and model invocation.

## Concepts

- `buildPromptMessages` keeps prompt construction separate from runtime wiring.
- `FakeChatModel` satisfies Eino's chat model shape without requiring remote
  credentials.
- `runDemo` is the minimal application boundary the later chapters will extend.

## Run

```bash
gofmt -l labs/part3-agent/chapter26/eino-basics
go -C labs vet ./part3-agent/chapter26/eino-basics
go -C labs test ./part3-agent/chapter26/eino-basics -race -count=1
go -C labs test ./part3-agent/chapter26/eino-basics -cover
go build -o /tmp/eino-basics ./labs/part3-agent/chapter26/eino-basics
go -C labs run ./part3-agent/chapter26/eino-basics
```

Expected output:

```text
Eino basics demo: topic=retrieval pipeline audience=Go reader
```

## Code Map

- [main.go](main.go): defines the prompt message builder, a fake model that
  matches Eino's chat model interface shape, and the bounded demo runner.
- [main_test.go](main_test.go): verifies prompt content, fake model output, and
  runtime validation.

## Related Exercises

- [Chapter 26 Exercises](../../../../book/part3-agent/chapter26.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

The next chapter keeps the Eino boundary but moves from prompt/model basics to
embedding and storage. Continue with
[eino-vector-store](../../chapter28/eino-vector-store) when you want the
retrieval infrastructure layer.
