# 第 10 章：Web 服务

配套实验：[HTTP server](../../../labs/part1-golang/chapter10/http-server/README.md)、[中间件与客户端](../../../labs/part1-golang/chapter10/http-middleware-client/README.md)、[GORM 与 MySQL](../../../labs/part2-backend/chapter12/gorm-mysql/README.md)、[repository pattern](../../../labs/part2-backend/chapter12/repository-pattern/README.md)。

Web 服务的复杂度集中在两个边界：服务端怎么处理进来的请求，客户端怎么调用别的服务。本章用标准库 `net/http` 和 `database/sql` 把这两个边界打通。

这两个边界对 AI 工程师尤其具体：Agent 的一次工具调用，落到传输层就是一次 HTTP 事务。一次研究型任务可能发出几十次下游调用，每次调用的 P99 假设是 300ms，但只要其中一次遇到挂死的下游、`http.DefaultClient` 又没有超时，那个 goroutine 就会永远等下去——连接、内存和重试预算一起被拖住，进程表面还活着，实际上已经停摆。本章的每个机制都对应这类真实故障的一种解法。

## 10.1 一次请求跨过 URL、DNS 与 HTTP

**一次网页访问 = DNS 解析 → TCP 连接 → HTTP 请求/响应 → 渲染。** 浏览器输入 URL 回车之后，依次发生：

1. 浏览器请求 DNS 服务器，把域名解析成 IP 地址。
2. 通过 IP 找到服务器，建立 TCP 连接。
3. 浏览器发送 HTTP 请求包。
4. 服务器接收并处理请求，返回 HTTP 响应包。
5. 浏览器渲染响应体；解析到页面中的图片、CSS、JS 链接时，再发起新的静态资源请求。

记住这条链的价值在于排错：页面打不开时，先确定断在哪一环——DNS 解析失败、TCP 连不上、HTTP 返回错误码、还是渲染缺资源，每一环的排查工具完全不同。

### URL 与 DNS

URL 的基本格式：

```text
scheme://host[:port]/path/[?query-string][#anchor]
```

`scheme` 是底层协议（http、https、ftp），`host` 是 IP 或域名，HTTP 默认端口 80 可省略，`query-string` 是发给服务器的数据，`anchor` 是页面内锚点（不发送到服务器）。

DNS 把域名翻译成 IP，解析按顺序尝试：操作系统 hosts 文件 → 本地解析器缓存 → 首选 DNS 服务器（区域解析结果具有权威性，缓存结果不具权威性）→ 未设转发时从根 DNS 服务器逐级向下查询（迭代），设转发时把请求转交上一级 DNS（递归）。

### HTTP 协议

**HTTP 只能由客户端发起事务，本身无状态**：服务器不知道两次请求是否来自同一客户端，Web 程序因此引入 Cookie 维持会话。无状态不等于短连接——HTTP/1.1 起默认 Keep-Alive，TCP 连接可复用一段时间。

HTTP 建立在 TCP 之上（默认 80 端口）。请求包由请求行、请求头、空行和消息体组成：

```text
GET /domains/example/ HTTP/1.1     // 请求行：方法 URI 协议/版本
Host: www.iana.org                 // 请求头
User-Agent: Mozilla/5.0 ...
Accept: text/html,...
Accept-Encoding: gzip,deflate
                                   // 空行之后是消息体（GET 通常为空）
```

四种基本方法对应资源的 CRUD：GET（查）、POST（增）、PUT（改）、DELETE（删）。GET 与 POST 的关键区别：GET 参数附在 URL 后（`?` 分隔、`&` 连接），有长度限制且可能被缓存或历史记录泄露；POST 数据放在消息体中，无此限制。

响应包由状态行、响应头、空行和消息体组成：

```text
HTTP/1.1 200 OK                    // 状态行：协议版本 状态码 状态消息
Server: nginx/1.0.8
Content-Type: text/html
Content-Length: 90
                                   // 空行之后是消息体
```

状态码按首位数字分五类：1XX 提示信息、2XX 成功、3XX 重定向、4XX 客户端错误、5XX 服务器错误。最常用的如 200（正常）、301/302（跳转）、404（未找到）、500（服务器内部错误）。10.4 节的重试策略会直接消费这个分类——4XX 通常意味着请求本身有问题，重发无意义；5XX 和 429 才值得退避重试。

## 10.2 把服务装配与请求处理分开

**一个最小 Go Web 服务只需要两个函数：`http.HandleFunc` 注册路由，`http.ListenAndServe` 监听端口**——不需要 nginx 或 Apache 这类外部 Web 服务器。

