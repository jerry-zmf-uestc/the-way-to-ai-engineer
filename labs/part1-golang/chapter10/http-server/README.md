# 示例： HTTP Server

## Goal

Use the standard library to build one minimal `/healthz` endpoint, then compare
two ways to drive it: `httptest.NewServer` for test-friendly demos and
`http.Server + net.Listener + Shutdown` for production-style lifecycle control.

## Core Model

| Layer | In code | Responsibility |
| --- | --- | --- |
| Routing | `NewServer()` | Register `/healthz` on a `http.ServeMux` |
| Handler | `HealthHandler` | Validate method and return stable JSON |
| Demo A | `DemoRequest` | Use `httptest.NewServer` to exercise the handler |
| Demo B | `DemoRequestListenAndServe` | Use `http.Server`, random port, and graceful shutdown |

The important lesson is not “how to print JSON”, but how to keep HTTP behavior
bounded and testable without leaving a listener running forever.

## Run

```bash
go -C labs test ./part1-golang/chapter10/http-server -race -count=1
go -C labs run ./part1-golang/chapter10/http-server
```

Expected output:

```text
[A httptest]        GET /healthz -> 200 {"status":"ok"}
[B ListenAndServe]  GET /healthz -> 200 {"status":"ok"}
```

## Code Map

- [main.go](main.go)
  - `NewServer` wires the mux
  - `HealthHandler` keeps the endpoint logic small
  - `DemoRequest` shows the `httptest.NewServer` path
  - `DemoRequestListenAndServe` shows the explicit `http.Server` lifecycle
- [main_test.go](main_test.go)
  - table-driven cases cover `GET`, `HEAD`, invalid method, and unknown route
  - `ExampleDemoRequest` locks the demo output
- API reference:
  [Server.Shutdown](https://pkg.go.dev/net/http#Server.Shutdown)

## Common Pitfalls

- Calling `http.ListenAndServe` directly in learning demos and forgetting it blocks forever.
- Testing handlers through a real port when `httptest.NewRecorder` or `httptest.NewServer`
  would make the behavior deterministic.
- Writing headers after the body has already been written.
- Forgetting graceful shutdown when switching from demo code to long-lived server code.

## Related Exercises

- [Chapter 10 · HTTP Exercises](../../../../content/book/part1-golang/chapter10.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

After this topic, move to [http-middleware-client](../http-middleware-client),
where the server side gains middleware and the client side learns timeout and retry behavior.

对照本 topic：[main_test.go](main_test.go) 里的 `TestNewServerHealthz` 用 `NewRecorder`；`DemoRequest` 用 `NewServer`，正好覆盖两条路。

## Layer 8 — 进阶主题（知道有就行）

| 主题 | 关键词 |
|---|---|
| **HTTPS / mTLS** | `tls.Config`, `srv.ListenAndServeTLS`, `crypto/tls` |
| **HTTP/2** | `net/http` 默认支持；`golang.org/x/net/http2` 显式配置 |
| **HTTP/3 (QUIC)** | 标准库不带，用 `quic-go` |
| **反向代理** | `httputil.ReverseProxy`，改 `Director` / `ModifyResponse` |
| **流式响应** | `http.Flusher`（SSE、chunked） |
| **WebSocket 升级** | `http.Hijacker`，或用 `gorilla/websocket` / `nhooyr.io/websocket` |
| **性能** | 连接池 (`Transport.MaxIdleConns...`)、`sync.Pool`、`net/http/pprof` |
| **框架** | `chi`（路由）、`gin`（生态大）、`echo`、`fiber`（fasthttp）、`go-kit`（分层） |
| **RPC 替代** | gRPC（`google.golang.org/grpc`）、CloudWeGo Kitex（见 [../../chapter16/kitex-rpc/](../../../part2-backend/chapter17/kitex-rpc)） |

---

## 学习路径建议（按顺序打卡）

1. **写一个 `/healthz`**：`Handler` + `ServeMux` + `ListenAndServe`。← 本 topic 已覆盖
2. **加优雅关闭**：`http.Server` + goroutine + `Shutdown` + signal。← 本 topic 方案 B 已覆盖
3. **写一个 JSON POST API**：`json.NewDecoder`、`Content-Type` 校验、字段验证、`http.Error`。← 见 [../../chapter11/practice-json-api/](../../chapter11/practice-json-api) 和分层版 [../../chapter11/json-api/cmd/](../../chapter11/json-api/cmd)
4. **加中间件三件套**：Logging / Recover / Timeout。← 建议下一步补
5. **写 Client 调下游**：`http.Client{Timeout}` + `NewRequestWithContext` + `defer Body.Close`。← 建议下一步补
6. **测试三种写法**：Recorder / NewServer / 端到端。← 本 topic 已覆盖前两种
7. **HTTPS + Graceful Shutdown 完整版**：`ListenAndServeTLS` + signal + `errgroup`。
8. **反向代理**：`httputil.ReverseProxy` 改 Director。
9. **HTTP/2 push、SSE、WebSocket 升级**：至少各写一个 demo，理解 `Flusher` / `Hijacker`。

## 必背 API 清单

**服务端**

```
http.Handler / http.HandlerFunc
http.ServeMux + NewServeMux() / mux.HandleFunc / mux.Handle
http.Server{Addr, Handler, ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout}
srv.ListenAndServe / srv.Serve / srv.ListenAndServeTLS / srv.Shutdown / srv.Close
http.Error / http.Redirect / http.NotFound
http.ServeFile / http.FileServer / http.StripPrefix
http.MethodGet/Post/... / http.StatusOK/BadRequest/...
http.ErrServerClosed
```

**Request / Response**

```
*http.Request.Method / URL / Header / Body / Context() / PathValue / FormValue
http.ResponseWriter.Header() / WriteHeader / Write
http.Header.Get / Set / Add / Del / Values
```

**客户端**

```
http.Client{Timeout, Transport, CheckRedirect}
http.NewRequestWithContext(ctx, method, url, body)
client.Do(req) / http.Get/Post/Head
resp.Body.Close() 【必背】
http.Transport{MaxIdleConns, MaxConnsPerHost, IdleConnTimeout, TLSClientConfig}
```

**测试**

```
httptest.NewRequest / NewRecorder / NewServer / NewTLSServer
rec.Code / rec.Body / rec.Header() / rec.Result()
```

**Context**

```
r.Context() / r.WithContext(ctx)
context.WithTimeout / WithCancel / WithValue
```

---

## 来源

- <https://pkg.go.dev/net/http>
- <https://pkg.go.dev/net/http/httptest>
- <https://pkg.go.dev/context>
- Effective Go, "Concurrency" 与 "Errors" 章节
