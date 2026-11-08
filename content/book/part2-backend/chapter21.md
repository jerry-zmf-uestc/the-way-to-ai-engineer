# 第 21 章：发布、关闭与综合演练

事务章节解决多步骤业务怎样一起成功或一起失败；本章解决边界清楚的服务怎样工程化落地——配置兜底、取消传播、探针信号、日志定位。事务在这里只是前置能力，不再讲 begin / commit / rollback。

## 发布时给已有工作留一个出口

发布的第一个判断是是否还能接纳新任务。摘流量后，即使旧连接还打到实例，也应拒绝新的提交，允许查已有任务和确认已接纳的重试。第二个判断是当前进程能否在关闭预算内排空。内存队列适合看清这个时序；进程退出后要继续查询，任务和结果必须保存在能跨进程读取的地方。

[报表综合实验](../../../labs/part2-backend/chapter21/report-flow/README.md)提供可控摘流量：暂停模式先接纳一个任务，再摘流量，观察 readiness 变为 503、新提交被拒、旧任务仍能完成并查询。随后用同一个 SQLite 文件分别演练结果提交前和提交后、确认前退出。重启后按原任务 ID 查询 CSV，并用写入次数与待办确认核对恢复；再对照[真实持久化恢复](../../../labs/part2-backend/chapter12/report-sql/README.md)中的新进程，说明两者各依赖哪份事实。

## 21.1 配置加载：默认值 → env 覆盖 → Validate

**配置加载顺序必须稳定：先默认值，再环境变量覆盖，最后统一校验。** runtime 的各个部件接收同一份已校验配置，不再分别兜底默认值。

```go
// 摘录自 service-runtime/main.go。
func LoadConfig(env map[string]string) (Config, error) {
	cfg := DefaultConfig()

	if value := strings.TrimSpace(env["SERVICE_NAME"]); value != "" {
		cfg.ServiceName = value
	}
	if value := strings.TrimSpace(env["APP_ENV"]); value != "" {
		cfg.Environment = value
	}
	if value := strings.TrimSpace(env["HTTP_ADDR"]); value != "" {
		cfg.HTTPAddr = value
	}
	if value := strings.TrimSpace(env["REQUEST_TIMEOUT"]); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("%w: parse REQUEST_TIMEOUT: %v", ErrInvalidConfig, err)
		}
		cfg.RequestTimeout = parsed
	}
	if value := strings.TrimSpace(env["SHUTDOWN_TIMEOUT"]); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("%w: parse SHUTDOWN_TIMEOUT: %v", ErrInvalidConfig, err)
		}
		cfg.ShutdownTimeout = parsed
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
```

- 解析失败用 `ErrInvalidConfig` 包装，非法值在启动边界被拒绝，不会流到运行期。

## 21.2 服务入口统一包裹 timeout、日志和业务调用

**运行管理的要求由 runtime 入口负责，service 只管业务规则。** 入口为业务调用设置超时、补齐日志字段、记录错误：

```go
// 摘录自 service-runtime/main.go。
func (demoCheckoutService) PlaceOrder(ctx context.Context, req CheckoutRequest) (OrderSummary, error) {
	select {
	case <-ctx.Done():
		return OrderSummary{}, ctx.Err()
	case <-time.After(20 * time.Millisecond):
		return OrderSummary{OrderID: req.OrderID, Status: "created"}, nil
	}
}
```

- 日志字段（含请求 ID 的来源与缺省策略）在入口统一添加；**不要记录完整凭据、用户敏感输入或原始连接串**，排查通常只需要脱敏标识和错误类别。
- 本例只覆盖结构化日志，未实现完整的指标与追踪系统。

## 21.3 优雅关闭：先摘流量，再复制 hooks，最后带超时清理

**Shutdown 三个动作的顺序不能乱：先 `SetReady(false)` 摘流量，再复制 hooks 快照，最后带超时逐个清理。**

