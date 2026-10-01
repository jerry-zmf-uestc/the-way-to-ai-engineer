# 示例： Kitex IDL and Handler — Contract Design & Evolution

This topic is about **designing a Thrift contract and evolving it safely**. It
assumes you already know how to write a Kitex handler (see
[../kitex-rpc/](../kitex-rpc) for the "first Kitex service" basics) and instead
zooms in on the questions that bite real services:

- How does a Thrift field qualifier (`required` vs `optional`) change the
  **generated Go type**, and why does that matter?
- What changes to an IDL are **backward compatible**, and which ones break
  callers?
- How do you keep a handler from silently **drifting** away from the contract?

> **How this differs from `kitex-rpc`:** `kitex-rpc` is the Hello-World — write
> an IDL, generate, implement a couple of methods. This topic keeps the handler
> deliberately thin and spends its attention on the **contract side**: pointer
> semantics of optional fields, compatible evolution, field-number discipline,
> and the compile-time contract check.

## 1. `required` vs `optional` → what codegen emits

The single most important thing to internalize: a field's Thrift qualifier
decides its **Go type** after `kitex gen`.

| IDL declaration | Generated Go field | Meaning of the zero state |
| --- | --- | --- |
| `1: required string sku` | `Sku string` | always present on the wire |
| `3: optional string currency` | `Currency *string` | `nil` = **caller omitted it** (distinct from `""`) |
| `4: optional i64 discount_bps` | `DiscountBps *int64` | `nil` = not requested (distinct from an explicit `0`) |

Codegen also emits an `IsSetCurrency()` helper and a `GetCurrency()` getter.
**The trap:** `GetCurrency()` returns the zero value when the pointer is `nil`,
so it *hides* the "omitted vs empty" distinction. When that distinction matters,
check `IsSetCurrency()` first — see [handler.go](handler.go):

```go
currency := defaultCurrency
if req.IsSetCurrency() {
    currency = strings.ToUpper(strings.TrimSpace(req.GetCurrency()))
}
```

This is exactly where "an old client that never sets Currency keeps working" is
implemented: its request arrives with `Currency == nil`, and the handler falls
back to the default.

## 2. Which IDL changes are backward compatible

[idl/order.thrift](idl/order.thrift) grew from v1 to v2 using only compatible
changes:

| Change | Compatible? | Why |
| --- | --- | --- |
| **Add an `optional` field** (`currency`, `discount_bps`) | ✅ | old peers don't send it → arrives `nil`; old peers ignore unknown fields |
| **Add a new method** (`Ping`) | ✅ | callers that only know `Quote` keep compiling and calling it unchanged |
| Add a `required` field | ⚠️ risky | old clients never send it → decode expectations break |
| Remove / rename a field or method | ❌ | peers on the old contract break |
| **Reuse a field number** for a different field | ❌❌ | the number is the field's wire identity; reuse silently corrupts data |

**Field-number discipline:** numbers (`1:`, `2:`, …) are assigned once and never
reused. To drop a field you retire its number, you never hand it to a new field.
That rule is the whole reason old and new peers can interoperate.

## 3. What a breaking change actually does (proven, not asserted)

It is easy to *say* "reusing a field number corrupts data." The counter-example
in [evolution_break_test.go](evolution_break_test.go) *proves* it with a real
Thrift Binary serialization round-trip (via `frugal`, the tag-driven codec Kitex
already uses) — an old peer encodes bytes, a peer on a changed schema decodes
them, and we assert on the damage. No server or port required.

The decoder matches bytes to fields **by number, never by name**. Two breaking
cases, both of which compile fine and (this is the scary part) raise no error:

| Scenario | Producer wrote | Consumer schema | Result |
| --- | --- | --- | --- |
| **Renumber** — field number reused by a new field | `quantity=42` at number `2` | number `2` now a new `string`; old field moved to `3` | `quantity` **silently vanishes** (arrives `0`); only the untouched number `1` survives |
| **Swap** — two same-typed fields trade numbers | `warehouse=100` (1), `shipTo=200` (2) | numbers `1`/`2` swapped | values **silently land in the wrong field** — no error, no loss, just wrong |

The swap case is the most dangerous: same type, so nothing mismatches, nothing
errors, nothing is lost — the data is just quietly wrong, the kind of bug that
passes review and ships. Contrast with the compatible case in the same test
file: appending an `optional` field at a fresh number leaves old bytes fully
decodable (`sku`/`quantity` intact, new field `nil`).

```bash
go -C labs test ./part2-backend/chapter17/kitex-idl-handler -run TestBreakingChange -v
```

This is the mechanical reason field numbers are immutable: the wire format has
no memory of names, only numbers.

## 4. The compile-time contract check

The IDL owns the method set; the handler must satisfy every method codegen
declares. [main_test.go](main_test.go) locks this in with one line:

```go
var _ order.OrderService = NewOrderServiceHandler(nil)
```

Add `Ping` to the IDL and regenerate, but forget to implement it, and this line
**stops compiling**. That is the contract refusing to drift — the mechanical
enforcement of "IDL is the single source of truth".

## 4.1 Code Map

- [idl/order.thrift](idl/order.thrift): the contract under discussion, including
  the optional fields and the added `Ping` method.
- [handler.go](handler.go): shows how `IsSetCurrency()` and fallback defaults
  preserve backward compatibility in real handler logic.
- [main.go](main.go): keeps the demo bounded while exercising v1/v2 request
  shapes against the same handler.
