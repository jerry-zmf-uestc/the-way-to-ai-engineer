# 示例： reflection basics

## Goal

Learn how Go reflection (`reflect`) inspects and manipulates values at runtime
when the concrete type is unknown at compile time. This is the mechanism behind
`json.Marshal`, Hertz `BindAndValidate`, and ORMs.

## Concepts

Reflection splits a value into two orthogonal views:

| Entry | Question | Returns |
| --- | --- | --- |
| `reflect.TypeOf(x)` | "what type is it" | `reflect.Type` (interface) |
| `reflect.ValueOf(x)` | "what value, can I change it" | `reflect.Value` (struct) |

Both take `any`, because an interface holds a `(type, value)` pair — reflection
just pulls the two halves apart.

### Type vs Kind

- **Type / `Name()`**: the concrete named type (`User`, `MyInt`, `time.Duration`).
- **Kind**: the fixed underlying category (`int`, `string`, `struct`, `slice`, `ptr`...).

Branch logic almost always switches on `Kind()`, because you cannot enumerate
every user-defined type but the underlying kinds are finite.

### The three laws of reflection

1. interface value → reflection object (`TypeOf` / `ValueOf`)
2. reflection object → interface value (`Value.Interface()`)
3. **to modify a reflection object, the value must be settable**

Law 3 is why every mutation API needs a pointer: `ValueOf(x)` copies `x`, so you
must pass `&x` and call `.Elem()` to reach the real memory. Unexported fields are
addressable but **not** settable (`CanSet() == false`) — see `setString`.

## Functions

| Function | Reflection capability |
| --- | --- |
| `describeType` | type introspection (`TypeOf` + `Name` vs `Kind`) |
| `fieldTags` | struct field + tag traversal (`Type.Field(i).Tag`) |
| `setString` | mutate via pointer (laws of reflection + `CanSet`) |
| `callMethod` | dynamic method dispatch (`MethodByName().Call()`) |

All error paths return wrapped sentinel errors, so callers use `errors.Is`
instead of string comparison.

## Code Map

- [main.go](main.go): keeps the four reflection operations in one file so the
  laws of reflection, tag access, mutation, and dynamic dispatch are easy to
  compare side by side.
- [main_test.go](main_test.go): covers the happy path and the sentinel-error
  branches, which is where reflection code most often gets brittle.

## Run

```bash
gofmt -l labs/part1-golang/chapter07/reflection-basics
go -C labs vet ./part1-golang/chapter07/reflection-basics
go -C labs test ./part1-golang/chapter07/reflection-basics -race -count=1
go -C labs test ./part1-golang/chapter07/reflection-basics -cover
go build -o /tmp/reflection-basics ./labs/part1-golang/chapter07/reflection-basics
go -C labs run ./part1-golang/chapter07/reflection-basics
```

Expected output:

```text
float64(float64)
User(struct)
[]int(slice)
json tags: [Name=name Age=age]
after set: name=new age=20
Add(3,4) = 7
```

## Cost & when to avoid

Reflection is roughly an order of magnitude slower than direct calls (dynamic
type lookup, boxing, no inlining) and trades compile-time safety for runtime
panics. Prefer, in order:

1. **Generics** (Go 1.18+) — compile-time safe, zero runtime cost.
2. **Interfaces** — polymorphism without reflection.
3. **Reflection** — only when the type is genuinely unknown until runtime.

## Project Use

This repo already relies on reflection indirectly: `c.BindAndValidate(&req)` and
`json.Marshal` read struct tags via reflection, while Kitex's `kitex_gen`
codegen exists precisely to **avoid** reflection on the hot path (FastCodec).
That contrast — reflection for flexibility at the edge, codegen for speed at the
core — is the key takeaway.

## Related Exercises

- [Chapter 7 · Reflection Exercises](../../../../content/book/part1-golang/chapter07.md#reflection-exercises): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [Chapter 7 · Reflection](../../../../content/book/part1-golang/chapter07.md#reflection) for the chapter index, then continue with
[generics-basics](../generics-basics) to compare reflection's
runtime flexibility with generics' compile-time type safety.
