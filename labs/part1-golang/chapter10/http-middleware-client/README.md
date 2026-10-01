# 示例： HTTP Middleware & Retryable Client

## Goal

Add the two missing layers around a basic HTTP handler:

- server-side middleware for logging, panic recovery, and timeout
- client-side retry with backoff, `Retry-After`, and context cancellation

This topic is the first place where the repository shows HTTP as a full
request/response boundary instead of a single handler function.

## Core Model

| Layer | In code | Responsibility |
| --- | --- | --- |
| Middleware chain | `Middleware`, `Chain` | Compose handler wrappers in one shape |
| Server concerns | `Logging`, `Recover`, `Timeout` | Add observability and failure control |
| Retryable client | `Client.DoWithRetry` | Retry only when policy allows |
| Retry policy | `RetryPolicy`, `DefaultIsRetryable` | Decide which method/status/error can retry |

The important split is that middleware handles server-side cross-cutting
behavior, while `DoWithRetry` handles outbound client behavior.

## Run

```bash
gofmt -l labs/part1-golang/chapter10/http-middleware-client
go -C labs vet ./part1-golang/chapter10/http-middleware-client
go -C labs test ./part1-golang/chapter10/http-middleware-client -race -count=1
go -C labs test ./part1-golang/chapter10/http-middleware-client -cover -count=1
go build -o /tmp/http-middleware-client ./labs/part1-golang/chapter10/http-middleware-client
go -C labs run ./part1-golang/chapter10/http-middleware-client
```

Expected output:

```text
demo=echo         status=200 body={"echo":"ok"}   attempts=1
demo=recover      status=500 body=internal error  attempts=1
demo=timeout      status=503 body=timeout         attempts=1
demo=retry-then-ok status=200 body={"echo":"ok"}  attempts=3
```

## Code Map

- [main.go](main.go): bounded demo covering echo, panic, timeout, and retry
- [middleware.go](middleware.go): middleware shape plus `Logging`, `Recover`,
  `Timeout`, and `statusRecorder`
- [client.go](client.go): retry loop, retry policy, and `Retry-After` parsing
- [middleware_test.go](middleware_test.go): middleware behavior tests
- [client_test.go](client_test.go): retry, transport error, and cancellation tests
- [main_test.go](main_test.go): locks demo output
- API reference:
  [http.Client](https://pkg.go.dev/net/http#Client)

## Common Pitfalls

1. Leaving outbound calls on `http.DefaultClient` with no timeout.
2. Retrying non-idempotent methods by default.
3. Sleeping during backoff without respecting `ctx.Done()`.
4. Forgetting to drain and close response bodies before retrying.
5. Treating middleware and retry as the same concern when they sit on opposite sides of the HTTP boundary.

## Related Exercises

- [Chapter 10 · HTTP Exercises](../../../../book/part1-golang/chapter10.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

After the HTTP boundary is stable, move to [repository-pattern](../../../part2-backend/chapter12/repository-pattern)
and [gorm-mysql](../../../part2-backend/chapter12/gorm-mysql) to see how service code talks to a data layer.
