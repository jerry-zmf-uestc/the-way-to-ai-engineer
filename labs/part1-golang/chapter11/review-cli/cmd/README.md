# 示例： review cli

`review-cli` is the Day 13 SDD practice command. It accepts repeated `-item topic=score` flags and prints a stable one-line evening review summary.

## Goal

Keep CLI process wiring in `cmd/` while moving parsing and aggregation into an
internal package that is easier to test and evolve.

## Run

```bash
go -C labs run ./part1-golang/chapter11/review-cli/cmd -item gofmt=5 -item errors=4
```

Expected output:

```text
items=2 average=4.50 status=on-track weakest=errors
```

## Input Rules

- `topic` is trimmed UTF-8 text with length 1-64 runes.
- `score` must be an integer from 0 through 5.
- Validation errors are written to stderr and return exit code 2.

## SDD Notes

The command keeps process wiring in `labs/part1-golang/chapter11/review-cli/cmd` and puts parsing plus aggregation in `labs/part1-golang/chapter11/review-cli/internal/review`. That split keeps the CLI small while making the learning logic easy to test with table-driven tests.

## Code Map

- [main.go](main.go): CLI entry that parses repeated `-item` flags and renders
  one stable summary line.
- [main_test.go](main_test.go): command-level checks for stdout, stderr, and exit
  behavior.
- [../internal/review/review.go](../internal/review/review.go): parsing and
  aggregation logic extracted from the command.
- [../internal/review/review_test.go](../internal/review/review_test.go): table-
  driven tests for the private review package.

## 相关练习

- [Chapter 11 · Engineering Exercises](../../../../../book/part1-golang/chapter11.md#练习)：回到正文里的练习区，按伪代码和验收标准继续
  扩展这个 topic。

## Continue

Compare this directory with [json-api/cmd](../../json-api/cmd) to see how the
same `cmd + internal` split works for HTTP services and CLIs.