- [main_test.go](main_test.go): contains the compile-time contract assertion.
- [evolution_break_test.go](evolution_break_test.go): proves that field-number
  reuse or swapping silently damages decoding.

## 5. The three layers (recap)

1. **IDL (you write this)** — [idl/order.thrift](idl/order.thrift) is the single
   source of truth: services, methods, fields, and their qualifiers.
2. **Generated code (the tool writes this)** — `kitex gen` turns the IDL into
   `kitex_gen/order/` (structs + getters + `IsSet*` + the `order.OrderService`
   interface + client/server plumbing). Files carry `DO NOT EDIT`; never touch
   them by hand.
3. **Handler (you write this)** — [handler.go](handler.go) implements
   `order.OrderService` and holds the business logic, checking `ctx.Err()`
   before doing work so cancellations are preserved.

## 6. Code generation (replay the middle layer)

Install the tools once:

```bash
go install github.com/cloudwego/thriftgo@v0.4.5
go install github.com/cloudwego/kitex/tool/cmd/kitex@v0.16.2
export PATH="$PATH:$(go env GOPATH)/bin"
```

Regenerate from this directory whenever [idl/order.thrift](idl/order.thrift)
changes:

```bash
cd labs/part2-backend/chapter17/kitex-idl-handler
kitex -module github.com/jerry-zmf-uestc/the-way-to-ai-engineer -no-fast-api idl/order.thrift
```

The generated files are checked in, so a plain `go test` does not require the
tools. **Change the IDL and regenerate — never edit `kitex_gen/` by hand.**

## 7. Teaching layout vs. production practice

This topic keeps `idl/order.thrift` and `kitex_gen/` inside the example
directory so the whole chain is readable in one place. Real Go backends
(including large-scale setups) usually manage IDL differently. The contract is
still hand-authored, but *where* it lives and *how* it is generated change:

| Aspect | This example (teaching) | Typical production |
| --- | --- | --- |
| Where the IDL lives | next to the code in `idl/` | a **dedicated IDL repo/registry** shared across teams |
| Who writes it | you, locally | the service owner, but via **MR + review**, like reviewing a public API |
| How code is generated | you run `kitex gen` locally | a platform/CI generates and publishes it, often as a **versioned dependency** |
| Are generated files committed | yes, checked into this repo | frequently **not** in the business repo — pulled as a versioned package instead |
| Versioning | none | the IDL is **versioned**; downstreams pin a version for compatibility |

What stays true everywhere:

- **The IDL is hand-authored** — writing it is *contract design*, not
  boilerplate. Field numbers, `required` vs. `optional`, and method shape are
  compatibility decisions a human must own.
- **Generated code is never hand-written.** Whether it is committed here or
  published as a package elsewhere, you always change the IDL and regenerate.
- IDL is often *seeded* (copied from a sibling service, scaffolded, converted
  from OpenAPI, or AI-drafted) but the field numbers and compatibility must
  still be reviewed by a person, because the cost of a wrong edit (a live
  incompatibility) is high.

So the mental model — **hand-write the contract, generate everything else** —
matches production. The difference is that production centralizes, versions, and
platform-generates the IDL rather than scattering it across topic folders like
this learning repo does.

## 8. Run

```bash
gofmt -l labs/part2-backend/chapter17/kitex-idl-handler
go -C labs vet ./part2-backend/chapter17/kitex-idl-handler
go -C labs test ./part2-backend/chapter17/kitex-idl-handler -race -count=1
go -C labs test ./part2-backend/chapter17/kitex-idl-handler -cover
go build -o /tmp/kitex-idl-handler ./labs/part2-backend/chapter17/kitex-idl-handler
go -C labs run ./part2-backend/chapter17/kitex-idl-handler
```

Expected output:

```text
v1(no-opt) -> quote KITEX x3 = $89.97 USD | v2(currency) -> quote KITEX x3 = $89.97 EUR | v2(discount) -> 8548 cents
```

The three segments contrast contract evolution in one line: a v1-style caller
that omits the optional fields (falls back to `USD`), a v2 caller that sets
`Currency`, and a v2 caller that sets `DiscountBps` (500 bps = 5% off 8997 =
8548 cents).

## 9. Key Points

- A Thrift field qualifier decides the generated Go type: `optional` → pointer,
  so `nil` (omitted) is distinguishable from the zero value.
- Compatible evolution = add optional fields / add methods; breaking = remove /
  rename / reuse a field number / add a required field.
- Breaking changes are *proven* with a real serialization round-trip in
  [evolution_break_test.go](evolution_break_test.go): renumbering silently drops
  a value, swapping numbers silently misroutes it — no error either way, because
  the wire format keys on numbers, not names.
- The compile-time `var _ order.OrderService = handler` assertion is the
  mechanical guarantee that the handler can't drift from the contract.
- The handler still checks `ctx.Err()` first and returns sentinel errors that
  tests assert with `errors.Is`.
- `go run` stays bounded in daily automation; long-lived Kitex server wiring
  (`server.Serve()`) belongs in later project tasks such as
  [../../chapter15/hertz-kitex-real-chain/](../../chapter16/hertz-kitex-real-chain).

## Related Exercises

- [Chapter 17 Exercises](../../../../book/part2-backend/chapter17.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

- Return to [chapter17/README.md](../../../../book/part2-backend/chapter17.md) for the chapter index and reading order.
- Read [kitex-rpc](../kitex-rpc) first if you want the smaller “first service”
  version before the contract-evolution discussion.
- Continue to [transactions](../../chapter13/transactions) for the next
  boundary problem: once a contract is stable, how do multiple writes commit or
  roll back together?
