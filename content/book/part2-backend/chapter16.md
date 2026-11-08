# 第 12 章：请求边界与任务 API

点击「导出」后，页面收到一个任务 ID，几次轮询后才看见 CSV。先打开 [报表导出浏览器实验](../../../labs/part2-backend/chapter21/report-flow/README.md)，在 Network 面板跟一遍 POST 和 GET。POST 承诺任务已被接纳，GET 才回答执行到了哪一步。

## 从 UI 状态到服务端承诺

| 页面动作 | HTTP 契约 | 服务端事实 |
| --- | --- | --- |
| 点击导出 | POST 返回 202 和任务 ID | 校验请求与身份，接纳任务并记录初始状态 |
| 等待结果 | GET 返回 200 和状态 | 从任务记录读取 queued、running 或 succeeded |
| 展示 CSV | succeeded 才包含结果 | 后台执行方已经提交结果 |
| 没收到响应 | 本地标为结果未知 | 客户端没有足够证据判断是否创建，保留原键 |

让前端把「提交中」和「后台运行中」分开。网络请求结束并不会结束已接纳的任务；取消轮询只停止查询。服务端应为重复查询提供稳定标识，并为输入错误、资源不存在和暂时不可用保留不同的 HTTP 语义。生产场景还要定义失败终态、结果保留期和查询限额。

## 任务 ID 是索引，身份决定能否读取

拿到一个 ID 并不代表持有者有权访问该资源。导出服务应在查任务和取结果时都用已认证的身份校验归属。入口从认证上下文取得身份，存储层按身份与资源查询，避免只在前端隐藏按钮。幂等键也应按身份和操作隔离，否则两个用户碰巧用了同一个键就会共享任务。综合 Lab 用两个固定教学身份验证同键隔离和跨用户 404，身份头只用于本地实验。

下面的 Hertz 示例展示入口职责如何落到框架里。完整跨进程契约与实例发现见[第 17 章](chapter17.md)。

## 1. 组合 engine、middleware 和 route group

**网关入口 = engine + 全局中间件 + 路由组**：engine 负责监听，middleware 负责横切逻辑，route group 负责路径前缀。注册路由时先分清哪些中间件是全局的、哪些请求字段供所有 handler 读取。

```go
func newRouter() *server.Hertz {
	h := server.New(server.WithHostPorts(":8888"))
	h.Use(requestIDMiddleware())

	h.GET("/healthz", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, healthResponse{
			Status:    "ok",
			RequestID: requestID(c),
		})
	})

	api := h.Group("/api")
	api.GET("/books/:id", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, bookResponse{
			ID:        c.Param("id"),
			RequestID: requestID(c),
		})
	})
	api.GET("/search", func(ctx context.Context, c *app.RequestContext) {
		query := c.Query("q")
		if query == "" {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "q is required"})
			return
		}
		c.JSON(consts.StatusOK, searchResponse{
			Query:     query,
			RequestID: requestID(c),
		})
	})

	return h
}
```

摘录自 [hertz-routing/main.go](../../../labs/part2-backend/chapter16/hertz-routing/main.go)。运行（Hertz 默认还会打印几行路由注册 debug 日志）：

```bash
go -C labs run ./part2-backend/chapter16/hertz-routing
```

```text
hertz routes ready: GET /healthz, GET /api/books/:id, GET /api/search?q=go
```

## 2. `RequestContext` 同时承担路径参数、查询串、请求级状态和响应回写

**请求级状态的流向是单向的**：header 先进入 middleware，被整理成稳定的 `request_id`；后续 handler 通过 `requestID(c)` 统一读取。trace 字段、审计字段和公共上下文不会散落到每个 handler 自己重复解析。

```go
func requestIDMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := string(c.Request.Header.Peek("X-Request-ID"))
		if requestID == "" {
			requestID = "generated-demo-id"
		}
		c.Set("request_id", requestID)
		c.Next(ctx)
		c.Response.Header.Set("X-Request-ID", requestID)
	}
}

func requestID(c *app.RequestContext) string {
	value, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	id, ok := value.(string)
	if !ok {
		return ""
	}
	return id
}
```

摘录自 [hertz-routing/main.go](../../../labs/part2-backend/chapter16/hertz-routing/main.go)。

## 3. 网关向后端转交 HTTP 请求

**网关只做三件事：提取 HTTP 参数、调用下游接口、把结果转换为 HTTP 响应**。修改库存、计算价格和查询数据库等业务判断，继续由 RPC 后面的服务处理。

```go
func getUserOrders(userClient userservice.Client) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		userID := c.Param("userID")
		resp, err := userClient.GetUserOrders(ctx, &commerce.UserOrdersRequest{UserId: userID})
		if err != nil {
			status := statusFromError(err)
			c.JSON(status, map[string]any{
				"status": status,
				"error":  err.Error(),
			})
			return
		}

		trace, _ := obs.TraceID(ctx)
		c.JSON(consts.StatusOK, map[string]any{
			"status":      consts.StatusOK,
			"user_id":     resp.GetUser().GetId(),
			"user_name":   resp.GetUser().GetName(),
			"orders":      len(resp.GetOrders()),
			"total_cents": total(resp.GetOrders()),
			"trace":       trace,
			"attempts":    1,
		})
	}
}
```