最小服务骨架（省略包声明，不能独立运行）：

```go
func sayHello(w http.ResponseWriter, r *http.Request) {
	r.ParseForm() // 解析 URL 查询参数和表单；不调用则 r.Form 为空（常见坑）
	fmt.Fprintf(w, "Hello!") // 写入 w 的内容才返回给客户端；fmt.Println 只打印到服务端终端
}

func main() {
	http.HandleFunc("/", sayHello)      // 向默认路由器注册路由
	log.Fatal(http.ListenAndServe(":9090", nil)) // nil 表示使用默认路由器
}
```

- 两个输出方向不要混淆：`fmt.Println` 打印到服务端终端，`fmt.Fprintf(w, ...)` 写入响应发给客户端。
- 重复查询参数（`?url_long=111&url_long=222`）以切片形式存入 `r.Form`。

### 装配与处理分离：可测试的服务写法

[http-server](../../../labs/part1-golang/chapter10/http-server) 示例把路由装配和 handler 分开，便于测试。摘录自 [main.go](../../../labs/part1-golang/chapter10/http-server/main.go)：

```go
func NewServer() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", HealthHandler)
	return mux
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if err := validateHealthMethod(r.Method); err != nil {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	if err := json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}); err != nil {
		http.Error(w, fmt.Errorf("write health response: %w", err).Error(), http.StatusInternalServerError)
	}
}
```

注意这个 handler 的几个刻意设计：

- **方法校验先行**：`validateHealthMethod` 只放行 GET 和 HEAD，其余方法返回 405。健康检查是幂等只读端点，不该接受 POST。
- **HEAD 走完整状态路径但不写 body**：先设 Header、`WriteHeader(200)`，然后直接返回——HEAD 的语义就是"和 GET 一样的状态行与响应头，但没有消息体"。
- **写响应失败也要上报**：`json.NewEncoder(w).Encode` 的 error 没有被丢弃，而是转成 500。响应写一半失败通常意味着连接已断，但日志里需要这个信号。

handler 只处理请求/响应，路由由 `mux` 装配，监听端口交给程序入口——测试可以用 `httptest` 直接构造请求，不必启动长期服务。

### 两种驱动方式：httptest 与真实监听器

同一个 `NewServer()`，示例用了两种方式驱动它，恰好覆盖了"测试"与"生产"两个极端。方案 B 摘录自 [main.go](../../../labs/part1-golang/chapter10/http-server/main.go)（有删节）：

```go
srv := &http.Server{
	Handler:           handler,
	ReadHeaderTimeout: 2 * time.Second, // 防止 slow-header 之类的低级 DoS
}

// 独立 goroutine 跑 server；用带缓冲的 channel 把 Serve 的返回值送回来，
// 这样主流程能在关闭阶段判断是否有异常退出。
serveErr := make(chan error, 1)
go func() {
	serveErr <- srv.Serve(listener)
}()
```

```go
// http.ErrServerClosed 是 Shutdown 触发的正常信号，不算错误；其他任何错误都要如实上报。
if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
	return fmt.Sprintf("ERROR: serve: %v", err)
}
```

这段代码藏着三个生产经验：用 `"127.0.0.1:0"` 让内核分配随机端口，`Listen` 返回时端口就已就绪，不需要 `time.Sleep` 等待；`Serve` 阻塞，必须放进独立 goroutine；`Shutdown` 之后 `Serve` 返回的 `http.ErrServerClosed` 是正常信号，用 `errors.Is` 显式放行而不是当错误上报。

三种驱动方式的取舍：

| 驱动方式 | 经过真实网络？ | 生命周期 | 适用 |
| --- | --- | --- | --- |
| `httptest.NewRecorder` + 直接调 `ServeHTTP` | 否，纯内存 | 随测随弃 | handler 单元测试，最快最确定 |
| `httptest.NewServer`（方案 A） | 是，回环端口 | httptest 托管，`defer srv.Close()` | 端到端 demo、客户端联调 |
| `net.Listener` + `http.Server`（方案 B） | 是 | 手动 Listen / Serve / Shutdown | 贴近生产的生命周期演练 |

在仓库根目录运行：

```bash
go -C labs run ./part1-golang/chapter10/http-server
```

预期输出：

```text
[A httptest]        GET /healthz -> 200 {"status":"ok"}
[B ListenAndServe]  GET /healthz -> 200 {"status":"ok"}
```

