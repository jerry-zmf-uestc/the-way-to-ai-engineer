# 示例： Hertz Routing and Middleware

## Goal

Learn the smallest useful Hertz route setup: a health route, a grouped API route with a path parameter, a query-driven route, and one middleware that propagates a request ID.

## Concepts

- `server.New()` creates a Hertz engine without default middleware.
- `h.Use(...)` registers middleware for all routes.
- `h.Group("/api")` keeps related routes under one prefix.
- `c.Param("id")` reads path parameters such as `/api/books/:id`.
- `c.Query("q")` reads query parameters such as `/api/search?q=hertz`.
- `c.Next(ctx)` continues the middleware chain, then post-processing can set response headers.

## Run

```bash
gofmt -l labs/part2-backend/chapter16/hertz-routing
go -C labs vet ./part2-backend/chapter16/hertz-routing
go -C labs test ./part2-backend/chapter16/hertz-routing -race -count=1
go -C labs test ./part2-backend/chapter16/hertz-routing -cover
go build -o /tmp/hertz-routing ./labs/part2-backend/chapter16/hertz-routing
go -C labs run ./part2-backend/chapter16/hertz-routing
```

Expected output:

```text
hertz routes ready: GET /healthz, GET /api/books/:id, GET /api/search?q=go
```

Hertz may also print several debug lines while registering routes. The stable,
reader-facing line is the final `hertz routes ready: ...`.

## Code Map

- [main.go](main.go): defines the Hertz engine, middleware, grouped routes, and
  the bounded demo request flow.
- [main_test.go](main_test.go): checks the route behavior without turning the
  example into a long-lived server process.

## Project Use

This topic is the first CloudWeGo building block. Later examples can reuse the same route grouping and request ID middleware shape when adding Kitex calls, timeout control, and trace propagation.

## Related Exercises

- [Chapter 16 Exercises](../../../../content/book/part2-backend/chapter16.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [chapter16/README.md](../../../../content/book/part2-backend/chapter16.md) for the chapter map, then continue with
[hertz-kitex-real-chain](../hertz-kitex-real-chain) to see the same HTTP entry
layer call real Kitex services.
