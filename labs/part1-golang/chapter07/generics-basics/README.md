# 示例： generics basics

## Goal

Learn how Go type parameters let one function or type work with several concrete types while keeping compile-time type safety.

## Concepts

- `any` means a type parameter accepts any type.
- `comparable` is required when values become map keys or need `==`.
- A type set such as `~int | ~int64 | ~float64` limits generic code to types that support numeric operators.
- Generic functions can infer type arguments from ordinary function arguments.
- Generic structs keep the same method shape while preserving the element type.

## Run

```bash
gofmt -l labs/part1-golang/chapter07/generics-basics
go -C labs vet ./part1-golang/chapter07/generics-basics
go -C labs test ./part1-golang/chapter07/generics-basics -race -count=1
go -C labs test ./part1-golang/chapter07/generics-basics -cover
go build -o /tmp/generics-basics ./labs/part1-golang/chapter07/generics-basics
go -C labs run ./part1-golang/chapter07/generics-basics
```

Expected output:

```text
score total: 45
price total: 20.0
name lengths: [2 8]
first score: 10
contains cache: true, unique services: 2
```

## Code Map

- [main.go](main.go): demonstrates generic functions, constraints, and a typed
  generic collection in one bounded program.
- [main_test.go](main_test.go): verifies the examples with concrete instantiations
  so refactors preserve both numeric and collection behavior.

## Project Use

Generics are useful for small reusable helpers, typed collections, and algorithms that should preserve caller types without accepting `interface{}` and runtime type assertions.

## Related Exercises

- [Chapter 7 · Generics Exercises](../../../../book/part1-golang/chapter07.md#generics-exercises): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [Chapter 7 · Generics](../../../../book/part1-golang/chapter07.md#generics) for the chapter reading order, then read
[goroutine-channel-select](../../chapter09/goroutine-channel-select) to move
from type abstraction into concurrency primitives.