摘录自 [hertz-kitex-real-chain/main.go](../../../labs/part2-backend/chapter16/hertz-kitex-real-chain/main.go)，真实调用链从这里跟到 `internal/gateway/server.go`。

- 本章三个 lab 对应三层深度，按此顺序读：`hertz-routing` 是最小路由层（先看 `newRouter` 和 `requestIDMiddleware`）；`hertz-kitex-real-chain` 演示 gateway -> user/order/product 的一次完整调用；`hertz-kitex-demo` 是分层工程版（`cmd + internal`），入口只做 `os.Exit(app.Run(os.Stdout, os.Stderr, os.Environ()))`，wiring 留在 [cmd/main.go](../../../labs/part2-backend/chapter16/hertz-kitex-demo/cmd/main.go)，适合长期演进。

## 踩坑清单

### 把业务逻辑直接塞进网关 handler

```go
func bookHandler(ctx context.Context, c *app.RequestContext) {
	total := recalculateInventoryAndPrice(c.Param("id"))
	c.JSON(200, map[string]any{"total": total})
}
```

现象：网关很快长成"第二个业务服务"，HTTP 边界、领域逻辑和数据访问缠在一起。修复：把库存与价格计算移入下游服务，handler 只做参数提取、错误映射和向下游接力；用 fake 下游覆盖成功与失败，HTTP 状态和字段保持原契约。

### middleware 只改响应头，不把请求级状态放进 context

```go
func badRequestIDMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := string(c.Request.Header.Peek("X-Request-ID"))
		c.Response.Header.Set("X-Request-ID", requestID)
		c.Next(ctx)
	}
}
```

现象：浏览器看起来"收到了 request id"，但下游 handler 和 RPC client 拿不到统一的请求级状态——`RequestContext.Set` 只是 Hertz 请求内状态，RPC 下游不会自动读取。修复：除设置响应头外，把 ID 放进请求状态并显式写入 RPC 元数据、由下游中间件恢复；同一请求在入口与下游观察到相同 ID 才算通过，只测响应头不算。

### 学习 demo 默认永久阻塞

```go
func run() {
	router := newRouter()
	router.Spin() // 反例：`go run` 默认就卡住，自动化和教学演示都不好收尾。
}
```

现象：`go run` 一直监听，验证链路无法自然收口。修复：`hertz-routing` 默认以 non-blocking 有界演示运行，完成输出即退出；需要长期服务时选显式 `-serve` 模式，用取消或关闭信号结束。

### 所有错误都返回 200

参数错误、查无数据、下游超时与未知异常要映射为不同 HTTP 行为；也不要把含内部地址或凭据的原始错误直接展示给用户。鉴权是网关入口的职责之一，但本章三个 demo 并未提供完整认证系统。

### 把演示值当成真实基础设施

`generated-demo-id` 只是固定演示值，不具备全局唯一性，不能用作鉴权依据。真实调用链 demo 只在本机启动 RPC 服务，不等于访问远端生产；运行前确认端口和关闭路径，失败后先清理残留进程再重跑，避免第二次运行受残留进程影响。

已有测试通过只说明已覆盖的用例符合预期，新增边界仍要补充断言。

<a id="练习"></a>

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 16.1 增加 authors 路由 | 复用已有 api 路由组加 `/authors/:id`：`c.Param("id")` 取参数、`requestID(c)` 取请求 ID、返回 JSON，并确认继承了已有中间件 | `/api/authors/42` 返回 id=42 和 request_id，状态码 200 | `hertz-routing/main.go` |
| 16.2 检查请求 ID 的传递 | 用 `ut.PerformRequest` 发带 `X-Request-ID: demo-123` 的请求，分别断言响应头与 body.request_id，再测未传 header 时的演示默认值 | demo-123 同时出现在响应头与 JSON body 中；只查响应头不能证明 handler 收到了状态 | `hertz-routing/main_test.go` |

完成后运行：

```bash
go -C labs test ./part2-backend/chapter16/hertz-kitex-demo/... -count=1
go -C labs test ./part2-backend/chapter16/hertz-kitex-real-chain/... -count=1
go -C labs test ./part2-backend/chapter16/hertz-routing/... -count=1
```

更多练习见各 lab 的 README。

## 链接

- [Hertz 官方文档](https://www.cloudwego.io/docs/hertz/)：路由、中间件和服务运行的官方说明；接口细节以本仓锁定版本（Hertz v0.10.5、Kitex v0.16.2，见 `labs/go.mod`）为准。
- 《Go 入门指南》未直接覆盖 Hertz，以下三节与本章的网关职责最接近：[一个简单的 Web 服务器](https://learnku.com/docs/the-way-to-go/152-a-simple-web-server/3704)、[让 Web 应用更健壮](https://learnku.com/docs/the-way-to-go/155-making-a-web-application-robust/3707)、[功能更完整的 WebServer](https://learnku.com/docs/the-way-to-go/158-a-versatile-and-delicate-webserver/3710)。

[返回全书目录](../README.md) · [上一篇：Go 工程实践](../part1-golang/chapter11.md) · [下一篇：数据建模、SQL 与索引](chapter12.md)
