# 示例： Kitex RPC

This topic introduces Kitex with a small Thrift-first RPC exercise. It keeps the default demo bounded: `go run` calls the generated handler types directly and exits instead of starting a long-lived server.

## Goal

Build the smallest Thrift-first Kitex example: define an IDL, implement the
generated service interface, and keep `go run` bounded by calling the handler
directly instead of starting a long-lived server.

## Core Model

- Thrift IDL as the service contract.
- Kitex-generated Go types under `kitex_gen/`.
- A stateless `EchoServiceImpl` handler that matches the generated service interface.
- Context cancellation and sentinel errors checked with `errors.Is`.
- Direct handler tests that avoid ports, sleeps, and background listeners.

## Run

```bash
go -C labs run ./part2-backend/chapter17/kitex-rpc
```

Expected output:

```text
Echo -> hello kitex (11), Add -> 1024
```

## Test

```bash
go -C labs test ./part2-backend/chapter17/kitex-rpc -race -count=1
go -C labs test ./part2-backend/chapter17/kitex-rpc -cover
```

## Code Map

- [main.go](main.go): keeps the bounded demo in-process by calling the generated
  handler types directly.
- [main_test.go](main_test.go): verifies the handler behavior and contract-level
  error handling without opening a real port.
- [idl/echo.thrift](idl/echo.thrift): the source-of-truth RPC contract.

## Code Generation

Install the Kitex tools when code generation needs to be replayed:

```bash
go install github.com/cloudwego/thriftgo@v0.4.5
go install github.com/cloudwego/kitex/tool/cmd/kitex@v0.16.2
```

Run codegen from this directory:

```bash
cd labs/part2-backend/chapter17/kitex-rpc
kitex -module github.com/jerry-zmf-uestc/the-way-to-ai-engineer idl/echo.thrift
```

The generated files are checked in so normal `go test` runs do not require code generation. Do not manually edit files under `kitex_gen/`; change `idl/echo.thrift` and regenerate instead.

## Related Exercises

- [Chapter 17 Exercises](../../../../book/part2-backend/chapter17.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

- The default demo does not call `server.Serve()` because daily automation must not hang on a listener.
- A later exercise can split real server/client lifecycle wiring into `cmd/` and private logic into `internal/`.
- `Echo` trims whitespace and counts UTF-8 runes, so Unicode input is handled by character count rather than byte length.
- Continue with [kitex-idl-handler](../kitex-idl-handler) to see how the same
  Kitex flow changes when the focus shifts from “first RPC” to contract
  evolution and backward compatibility.
- Return to [chapter17/README.md](../../../../book/part2-backend/chapter17.md) for the chapter reading order.