两条路径输出完全一致——这正是要点：handler 的行为不依赖于它被谁驱动，所以测试可以用最便宜的方式覆盖它。

## 10.3 net/http 如何调度连接和 Handler

**一次请求的生命周期：监听端口 → 接受连接 → 读取并解析请求 → 交给 handler → 写回响应。** 服务端有四个核心概念：**Request**（请求信息）、**Response**（响应）、**Conn**（每次请求的连接）、**Handler**（处理逻辑）。

### 每连接一个 goroutine

**`for` 循环 + `Accept` + `go c.serve()`：每个连接由独立 goroutine 服务，请求之间互不影响——这就是 Go Web 高并发的来源**，也正是第 9 章并发知识的直接应用。`ListenAndServe` 的调用链（标准库源码摘录，有删节）：

```go
func ListenAndServe(addr string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}

func (srv *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", addr) // 底层用 TCP 监听
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

func (srv *Server) Serve(l net.Listener) error {
	for {
		rw, err := l.Accept()       // 1. 接收连接
		// ...
		c := srv.newConn(rw)        // 2. 创建 Conn
		go c.serve(connCtx)         // 3. 每个连接一个 goroutine
	}
}
```

`conn.serve` 内部循环读取该连接上的请求（Keep-Alive 复用），解析后交给 handler。推论直接而重要：**handler 运行在并发环境中**，它访问的任何共享状态（全局 map、计数器、连接池）都必须自己保证并发安全，编译器和框架不会替你管。

### Handler 接口与 HandlerFunc 适配器

摘录自标准库定义：

```go
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}

type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
	f(w, r)
}
```

普通的处理函数并没有实现 `ServeHTTP`，能注册是因为 `HandlerFunc` 类型做了适配器转换：`HandlerFunc(f)` 之后函数就拥有了 `ServeHTTP` 方法。这是"接口 + 函数类型适配"的经典用法，与第 7 章的接口知识直接呼应。10.4 节的中间件之所以能写成"接收 Handler、返回 Handler"的纯函数，靠的也是这个统一接口。

### ServeMux 路由匹配

默认路由器 `DefaultServeMux` 是一个带锁的路由表（结构摘录）：

```go
type ServeMux struct {
	mu sync.RWMutex
	m  map[string]muxEntry // 路由表达式 -> handler
}

type muxEntry struct {
	explicit bool
	h        Handler
	pattern  string
}
```

分发流程：`serverHandler.ServeHTTP` 先看 `srv.Handler` 是否为空，为空则落到 `DefaultServeMux`；`mux.ServeHTTP` 调 `mux.Handler(r)` 按 URL 匹配 handler（路径不洁净时先 301 重定向到净化后的路径；带 host 的规则优先；匹配不到返回 404），再调用该 handler 的 `ServeHTTP`。`http.HandleFunc("/", f)` 做的事就是向 DefaultServeMux 的 map 注册一条规则。

Go 1.22 起 ServeMux 的模式语法显著增强：可以写 `mux.HandleFunc("GET /items/{id}", h)` 同时约束方法和路径参数，handler 里用 `r.PathValue("id")` 取值；方法不匹配时自动返回 405。本仓库 labs 使用 Go 1.26，新代码优先用这种显式模式，而不是在 handler 里手写 `r.Method` 判断（10.2 节的 `validateHealthMethod` 演示的是底层语义，理解之后应交给 mux）。

自定义路由器只需要实现 `Handler` 接口，传给 `ListenAndServe` 的第二个参数：

```go
type MyMux struct{}

func (p *MyMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		fmt.Fprintf(w, "Hello myroute!")
		return
	}
	http.NotFound(w, r)
}

func main() {
	http.ListenAndServe(":9090", &MyMux{}) // 使用自定义路由器
}
```

### 优雅关闭与服务端超时

服务端生命周期比本地程序长，**关闭时要先停止接收新连接、再等存量请求处理完**：

```go
srv := &http.Server{Addr: ":8080", Handler: NewServer()}
go func() {
	<-stop // 收到退出信号
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx) // 不再 Accept，等待存量连接结束或超时
}()
```

`Shutdown` 与 `Close` 的区别要记牢：`Shutdown` 先关闭监听器、再等待空闲连接和在途请求结束（直到 ctx 超时）；`Close` 直接强关所有连接，在途请求被丢弃。滚动发布、缩容场景用前者；只有已经出错兜底时才用后者。

长期运行的服务还应显式设置超时字段，否则慢客户端可以无限占用连接：

