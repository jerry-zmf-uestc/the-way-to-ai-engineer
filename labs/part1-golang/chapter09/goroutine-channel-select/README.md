# 示例： goroutine channel select

## Goal

Learn how a goroutine starts concurrent work, how a channel carries results, and how `select` waits for the first useful event or a timeout.

## Concepts

- `go` starts work in another goroutine.
- Channels transfer typed values between goroutines.
- Buffered channels let a short worker send one result without blocking forever.
- `select` chooses the first ready channel case.
- `sync.WaitGroup` helps close a fan-in channel after all inputs finish.

## Run

```bash
go -C labs test ./part1-golang/chapter09/goroutine-channel-select -race -count=1
go -C labs run ./part1-golang/chapter09/goroutine-channel-select
```

Expected output:

```text
cache: cached profile
```

## Code Map

- [main.go](main.go): shows the smallest end-to-end path from a goroutine to a
  channel send to a `select` that chooses the first useful result.
- [main_test.go](main_test.go): keeps the timeout and fan-in behavior under
  test, which is where concurrent examples usually become flaky.

## Project Use

This pattern appears when a program races a fast cache against a slower source, fans in results from multiple workers, or needs a timeout so one slow operation does not block the whole request path.

## Related Exercises

- [Chapter 9 · Concurrency Exercises](../../../../content/book/part1-golang/chapter09.md#concurrency-exercises): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Stay in [chapter09/README.md](../../../../content/book/part1-golang/chapter09.md) to compare this one-worker example with
[worker-pools](../worker-pools), where the same channel ideas are extended to
bounded parallelism.
