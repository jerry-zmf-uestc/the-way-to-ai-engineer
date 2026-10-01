# 示例：hello-llm

## Goal

Understand the minimal LLM call chain in Go: the message model (system /
user / assistant roles), a `Provider` interface with a deterministic offline
fake, an OpenAI-compatible HTTP adapter tested against `httptest`, and
token/cost bookkeeping. Real provider calls stay behind the explicit
`E2E_LIVE` gate and never run in default tests.

Non-goals: streaming, tool calling, and retry policies — those belong to
later chapters.

## Prerequisites

- Book chapter: `book/part3-agent/chapter22.md`.
- Go toolchain per `labs/go.mod`; all labs share that single module.

## Concepts

- `Message` + `Role`: system carries stable rules, user carries the current
  request, assistant carries prior model output.
- `Provider` interface: callers depend on `Generate(ctx, []Message)`, so the
  fake and the HTTP adapter are interchangeable without caller changes.
- `FakeProvider`: deterministic rule-based replies with estimated token
  usage, so tests assert exact output offline.
- `OpenAICompatibleProvider`: POSTs OpenAI-style `/chat/completions` with a
  timeout, a response size cap, and `HTTPError` so callers can branch on the
  status code.
- `estimateCost`: hardcoded example prices, clearly marked as not a real
  bill.

## Run

```bash
gofmt -l labs/part3-agent/chapter22/hello-llm
go -C labs vet ./part3-agent/chapter22/hello-llm
go -C labs test ./part3-agent/chapter22/hello-llm -race -count=1
go -C labs test ./part3-agent/chapter22/hello-llm -cover
go build -o /tmp/hello-llm ./labs/part3-agent/chapter22/hello-llm
go -C labs run ./part3-agent/chapter22/hello-llm
```

Expected output:

```text
reply: pong
tokens: prompt=10 completion=2
estimated cost: $0.00000270 (example price, not a real bill)
```

`TestE2ELiveProvider` skips by default. To run it explicitly (real cost,
never in default CI):

```bash
E2E_LIVE=1 LLM_BASE_URL=https://your-provider.example.com LLM_API_KEY=... LLM_MODEL=... \
  go -C labs test ./part3-agent/chapter22/hello-llm -run TestE2ELiveProvider -count=1
```

## Code Map

- [main.go](main.go): message model, `Provider` interface, `FakeProvider`,
  `OpenAICompatibleProvider`, cost estimation, and the demo `main`.
- [main_test.go](main_test.go): fake determinism, httptest-backed success /
  HTTP 500 / timeout paths, and the E2E gate that skips by default.

## Variation Exercises

- Add a fake rule (e.g. messages containing "weather" get a fixed reply) and
  extend `TestFakeProviderDeterministic` to assert it.
- Add a 429 test case to the httptest server and assert `HTTPError.StatusCode`
  lets the caller distinguish rate limiting from a 500.

## Related Exercises

- [Chapter 22 Exercises](../../../../book/part3-agent/chapter22.md#动手练): the
  chapter page lists the executable practice tasks with acceptance checks.

## Continue

- [structured-output](../structured-output): make the provider return JSON,
  validate it against a schema, and retry with feedback on visible failures.

## Sources

Original code for this course, MIT licensed. Request/response shape follows
the OpenAI chat completions API conventions.