| `http.Server` 字段 | 管什么 | 不设的风险 |
| --- | --- | --- |
| `ReadHeaderTimeout` | 读完请求头的时限 | slow-header 攻击：客户端逐字节发头，耗尽连接 |
| `ReadTimeout` | 读完整个请求（含 body）的时限 | 慢 body 上传占住连接 |
| `WriteTimeout` | 写完响应的时限 | 慢客户端接收拖住 handler goroutine |
| `IdleTimeout` | Keep-Alive 空闲连接的保留时长 | 空闲连接堆积占用文件描述符 |

方案 B 里那行 `ReadHeaderTimeout: 2 * time.Second` 不是装饰——它是防 slow-header 这类低级 DoS 的最低配置。

## 10.4 中间件组织服务端策略，客户端显式控制失败

### 中间件：洋葱模型

**中间件就是一个接收 Handler、返回 Handler 的函数。** [http-middleware-client](../../../labs/part1-golang/chapter10/http-middleware-client) 把这个形状固定成类型，摘录自 [middleware.go](../../../labs/part1-golang/chapter10/http-middleware-client/middleware.go)：

```go
// Middleware decorates an http.Handler and returns the wrapped handler.
// Onion model: the first declared middleware is the outermost layer.
type Middleware func(http.Handler) http.Handler
```

组合器按声明顺序包裹，摘录自同一文件：

```go
func (c Chain) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.NotFoundHandler()
	}
	for i := len(c.mws) - 1; i >= 0; i-- {
		if c.mws[i] == nil {
			continue
		}
		h = c.mws[i](h)
	}
	return h
}
```

循环从尾部向前包，所以 `NewChain(a, b, c).Then(h)` 等价于 `a(b(c(h)))`——**声明在最前的中间件位于最外层**，请求先经过它，响应最后经过它。像洋葱：外层先收到请求、后看到响应。

日志、recover、超时这类横切逻辑不该写进每个 handler，而是各包一层。示例的三件套，摘录自 [middleware.go](../../../labs/part1-golang/chapter10/http-middleware-client/middleware.go)：

```go
// Recover returns a Middleware that recovers from panics in the handler
// goroutine, writes HTTP 500, and logs the panic value plus stack trace.
func Recover(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("panic", recovered),
						slog.String("stack", string(debug.Stack())),
					)
					http.Error(w, "internal error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout returns a Middleware that bounds each request to d.
// On timeout the wrapped handler is abandoned and 503 msg is written.
func Timeout(d time.Duration, msg string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, msg)
	}
}
```

为什么示例的装配顺序是 `NewChain(Logging, Timeout, Recover)`？

- **Logging 最外**：它要记录真实的状态码和耗时，必须在所有改写响应的层之外观察。
- **Timeout 居中**：`http.TimeoutHandler` 到点会接管响应写出 503，被它包住的 handler 实际上在另一个 goroutine 里被"放弃"——所以超时被放弃后仍在跑的 handler 不能再写响应（会 panic 或写不出去），业务代码要主动监听 `r.Context().Done()`。
- **Recover 贴着业务 handler**：panic 发生在最内层，recover 必须在它和外层之间兜底；若放在 Timeout 之外，内层 panic 会先穿过 TimeoutHandler，行为就复杂了。

`Logging` 里还有个细节值得知道：它用 `statusRecorder` 包装 `ResponseWriter` 来观察状态码，因为接口本身不提供"读回状态码"的能力。摘录里 `Write` 的实现揭示了 net/http 的一条规则：

```go
// Write forwards to the underlying ResponseWriter and counts written bytes.
// If WriteHeader was not called explicitly, it is implicitly 200.
func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}
```

**第一次 `Write` 会隐式写出 200**，之后再调 `WriteHeader` 或改 Header 都无效。这就是"先写 body 再设状态码，状态码回不去"这个经典坑的机制根源（见踩坑清单）。

### 客户端：超时是底线，重试是策略

**超时和重试属于调用边界，不属于业务代码。** 摘录自 [http-middleware-client/main.go](../../../labs/part1-golang/chapter10/http-middleware-client/main.go)：

```go
client := &http.Client{
	Timeout: 2 * time.Second, // 先把基础超时定下来。
}

req = req.WithContext(ctx) // 再把请求级取消和 trace 一起透传下去。
resp, err := client.Do(req)
```

客户端边界的三条纪律：

