# 示例： tool calling

## Goal

Build the execution half of tool calling: a tool registry that owns tool
permissions, plus a schema check that runs before any handler. Non-goals: no
real LLM provider, no network access, and no full agent loop (that lives in
[minimal-agent-loop](../minimal-agent-loop)).

## Prerequisites

- Chapter 4 (functions, closures) and Chapter 5 (error handling) in Part 1.
- Chapter 9 (`context`) for the handler signature.

## Concepts

- The model only *proposes* a tool call; the registry decides whether it runs.
- `Registry.Call` is the single enforcement point: unknown tools are rejected
  with `ErrUnknownTool`, arguments failing the schema return
  `ErrInvalidArguments`, and in both cases the handler never runs.
- Schema validation covers missing required arguments, wrong types, and
  undeclared arguments; error messages name the tool, the argument, and the
  expected versus actual type.
- The two demo tools (`calculator`, `get_time`) are deterministic fakes:
  `get_time` takes an injected clock so tests never depend on wall time.

## Run

```bash
gofmt -l labs/part3-agent/chapter23/tool-calling
go -C labs vet ./part3-agent/chapter23/tool-calling
go -C labs test ./part3-agent/chapter23/tool-calling -race -count=1
go -C labs run ./part3-agent/chapter23/tool-calling
```

Expected output:

```text
registered=[calculator get_time] calculator=42 get_time=2026-01-01T08:00:00Z rejected="tool calling: unknown tool: \"delete_database\""
```

## Code Map

- [main.go](main.go): the schema subset, the registry with its single `Call`
  entry point, argument validation, and two deterministic demo tools.
- [main_test.go](main_test.go): covers successful calls, unknown-tool
  rejection, invalid arguments never reaching the handler, readable validation
  messages, handler-level semantic errors (division by zero, unknown op), and
  duplicate registration.

## Exercises

- Variant: register a third tool `string_length` that takes one required
  string argument; assert that a numeric argument is rejected and the error
  message reports `expected string`.
- Failure path: call `calculator` with `{"op": "divide", "a": 1, "b": 0}` and
  confirm the error comes from the handler, not from schema validation — the
  two failure layers stay distinguishable.

## Sources & License

Original code for this course, MIT licensed. No third-party material.
