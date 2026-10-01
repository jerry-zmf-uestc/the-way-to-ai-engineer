# 示例： hertz kitex real chain

> 定位：这是 CloudWeGo 学习路径里的**学习版 / 最小闭环版**。
>
> 如果你要看仓库级、可长期演进的产品化入口，请使用 `labs/part2-backend/chapter16/hertz-kitex-demo/cmd` 与 `labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/`。

## 位置

- 框架入门：<https://www.cloudwego.io/docs/kitex/getting-started/quick_start/>（先运行当前目录示例，再对照官方用法）

## Goal

用 Hertz 起 HTTP 网关，Kitex 起 UserService / OrderService / ProductService 三节 RPC 链，**单进程内**起 4 个真 server（各自 `net.Listen("tcp","127.0.0.1:0")` 拿随机端口），网关向前把 `GET /api/users/:userID/orders` 打通到 Kitex 尾端 Product 服务；打 1 次请求 → 打印单行 stdout → 级联 Shutdown。

```text
Frontend / App
   ↓ HTTP :random
Hertz Gateway (/api/users/:userID/orders)
   ↓ Kitex RPC :random
Kitex UserService.GetUserOrders
   ↓ Kitex RPC :random
Kitex OrderService.ListOrders
   ↓ Kitex RPC :random
Kitex ProductService.GetProduct
```

## 生产实践清单

按 spec §Design §1 全部要落：

1. **Middleware**：Hertz `Recover + Logging + inject trace-id`；Kitex client `KitexClientObserve`
2. **超时**：Kitex client `WithRPCTimeout(500ms)`；Hertz client `Timeout(2s)`；Shutdown 500ms
3. **幂等重试**：Kitex `client.WithFailureRetry` + `retry.NewFailurePolicy`（MaxAttempts=3，幂等读方法专用）
4. **优雅关闭**：`chain.Runner` 按 Gateway → User → Order → Product 反序 `Shutdown(ctx)`；二进制只构建到 `/tmp`
5. **配置外部化**：`internal/config.Config` + `FromEnv()`；defaults 保底
6. **健康检查**：三个 Kitex 服务暴露 `Ping()`；Hertz 暴露 `/healthz`

## 当前状态

**Complete** —— 单进程 4 个真 server 已落地：Product / Order / User 用 Kitex，Gateway 用 Hertz；`go run` 会启动完整链路、打一条 HTTP 请求、打印确定性 stdout 并级联关闭。

## 运行

```bash
export PATH="$PATH:$(go env GOPATH)/bin"    # 首次装 kitex/thriftgo 后一次即可
go build -o /tmp/hertz-kitex-real-chain ./labs/part2-backend/chapter16/hertz-kitex-real-chain
go run  ./labs/part2-backend/chapter16/hertz-kitex-real-chain
# stdout: demo=user-orders status=200 orders=2 total_cents=2497 trace=Frontend→Hertz→UserSvc→OrderSvc→ProductSvc attempts=1
```

也可以在示例目录内执行 `make test` 和 `make demo`；如果本机默认 PATH 没有 Go 1.26，请用 `<go-bin>/go` 直接跑验收命令。

## 目录结构（终态）

```text
labs/part2-backend/chapter16/hertz-kitex-real-chain/
├── main.go                     # bounded demo 编排
├── main_test.go                # TestMainDemo 单行断言
├── idl/commerce.thrift         # 契约
├── kitex_gen/commerce/         # kitex CLI 生成产物（commit）
├── internal/
│   ├── config/                 # Config + FromEnv
│   ├── obs/                    # slog + trace-id + middleware
│   ├── chain/                  # Server 抽象 + Runner
│   ├── product/                # ProductService handler + Start
│   ├── order/                  # OrderService handler + Start
│   ├── user/                   # UserService handler + Start
│   └── gateway/                # Hertz 路由 + Kitex client 装配
├── Makefile
├── meta.yml
└── README.md
```

## 关键源码索引

- [main.go](main.go)：编排 bounded demo，启动网关与三个 Kitex 服务，打一条请求后级联关闭。
- [main_test.go](main_test.go)：对单行 stdout 做端到端断言，保证教学输出保持稳定。
- [idl/commerce.thrift](idl/commerce.thrift)：四个服务共享的 RPC 契约来源。
- [internal/gateway/server.go](internal/gateway/server.go)：Hertz 路由、Kitex client 装配与 HTTP 入口。
- [internal/user/handler.go](internal/user/handler.go)：UserService，把 HTTP 入参继续向下游 OrderService 传递。
- [internal/order/handler.go](internal/order/handler.go)：OrderService，负责订单聚合并调用 ProductService。
- [internal/product/handler.go](internal/product/handler.go)：ProductService，提供最尾端商品查询。
- [internal/chain/runner.go](internal/chain/runner.go)：统一关闭顺序与优雅退出边界。
- [internal/obs/middleware.go](internal/obs/middleware.go)：trace-id、日志与 middleware 的观测入口。

## 重新生成 kitex_gen

```bash
make -C labs/part2-backend/chapter16/hertz-kitex-real-chain gen
```

**依赖版本**（与本 spec 绑定）：

- `kitex v0.16.3`
- `thriftgo v0.4.5`

安装：

```bash
go install github.com/cloudwego/kitex/tool/cmd/kitex@v0.16.2
go install github.com/cloudwego/thriftgo@v0.4.5
```

## 距离生产还缺什么（Non-Goals，见 spec §Requirements §8）

- **多进程/独立 cmd**：本 demo 单进程；生产要拆 `cmd/gateway` `cmd/user` `cmd/order` `cmd/product`
- **服务发现**：demo 里 client 直接拿 `Listener.Addr()`；生产走 etcd / nacos / consul
- **指标 / trace**：本 demo 只有 `trace_id` 串日志；生产接 Prometheus + OpenTelemetry
- **TLS / mTLS**：本 demo 明文；生产必须 mTLS 或 SPIFFE
- **限流 / 熔断**：本 demo 不做
- **配置中心**：`FromEnv()` 仅本地；生产走 apollo / nacos / configmap
- **CI/CD & 多副本 & 蓝绿**：本 demo 不覆盖

## 相关练习

- [第 16 章练习](../../../../book/part2-backend/chapter16.md#练习)：回到正文里的练习区，按伪代码和验收标准继续
  扩展这个 topic。

## Continue

- 返回 [chapter16/README.md](../../../../book/part2-backend/chapter16.md) 看本章其余 Hertz / CloudWeGo 主题。
- 先读 [hertz-routing](../hertz-routing) 回到最小 HTTP 入口，再回来理解完整调用链。
- 再读 [kitex-idl-handler](../../chapter17/kitex-idl-handler) 看下一章如何把“服务能跑”推进到“契约可演进”。

## 参考

- [HTTP 生命周期](https://pkg.go.dev/net/http#Server.Shutdown) — HTTP server 生命周期
- [Kitex 代码生成](https://www.cloudwego.io/docs/kitex/tutorials/code-gen/) — Kitex handler + kitex_gen
- [HTTP 客户端](https://pkg.go.dev/net/http#Client) — Middleware / RetryPolicy 心智
- [CloudWeGo Kitex retry](https://www.cloudwego.io/docs/kitex/tutorials/service-governance/retry/)
- [CloudWeGo Kitex timeout](https://www.cloudwego.io/docs/kitex/tutorials/service-governance/timeout/)
- [CloudWeGo Hertz middleware](https://www.cloudwego.io/docs/hertz/tutorials/basic-feature/middleware/)