- **必须显式设置超时**：`http.DefaultClient` 没有超时，下游挂住时调用方会永远等待。
- **必须关闭响应 Body**：`resp.Body.Close()` 不执行会导致连接无法复用、资源泄漏；重试旧响应前尤其要先释放它。
- **重试要有上限、退避和幂等性约束**：GET 通常可重试，POST 不能仅因遇到 500 就无条件重放；有请求体时要确认它能被重新读取。超时和重试共同消耗总调用预算，调大重试次数可能抬高尾部延迟，并不保证成功率提高。

❌ 与 ✅ 对照：

```go
// ❌ 默认 client 没有超时，下游挂住时 goroutine 永远阻塞
resp, err := http.DefaultClient.Get(url)

// ❌ 不分方法、不分错误类型地无条件重试
for i := 0; i < 3; i++ {
	resp, err = client.Do(req) // POST 重复扣款的风险就在这里
}

// ✅ 显式超时 + ctx 透传 + 只对幂等方法的特定失败重试
client := &http.Client{Timeout: 2 * time.Second}
resp, err := retryClient.DoWithRetry(ctx, req)
```

示例的重试客户端把"什么时候值得重试"收敛成一个谓词，摘录自 [client.go](../../../labs/part1-golang/chapter10/http-middleware-client/client.go)：

```go
// DefaultIsRetryable returns true when the request method is idempotent
// (GET / HEAD / PUT / DELETE) and either the transport failed or the server
// responded with 5xx (or 429).
func DefaultIsRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusTooManyRequests ||
		(resp.StatusCode >= http.StatusInternalServerError && resp.StatusCode <= 599)
}

// isIdempotent reports whether HTTP method m is safe to replay.
func isIdempotent(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}
```

注意语义分类在这里落地了：传输层错误（连接被拒、TLS 失败，响应根本没发出）可以放心重试；5XX 是服务端明确说自己出错，重试可能成功；4XX 是请求本身有问题，重发一百次也是同样的 4XX。这正是 10.1 节状态码分类的工程用途。

`RetryPolicy` 的字段决定重试节奏（字段语义摘录自 client.go 的注释）：

| 字段 | 含义 | 要点 |
| --- | --- | --- |
| `MaxAttempts` | 含首次在内的总尝试次数 | ≤ 1 表示不重试 |
| `BaseDelay` | 指数退避的基数 | 第 n 次退避约 `BaseDelay * 2^(n-1)` |
| `MaxDelay` | 单次退避的上限 | 防止指数退避失控 |
| `Jitter` | 随机抖动的上界 | 避免大量客户端同步重试形成惊群 |
| `IsRetryable` | 自定义重试谓词 | nil 时用 `DefaultIsRetryable` |

主循环摘录自 [client.go](../../../labs/part1-golang/chapter10/http-middleware-client/client.go)（有删节）：

```go
	methodRetryable := isIdempotent(req.Method)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		attemptReq := req.Clone(ctx)
		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("reset request body: %w", err)
			}
			attemptReq.Body = body
		}

		resp, err := httpClient.Do(attemptReq)
		if err != nil {
			lastErr = err
		}

		shouldRetry := methodRetryable && attempt < maxAttempts && isRetryable(resp, err)
		if !shouldRetry {
			if err != nil {
				return resp, fmt.Errorf("attempt %d: %w", attempt, err)
			}
			return resp, nil
		}

		drainAndClose(resp) // 重试前排空并关闭旧响应体，连接才能复用
		delay := retryDelay(policy, attempt, resp, c.now)
		// ... 记录日志后 sleep(ctx, delay)；ctx 取消会立刻中断退避
	}
```

三个容易漏掉的细节：每次重试前检查 `ctx.Err()`，取消能中断退避等待而不只是中断请求；带 body 的请求靠 `req.GetBody` 重新生成请求体，没有 `GetBody` 的 body 重试时已是空流；旧响应先 `drainAndClose`——读空再关闭，底层连接才能归池复用。此外 `retryDelay` 会优先遵守服务端返回的 `Retry-After` 头（秒粒度），服务端主动告知的等待时间比本地退避更可信。

运行示例可以看到四种边界各就各位：

```bash
go -C labs run ./part1-golang/chapter10/http-middleware-client
```

预期输出：

```text
demo=echo         status=200 body={"echo":"ok"}   attempts=1
demo=recover      status=500 body=internal error  attempts=1
demo=timeout      status=503 body=timeout         attempts=1
demo=retry-then-ok status=200 body={"echo":"ok"}  attempts=3
```

`recover` 分支演示中间件把 panic 兜成 500；`timeout` 分支演示 50ms 请求超时切断挂死 handler；`retry-then-ok` 分支演示前两次 503 后第三次成功、`attempts=3` 的完整重试链路。