```go
// 摘录自 service-runtime/main.go。
func (a *App) Shutdown(ctx context.Context) error {
	a.SetReady(false)

	shutdownCtx, cancel := context.WithTimeout(ctx, a.cfg.ShutdownTimeout)
	defer cancel()

	a.hooksMu.Lock()
	hooks := append([]shutdownHook(nil), a.hooks...)
	a.hooksMu.Unlock()

	for _, hook := range hooks {
		if err := hook(shutdownCtx); err != nil {
			a.logger.ErrorContext(shutdownCtx, "shutdown_failed",
				slog.String("service", a.cfg.ServiceName),
				slog.String("error", err.Error()),
			)
			return err
		}
	}

	a.logger.InfoContext(shutdownCtx, "shutdown_complete",
		slog.String("service", a.cfg.ServiceName),
		slog.Duration("timeout", a.cfg.ShutdownTimeout),
	)
	return nil
}
```

- 复制 hooks 是为避免 shutdown 过程中直接操作共享切片；`WithTimeout` 只提供取消信号和截止时间，**Context 取消是协作式的——忽略 ctx 的 hook 仍可能一直阻塞**。
- 就绪探针（readiness）回答"现在能不能接新流量"，存活探针（liveness）回答"进程还活着吗"。`SetReady(false)` 只是发出状态信号，流量摘除还取决于部署平台的探测与传播时间。
- 当前实现遇到第一个 hook 错误就返回，后面的 hook 不会执行；正式应用需要明确是否继续清理其他资源、如何汇总错误以及进程最终退出策略。

完整 demo 把 config、timeout、readiness、graceful shutdown 四个边界串起来，在仓库根目录运行：

```bash
go -C labs run ./part2-backend/chapter21/service-runtime
```

预期输出：

```text
service=checkout-runtime env=dev order=ORD-2001 status=created request_timeout=2s shutdown=graceful
```

源码重点看 `LoadConfig`、`PlaceOrder`、`Shutdown`；测试重点看 timeout、probe、shutdown 三条路径（[service-runtime](../../../labs/part2-backend/chapter21/service-runtime/README.md)）。

## 从用户症状追到耗时的那一层

浏览器 Network 面板能看到提交请求超时；它不知道网关是否等到了 RPC，也不知道导出任务是否已经入队。服务端的日志、指标和追踪各自回答一个问题：

| 信号 | 适合回答 | 在导出链路上记录什么 |
| --- | --- | --- |
| 指标 | 影响多少请求，何时开始 | 入口速率、错误率、延迟分布、连接池和队列饱和度；单独统计接纳和完成 |
| Trace | 这一次请求在哪一段等待 | 网关、RPC、数据库跨度，以及发布和消费之间的关联；异步任务不能假定与 HTTP 同时结束 |
| 结构化日志 | 某次尝试发生了什么 | 请求标识、任务标识、失败类型、阶段和配置版本；不记录凭据或完整请求体 |

先定义用户目标的分母。例如“过去 30 分钟内，有权限且参数合法的导出提交中，有多少在 300ms 内确认接纳”，以及“已接纳任务中，有多少在约定时间完成”。指标用路由模板、结果类别等有限取值的标签；逐请求 ID 适合放日志或追踪，高基数会增加指标存储成本。

如果提交 p95 没变，用户却说导出一直转圈，先看任务完成率、最旧待处理任务年龄与 worker 失败率；再用任务 ID 找相关事件。如果提交本身变慢，按网关、RPC、数据库池等待和 SQL 执行耗时逐层缩小范围。Trace 可能采样，缺少某条跨度不能单独证明没执行。当前 Lab 演示日志、探针与关闭，跨服务追踪是延伸实践。

<a id="report-export-flow"></a>

## Part 2 综合实验 · 报表导出从提交到结果

打开 [报表导出综合实验](../../../labs/part2-backend/chapter21/report-flow/README.md)，用本地 HTTP 走完提交、排队、后台执行和结果查询。启动后可在浏览器打开本地练习页，提交任务并观察轮询、CSV 和队列指标。先跑正常链路，按任务 ID 对照日志和 CSV。随后每次重启只开启一种故障，在响应、队列指标和执行日志中各找一条证据。

