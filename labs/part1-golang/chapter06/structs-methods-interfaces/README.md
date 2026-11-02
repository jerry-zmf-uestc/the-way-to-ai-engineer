# 示例： structs methods interfaces

## Goal

Learn how structs, methods, and interfaces fit together in small Go programs.

## Concepts

- struct literals and field access
- pointer receivers for mutation
- value receivers for read-only summaries
- implicit interface implementation
- nil interface vs interface holding a nil concrete pointer

## Run

```bash
go -C labs run ./part1-golang/chapter06/structs-methods-interfaces
go -C labs test ./part1-golang/chapter06/structs-methods-interfaces
```

## Code Map

- [main.go](main.go): introduces the struct, methods, and interface examples in
  one file so the relationships stay visible.
- [main_test.go](main_test.go): covers receiver behavior and interface matching,
  including the nil-interface edge case.

## Project Use

Structs model state, methods keep behavior close to that state, and interfaces describe the behavior a caller needs.

## Related Exercises

- [Chapter 6 Exercises](../../../../content/book/part1-golang/chapter06.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [chapter6/README.md](../../../../content/book/part1-golang/chapter06.md) for the chapter map, then continue with
[reflection-basics](../../chapter07/reflection-basics) to see how types can be
inspected and generalized.
