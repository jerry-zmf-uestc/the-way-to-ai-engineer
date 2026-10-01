# 示例： Effective Go

This topic starts a small convention-focused example for Day 15.

## Goal

- Keep names short and specific near their use.
- Prefer useful zero values over extra constructors.
- Return errors early and wrap parsing failures with context.
- Cover behavior with table-driven tests.

## Run

```bash
gofmt -l labs/part1-golang/chapter11/effective-go
go -C labs vet ./part1-golang/chapter11/effective-go
go -C labs test ./part1-golang/chapter11/effective-go -race -count=1
go -C labs test ./part1-golang/chapter11/effective-go -cover
go build -o /tmp/effective-go ./labs/part1-golang/chapter11/effective-go
go -C labs run ./part1-golang/chapter11/effective-go
```

## Code Map

- [main.go](main.go): keeps the naming, zero-value, and early-return examples in
  one compact program.
- [main_test.go](main_test.go): shows the table-driven test shape that the topic
  recommends.

## Project Use

The example is intentionally small. It is meant to make Effective Go rules visible in code before larger convention rewrites start later this week.

## Related Exercises

- [Chapter 11 · Engineering Exercises](../../../../book/part1-golang/chapter11.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [Chapter 11 · Engineering](../../../../book/part1-golang/chapter11.md) for the engineering-topic index, then
continue with [code-review-comments](../code-review-comments) or
[reflection-basics](../../chapter07/reflection-basics) depending on whether you
want one more style guide topic or the next runtime abstraction topic.
