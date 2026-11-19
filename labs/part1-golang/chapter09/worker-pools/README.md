# 示例： worker pools

## Goal

Learn how a fixed set of goroutines can share work from a channel, wait for all workers to finish with `sync.WaitGroup`, and protect shared counters with `sync.Mutex`.

## Concepts

- A jobs channel distributes tasks to workers.
- Each worker loops until the jobs channel is closed.
- `sync.WaitGroup` lets the main goroutine wait for every worker to exit before closing the reports channel.
- `sync.Mutex` protects shared stats when multiple workers update the same counters.
- Results are sorted before returning so tests can assert stable output even though work runs concurrently.

## Run

```bash
gofmt -l labs/part1-golang/chapter09/worker-pools
go -C labs vet ./part1-golang/chapter09/worker-pools
go -C labs test ./part1-golang/chapter09/worker-pools -race -count=1
go -C labs test ./part1-golang/chapter09/worker-pools -cover
go build -o /tmp/worker-pools ./labs/part1-golang/chapter09/worker-pools
go -C labs run ./part1-golang/chapter09/worker-pools
```

Expected output:

```text
job 1 handled by worker 1 scored 4
job 2 handled by worker 2 scored 16
job 3 handled by worker 1 scored 9
completed 3 jobs, total quality 29
```

Worker IDs may vary because goroutine scheduling is nondeterministic.

## Code Map

- [main.go](main.go): builds the worker pool, shared stats, and deterministic
  result sorting in one place.
- [main_test.go](main_test.go): focuses on the bounded-concurrency guarantees and
  the stable result shape exposed to callers.

## Project Use

Worker pools are useful when a service needs bounded concurrency: processing queued jobs, fetching many URLs with a limit, or running CPU-heavy work without starting an unbounded goroutine for every request.

## Related Exercises

- [Chapter 9 · Concurrency Exercises](../../../../content/book/part1-golang/chapter09.md#concurrency-exercises): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [chapter09/README.md](../../../../content/book/part1-golang/chapter09.md) for the full concurrency chapter, then move
to [http-server](../../chapter10/http-server) to see the same goroutine and
timeout ideas show up in network services.
