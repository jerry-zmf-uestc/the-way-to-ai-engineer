# 示例： minimal agent loop

## Goal

Run the smallest honest agent loop: a scripted model proposes tool calls, an
executor runs them, observations go back into the message history, and the
model is asked again — until it produces a final answer. Non-goals: no real
LLM provider, no streaming, no parameter schema validation (that lives in
[tool-calling](../tool-calling)).

## Prerequisites

- [tool-calling](../tool-calling) in the same chapter for the registry and
  validation boundary.
- Chapter 9 (`context`, cancellation and timeout) in Part 1.

## Concepts

- The loop is `model → tool → observation → model`; the session's message
  history is the only state, so saving it makes the conversation resumable.
- Tool failures never abort the loop: errors are serialized into tool messages
  (`"error: ..."`) so the model can see the failure and decide what to do.
- Three stop conditions, all observable: the model's final answer,
  `ErrMaxStepsExceeded`, and context timeout/cancellation.
- `ScriptedModel` fixes the model's outputs, keeping tests deterministic.

## Run

```bash
gofmt -l labs/part3-agent/chapter23/minimal-agent-loop
go -C labs vet ./part3-agent/chapter23/minimal-agent-loop
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -race -count=1
go -C labs run ./part3-agent/chapter23/minimal-agent-loop
```

Expected output:

```text
answer="6+7=13；当前时间 2026-01-01T08:00:00Z" steps=2 messages=5
```

## Code Map

- [main.go](main.go): message and tool-call shapes, the `ToolSet` executor,
  the append-only `Session`, `Agent.Run` with max steps and context handling,
  and the scripted model used by demo and tests.
- [main_test.go](main_test.go): covers a normal two-step run with observation
  history, max-steps exhaustion, whole-run timeout, external cancellation,
  tool failures fed back as observations, and session copy semantics.

## Exercises

- Failure path: set `MaxSteps` to 1 while the model's first response is a tool
  call; predict the error, then assert `ErrMaxStepsExceeded` and that the
  executed tool's observation is already in the session history.
- Variant: script two rounds where the model calls the same tool twice;
  assert observations land in call order with matching `ToolCallID`s.

## Sources & License

Original code for this course, MIT licensed. No third-party material.