| 故障位置 | 用这个实验回答 |
| --- | --- |
| [缓存](chapter14.md)与[队列确认](chapter15.md) | 为什么缓存还显示运行中？为何投递两次却只生成一份报表？ |
| [实例发现](chapter18.md)与[提交重试](chapter19.md) | A 下线后请求去了哪里？客户端没收到响应时怎样找回原任务？ |
| [容量保护](chapter20.md)与任务积压 | 何时返回 503？停止接收或恢复消费后，怎样验证旧任务已排空？ |

## 踩坑清单

### 配置读取分散到 handler 或 service，各层自己兜底

```go
func handler(w http.ResponseWriter, r *http.Request) {
	timeout := os.Getenv("REQUEST_TIMEOUT")
	env := os.Getenv("APP_ENV")
	_ = timeout
	_ = env
}
```

现象：默认值、格式解析、非法值和配置文档在各层重复处理，且互相不一致。修复：启动时统一执行 `LoadConfig`，运行时只使用已校验的 `Config`。

### 创建了 timeout，却在调用业务时改用 Background

```go
func (a *App) PlaceOrder(ctx context.Context, req CheckoutRequest) (OrderSummary, error) {
	requestCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()

	return a.service.PlaceOrder(context.Background(), req) // 反例：真正执行时已经感知不到 timeout 和 cancel。
}
```

现象：代码表面"看起来已经加了 timeout"，但 service 收到的不是 `requestCtx`，下游根本不会被取消。修复：谁创建了派生 context，谁就必须把它一路传下去；测试用可感知取消的 fake 断言业务收到 Done，不要只断言创建了 Context。

### readyz 永远返回 200

```go
func (a *App) routes() {
	a.mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // 反例：关闭流量前仍然告诉平台"我还 ready"。
	})
}
```

现象：滚动发布时平台没法先摘流量，实例无法优雅关闭。修复：Shutdown 开始先 `SetReady(false)`，让就绪检查读取实际状态；反过来，依赖短暂故障时不要把 liveness 也判失败，否则会造成反复重启。

修复后在独立副本把非法配置、取消传播、关闭期探针三类边界输入补进测试再复验；已有测试通过只说明已覆盖的用例符合预期，新增边界仍要补断言。

<a id="练习"></a>

## 动手练

先运行配套实验并记录故障与恢复时的指标。[报表任务积压与恢复](../../../labs/part2-backend/chapter21/report-backlog/README.md)

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 21.1 增加日志级别配置 | `Config` 增加 `LogLevel`：默认 info，`LoadConfig` 读取 `LOG_LEVEL`，`Validate` 只接受 debug/info/warn/error；logger 初始化只接收配置，不再读环境 | 缺省、合法、非法三类输入的测试全部通过 | `service-runtime/main.go`、`main_test.go` |
| 21.2 把请求 ID 加入日志 | 由 runtime 入口获得 requestID，在 started / failed / completed 日志中添加同一字段 | 解码结构化日志断言同一次请求的 request_id 一致，不依赖字段打印顺序 | `service-runtime/main.go`、`main_test.go` |
| 21.3 给转圈定位 | 比较“提交慢”与“接纳后一直不完成”两组现象，各写出首查指标与下一步证据 | 两组能落到不同阶段，能用任务 ID 串起日志，不把任务 ID 放进指标标签 | 本章报表案例 |

完成后运行：

```bash
go -C labs test ./part2-backend/chapter21/service-runtime/... -count=1
```

更多练习（接入事务业务服务等）见各 lab 的 README。

## 链接

- [Google SRE：分布式系统监控](https://sre.google/sre-book/monitoring-distributed-systems/)：从延迟、流量、错误和饱和度判断用户影响。
- [Go 标准库：log/slog](https://pkg.go.dev/log/slog)：结构化日志的字段、级别与 Handler。
- [Go 标准库：net/http](https://pkg.go.dev/net/http)：对照 `Server.Shutdown` 的等待与超时行为。
- [Go 标准库：context](https://pkg.go.dev/context)：取消、截止时间及派生 Context 的传播方式。

[返回全书目录](../README.md) · [上一篇：可观测性与事故诊断](chapter18.md) · [下一篇：Agent 架构与模型接口](../part3-agent/building/chapter22-architecture.md)