取消 Context 可以通知下游停止工作，但不会自动替它执行清理，也不会自动把追踪字段放进 HTTP header。

## 10.5 database/sql 管连接池，repository 隔离存储实现

**Go 不内置任何数据库驱动，而是定义 `database/sql` 标准接口，第三方驱动按接口实现——按标准接口写的代码，迁移数据库时无需修改。**

### 驱动注册与连接

驱动在 `init` 函数中调用 `sql.Register` 注册自己，因此用匿名导入激活：

```go
import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql" // _ 表示只用其 init 副作用：注册 "mysql" 驱动
)
```

`sql.Open` 的第一个参数是注册时的驱动名，第二个是 DSN（数据源名称）：

```text
user:password@tcp(localhost:3306)/dbname?charset=utf8
user:password@/dbname                 // 省略协议和地址时用默认
```

- `sql.Open` 只校验参数、**并不真正建立连接**；`db.Ping()` 才验证连通性。
- `sql.DB` 内部维护连接池（空闲连接复用），是并发安全的——全局共享一个实例，不要每次请求都 Open/Close，也不要自己缓存底层 conn。
- 池子行为用三个方法调：`db.SetMaxOpenConns(n)` 限制同时打开的连接总数（默认无限，打满数据库的常见原因）、`db.SetMaxIdleConns(n)` 控制保留的空闲连接数、`db.SetConnMaxLifetime(d)` 强制回收存活过久的连接（MySQL 服务端有自己的 wait_timeout，客户端不回收就会拿到"已死"连接）。

### 增删改查骨架

下面的 CRUD 骨架需要真实 MySQL 实例才能运行：

```go
db, err := sql.Open("mysql", "user:password@/test?charset=utf8")
if err != nil {
	log.Fatal(err)
}
defer db.Close()

// 插入：Prepare + Exec，参数用 ? 占位（可一定程度防止 SQL 注入）
stmt, err := db.Prepare("INSERT INTO userinfo SET username=?,department=?,created=?")
// ...
res, err := stmt.Exec("astaxie", "研发部门", "2012-12-09")
id, err := res.LastInsertId() // 自增 ID

// 更新
stmt, err = db.Prepare("UPDATE userinfo SET username=? WHERE uid=?")
res, err = stmt.Exec("newname", id)
affect, err := res.RowsAffected() // 影响行数

// 查询：Query + rows.Next 循环 + Scan 按列绑定
rows, err := db.Query("SELECT uid,username,department,created FROM userinfo")
// ...
defer rows.Close() // 必须关闭，否则占用连接
for rows.Next() {
	var uid int
	var username, department, created string
	if err := rows.Scan(&uid, &username, &department, &created); err != nil {
		log.Fatal(err)
	}
}

// 删除
stmt, err = db.Prepare("DELETE FROM userinfo WHERE uid=?")
res, err = stmt.Exec(id)
```

要点：增删改用 `Prepare` + `Exec`，配合 `LastInsertId()`（自增 ID）和 `RowsAffected()`（影响行数）；查询用 `Query` + `rows.Next()` + `rows.Scan(&...)` 按列顺序绑定；`rows` 和 `stmt` 用完都要关闭以释放连接。事务用 `db.Begin()` 拿到 `Tx`，之后 `tx.Commit()` 或 `tx.Rollback()`——事务边界是后端部分专章的内容。

三个入口方法的选择：

| 方法 | 返回 | 什么时候用 |
| --- | --- | --- |
| `Exec` | `sql.Result`（LastInsertId / RowsAffected） | INSERT / UPDATE / DELETE，不要结果集 |
| `Query` | `*sql.Rows`，需 `Next` + `Scan` + `Close` | 返回多行的 SELECT |
| `QueryRow` | `*sql.Row`，`Scan` 自带关闭 | 确定至多一行的查询，最不易漏关 |

另外提醒一个语义边界：`?` 占位符让驱动把参数当数据传而不是拼进 SQL 文本，能防住典型的注入，但"一定程度"不是"绝对"——表名、列名、ORDER BY 字段这类无法参数化的位置，拼字符串前必须走白名单校验。

### repository：先抽接口，再接实现

**HTTP 层不直接依赖具体的数据库实现。** 本章沿用 repository 模式划定边界，摘录自 [repository-pattern/main.go](../../../labs/part2-backend/chapter12/repository-pattern/main.go)：

