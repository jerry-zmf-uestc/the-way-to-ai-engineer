# 示例： Service Runtime Capstone

## Goal

Learn how to take an already well-bounded checkout service and make it feel
like a real service project: validated config, timeout propagation, readiness
probes, structured logging, and graceful shutdown.

## Concepts

- `LoadConfig` starts from defaults, overlays environment variables, and rejects
  invalid runtime settings before the process starts.
- `App` centralizes logger, readiness state, HTTP probes, timeout-wrapped
  service calls, and shutdown hooks instead of scattering them across handlers.
- `PlaceOrder` uses `context.WithTimeout` at the runtime boundary so the
  underlying business service sees a single request-scoped context.
- `/healthz` and `/readyz` separate liveness from readiness, which lets the
  process stay alive while traffic is being drained.
- `Shutdown` flips readiness to false first, then runs cleanup hooks within a
  bounded timeout.

## Run

```bash
gofmt -l labs/part2-backend/chapter21/service-runtime
go -C labs vet ./part2-backend/chapter21/service-runtime
go -C labs test ./part2-backend/chapter21/service-runtime -race -count=1
go -C labs test ./part2-backend/chapter21/service-runtime -cover
go build -o /tmp/service-runtime ./labs/part2-backend/chapter21/service-runtime
go -C labs run ./part2-backend/chapter21/service-runtime
```

Expected output:

```text
service=checkout-runtime env=dev order=ORD-2001 status=created request_timeout=2s shutdown=graceful
```

## Code Map

- [main.go](main.go): defines config loading, probe handlers, timeout-wrapped
  runtime calls, shutdown hooks, and the bounded demo entrypoint.
- [main_test.go](main_test.go): verifies config validation, health and readiness
  probes, timeout behavior, structured logging, and graceful shutdown.

## Project Use

This topic is the "engineering boundary" half of the basic Go capstone. It
assumes the transaction boundary from [chapter12/transactions](../../chapter13/transactions)
already exists and focuses on what a deployable runtime needs around it.

## Related Exercises

- [Chapter 21 Exercises](../../../../content/book/part2-backend/chapter21.md#练习): return to the chapter page for
  executable practice tasks that extend the runtime skeleton with more
  delivery-oriented concerns.

## Continue

Return to [chapter21/README.md](../../../../content/book/part2-backend/chapter21.md) for the chapter map, then continue to
[chapter16](../../../../content/book/part2-backend/chapter16.md) when you want to move from the basic Go
capstone into the CloudWeGo framework chapters.
