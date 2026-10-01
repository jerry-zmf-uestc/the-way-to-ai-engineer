# 示例： Code Review Comments

This topic turns selected Go Code Review Comments into a small review checklist example for Day 16.

## Goal

- Pass `context.Context` as the first parameter when work can be canceled.
- Return explicit errors instead of in-band values.
- Keep error strings lowercase and without punctuation.
- Spell initialisms consistently, such as `ChangeID`.
- Let a nil slice represent no missing checks.
- Write test failures with input, actual value, and expected value.

## Run

```bash
gofmt -l labs/part1-golang/chapter11/code-review-comments
go -C labs vet ./part1-golang/chapter11/code-review-comments
go -C labs test ./part1-golang/chapter11/code-review-comments -race -count=1
go -C labs test ./part1-golang/chapter11/code-review-comments -cover
go build -o /tmp/code-review-comments ./labs/part1-golang/chapter11/code-review-comments
go -C labs run ./part1-golang/chapter11/code-review-comments
```

## Code Map

- [main.go](main.go): bundles several review-comment examples into one compact
  program.
- [main_test.go](main_test.go): demonstrates the style of assertion messages the
  checklist recommends.

## Project Use

The example focuses on review comments that can be shown in a compact program. It does not try to reproduce the full Go wiki page.

## Related Exercises

- [Chapter 11 · Engineering Exercises](../../../../book/part1-golang/chapter11.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [Chapter 11 · Engineering](../../../../book/part1-golang/chapter11.md) for the engineering-topic index, then
continue with [reflection-basics](../../chapter07/reflection-basics) to shift
from review guidance into runtime type mechanics.