```go
type ProductRepository interface {
	Create(ctx context.Context, product Product) (Product, error)
	FindBySKU(ctx context.Context, sku string) (Product, error)
	List(ctx context.Context, filter ProductFilter) ([]Product, error)
	UpdateStock(ctx context.Context, sku string, delta int) (Product, error)
}

type CatalogService struct {
	repo ProductRepository
}

func NewCatalogService(repo ProductRepository) CatalogService {
	return CatalogService{repo: repo}
}
```

注意接口方法全部接收 `ctx context.Context` 作为第一个参数——这不是巧合，而是让 HTTP 请求的超时与取消能一路穿透到数据库调用：请求方断开时，进行中的查询也跟着取消，不再白占连接。service 只依赖接口，测试注入内存实现（`InMemoryProductRepository`）就能验证查无记录返回 `ErrNotFound`、库存不为负等业务规则，完全不需要 MySQL。

运行 `go -C labs run ./part2-backend/chapter12/repository-pattern` 可以看到 service 与 repository 解耦的效果。[gorm-mysql](../../../labs/part2-backend/chapter12/gorm-mysql) 示例只有模型、配置与脱敏 DSN，没有连接 MySQL，也没有真实 GORM 查询；它的价值在于演示模型标签与连接参数的装配，摘录自 [main.go](../../../labs/part2-backend/chapter12/gorm-mysql/main.go)：

```go
type Product struct {
	ID         uint       `gorm:"primaryKey"`
	SKU        string     `gorm:"size:64;uniqueIndex;not null"`
	Name       string     `gorm:"size:128;not null"`
	PriceCents int64      `gorm:"not null;check:price_cents >= 0"`
	Active     bool       `gorm:"not null;default:true"`
	CreatedAt  time.Time  `gorm:"not null"`
	UpdatedAt  time.Time  `gorm:"not null"`
	DeletedAt  *time.Time `gorm:"index"`
}
```

注意 `DeletedAt` 指针字段本身不等于启用 GORM 软删除，它只是模型上的一个可空列。repository-pattern 用内存实现隔离业务调用，测试能证明该实现的行为，不能据此证明数据库约束、查询计划或事务隔离——内存 map 里没有唯一索引、没有锁、没有隔离级别，这些约束只有在真实数据库上才存在。真实 SQL、索引与 GORM 的深入内容在第 12 章展开。

## 踩坑清单

### 直接依赖 `http.DefaultClient`，没显式超时

```go
func CallDownstream(url string) (*http.Response, error) {
	// 反例：默认 client 没有合适的请求超时，挂住时很难收尾。
	return http.DefaultClient.Get(url)
}
```

现象：下游挂住时调用方永远等待。原因：`http.DefaultClient` 的 `Timeout` 为零值，表示永不超时。修复：自建 `http.Client` 并设置 `Timeout`，同时传递请求 Context、关闭响应体；用 httptest 构造慢服务复验，在限定时间内得到错误，测试不依赖公网。

### handler、路由和启动逻辑混成一坨

