# 示例： functions errors

## Goal

Learn functions, multiple return values, explicit errors, and error wrapping.

## Concepts

- function parameters and returns
- multiple return values
- `error`
- `fmt.Errorf` with `%w`
- sentinel errors with `errors.Is`

## Run

```bash
go -C labs run ./part1-golang/chapter05/functions-errors
go -C labs test ./part1-golang/chapter05/functions-errors
```

## Code Map

- [main.go](main.go): pairs small parsing helpers with wrapped and sentinel
  errors.
- [main_test.go](main_test.go): verifies both the returned values and the
  `errors.Is` behavior expected by callers.

## Project Use

Explicit error handling is a core Go habit used in parsing, IO, services, and CLI tools.

The example keeps parse failures wrapped with input context while using sentinel errors for expected empty-input cases.

## Related Exercises

- [Chapter 5 Exercises](../../../../book/part1-golang/chapter05.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [chapter5/README.md](../../../../book/part1-golang/chapter05.md) for the chapter index, then continue with
[structs-methods-interfaces](../../chapter06/structs-methods-interfaces) to see
how functions and errors attach to stateful types.
