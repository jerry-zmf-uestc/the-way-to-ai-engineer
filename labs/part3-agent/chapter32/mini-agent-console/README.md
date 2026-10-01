# Mini Agent Console

A minimal agent product that starts from a clean checkout with one command.
The web page sends a question, a deterministic offline fake model optionally
proposes one tool call (`calculator` or `echo`), and the answer streams back
token by token over Server-Sent Events. A stop button cancels the stream.

Non-goals: no real LLM provider, no multi-turn memory, no npm or build step,
no writable or irreversible tools. The tool registry is fixed by the
application, not by the prompt.

## Prerequisites

- Go 1.26 (see `labs/go.mod`); no other services or dependencies.
- Chapter 26 (agent basics) and chapter 28 (tool/RAG boundaries) give the
  agent-loop background; this lab is the streaming chapter's capstone and is
  fully self-contained — it imports nothing from other labs.

## Key mechanics and observation points

- `GET /chat?q=...` streams SSE events: `token`, `tool_call`, `tool_result`,
  `agent_error`, `done` (with `finish_reason` = `stop` | `error` | `timeout` |
  `cancelled`). EventSource only speaks GET, hence the query parameter.
- The stop button calls `EventSource.close()`, the request context is
  cancelled, and the agent goroutine exits — no goroutine leak.
- A server-side timeout (default 30s) ends long sessions with
  `finish_reason=timeout`; a token budget caps the reply length.
- Unknown tools are rejected by the registry before execution: try
  `tool rocket launch`. Tool arguments are validated before running: try
  `calc 2+` or `echo`.

Try these prompts:

- `calc 2+3*4` — one calculator call, then a streamed summary.
- `echo hello stream` — one echo call.
- `tool rocket launch` — the fake model proposes an unregistered tool and the
  registry refuses to run it.
- anything else — a direct streamed answer with no tool call.

## Run

From a clean checkout, one command:

```bash
go -C labs run ./part3-agent/chapter32/mini-agent-console
```

Then open http://127.0.0.1:8080 and ask `calc 2+3*4`. Expected terminal
output:

```text
mini agent console listening on http://127.0.0.1:8080 (Ctrl+C to stop)
```

Without a browser, watch the raw stream:

```bash
curl -N 'http://127.0.0.1:8080/chat?q=calc%202%2B3*4'
```

Expected stream (abridged):

```text
event: tool_call
data: {"args":"2+3*4","name":"calculator"}

event: tool_result
data: {"name":"calculator","result":"14"}

event: token
data: {"token":"The "}
...

event: done
data: {"finish_reason":"stop"}
```

## Test

```bash
gofmt -l labs/part3-agent/chapter32/mini-agent-console
go -C labs vet ./part3-agent/chapter32/mini-agent-console
go -C labs test ./part3-agent/chapter32/mini-agent-console -race -count=1
```

The httptest suite never listens on a real port: it covers a full streamed
session, client cancel (handler returns and the agent goroutine exits),
unknown-tool rejection, the server-side timeout path, empty-question
validation, and the calculator grammar.

## Screenshot

Placeholder: a page capture belongs at
`docs/screenshots/mini-agent-console.png` (question `calc 2+3*4` with the
`tool_call` / `tool_result` / token stream / `done` lines visible). The
screenshot is intentionally not generated or committed here.

## Failure and variant exercises

- Set the server timeout to 1 second, keep the 60 ms token delay, and ask a
  direct question: observe the stream end with `finish_reason=timeout`.
- Remove the registry check in `runAgent` and rerun
  `TestChatUnknownToolRejected`: watch it fail, proving the test guards the
  rejection path.
- Variant: add a third tool (e.g. `upper`) to `defaultRegistry`, extend
  `decide` to propose it, and cover it with a streaming test.

## Code Map

- [main.go](main.go): fake model decision rules, the tool registry
  (`calculator`, `echo`), the agent loop on a cancellable event channel, the
  SSE handler with timeout, and a small arithmetic parser.
- [main_test.go](main_test.go): httptest coverage of streaming, cancel,
  unknown-tool rejection, timeout, validation, and the calculator.
- [web/index.html](web/index.html): single-page console, native EventSource
  rendering, stop button; no npm, no build step, embedded via `go:embed`.

## Sources and license

Original code for this course, MIT license. No third-party material.