```go
func main() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// 反例：匿名函数里混了业务、路由和进程启动，测试边界很差。
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

现象：测试必须启动长期服务，边界很差。原因：路由注册、业务处理和进程生命周期三件事挤在一处，无法单独实例化。修复：路由组装留在 `NewServer`，处理函数作为 handler，监听端口交给入口；之后用 httptest 直接检查 GET/HEAD 与非法方法，无需启动长期服务。

### 先 Write 再改状态码：状态码改不回去了

```go
func ProductHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "not found")        // 反例：第一次 Write 已隐式写出 200
	w.WriteHeader(http.StatusNotFound) // 这行无效，且日志会报 superfluous WriteHeader
}
```

现象：想返回 404，客户端收到的却是 200。原因：`ResponseWriter.Write` 第一次调用会隐式 `WriteHeader(200)`，之后的状态码与 Header 修改全部无效（10.4 节 `statusRecorder` 的注释写明了这条规则）。修复：固定顺序——先 `Header().Set`，再 `WriteHeader`，最后 `Write`。

### rows / resp.Body 没关：连接池悄悄耗尽

```go
rows, _ := db.Query("SELECT ...")
// 反例：没有 defer rows.Close()，每次查询都占住一个池连接
```

现象：服务跑一段时间后新请求全部卡住，数据库侧看到连接数打满。原因：未关闭的 `rows` / `resp.Body` 一直占用底层连接，连接无法归还连接池。修复：`defer rows.Close()` 紧跟查询；客户端重试前像 `drainAndClose` 那样先读空再关闭旧响应体。用 `QueryRow` 替代 `Query` 处理单行查询可以从根上消除一类漏关。

### 还没抽 repository，就直接把 ORM 调进 handler

```go
func ProductHandler(w http.ResponseWriter, r *http.Request) {
	// 反例：HTTP 层直接依赖数据库实现，后面 service 边界很难补。
	var p Product
	_ = db.Where("sku = ?", "SKU-101").First(&p).Error
}
```

现象：HTTP 层锁死具体数据库实现，service 边界难补。修复：把查找动作放到 `ProductRepository` 实现中，handler 或 service 只依赖接口；用内存仓库验证查无记录和错误传播（这不等于验证了真实 SQL）。

已有测试通过只说明已覆盖的用例符合预期；修复中新增的边界仍要补充断言。

<a id="练习"></a>

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 10.1 新增就绪路由 | 在 `NewServer` 中注册 `/readyz`：校验方法、设置 JSON Content-Type、编码 `{"status":"ready"}`，并用 httptest 补测试 | GET /readyz 返回 200 和 status=ready；原健康检查保持不变；HEAD 请求有头无 body，POST 返回 405 | `http-server/main.go`、`main_test.go` |
| 10.2 对比超时与重试 | 把超时改为 100ms、尝试次数改为 3，复用 httptest 分别制造慢响应和先失败后成功 | 能区分总调用超时和尝试次数变化；断言尝试次数而非只看耗时，取消后不继续退避 | `http-middleware-client/main.go`、`client_test.go` |

完成后运行受影响示例及测试：

```bash
go -C labs test ./part2-backend/chapter12/gorm-mysql/... -count=1
go -C labs test ./part1-golang/chapter10/http-middleware-client/... -count=1
go -C labs test ./part1-golang/chapter10/http-server/... -count=1
go -C labs test ./part2-backend/chapter12/repository-pattern/... -count=1
```

更多练习见各 lab 的 README。

## 小结

- Web 排错先定位 DNS、连接、HTTP 事务还是页面渲染；不同层的失败不能用同一条重试策略处理。
- Handler 只处理请求，装配函数负责路由和依赖，启动函数负责监听与关闭；职责拆开后才能直接测试协议行为。
- 服务端必须设置读写与空闲超时，客户端必须有总超时；重试只能用于满足幂等条件且仍有剩余预算的失败。
- Header、状态码和响应体有固定写入顺序；响应体与数据库 rows 必须关闭，底层连接才能回到连接池。
- repository 隔离的是存储能力与失败语义。内存实现通过只能证明接口契约，不能替代真实 SQL、事务和连接池验证。

## 链接

- [Go 标准库：net/http](https://pkg.go.dev/net/http)：客户端超时、响应体关闭和 Server.Shutdown 行为的官方说明。
- [Go 标准库：net/http/httptest](https://pkg.go.dev/net/http/httptest)：`NewRecorder` 与 `NewServer` 两种测试驱动方式的官方文档。
- [Go 标准库：database/sql](https://pkg.go.dev/database/sql)：连接池方法（`SetMaxOpenConns` 等）与 `Query`/`Exec`/`QueryRow` 语义。
- [Go Web 编程（astaxie）](https://github.com/astaxie/build-web-application-with-golang/blob/master/zh/preface.md)：本章 10.1–10.3 与 10.5 的主要参考，包括 [Web 工作方式](https://github.com/astaxie/build-web-application-with-golang/blob/master/zh/03.1.md)、[搭建简单 Web 服务](https://github.com/astaxie/build-web-application-with-golang/blob/master/zh/03.2.md)、[Go 如何使得 Web 工作](https://github.com/astaxie/build-web-application-with-golang/blob/master/zh/03.3.md)、[http 包详解](https://github.com/astaxie/build-web-application-with-golang/blob/master/zh/03.4.md)、[database/sql 接口](https://github.com/astaxie/build-web-application-with-golang/blob/master/zh/05.1.md)、[使用 MySQL 数据库](https://github.com/astaxie/build-web-application-with-golang/blob/master/zh/05.2.md)。
- [TCP 服务器](https://learnku.com/docs/the-way-to-go/151-tcp-server/3703)、[一个简单的 Web 服务器](https://learnku.com/docs/the-way-to-go/152-a-simple-web-server/3704)：用 net 包和 http 包各写一个服务器的补充读物。

[返回全书目录](../README.md) · [上一篇：并发编程](chapter09.md) · [下一篇：工程实践与代码规范](chapter11.md)
