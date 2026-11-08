---
pageId: "ch09-concurrency"
title: "第 9 章：并发编程"
part: "Part 1 · Go 基础"
unitNumber: 9
order: 18
description: "内存模型、channel、sync 原语、context 与 GMP 调度。"
published: true
source: "book/part1-golang/chapter09.md"
labs: ["labs/part1-golang/chapter09/goroutine-channel-select","labs/part1-golang/chapter09/worker-pools"]
capabilities: ["goroutine","channel","sync","context"]
---
Go 的并发难不在启动——`go` 一个关键字就够——而在正确收尾：每个共享数据的决定都要回答"谁看得到这次写入、谁先结束"，答不上来就是内存可见性错误、goroutine 泄漏或数据竞争。

先用一个场景感受一下代价。HTTP 服务里每个请求派生 5 个 goroutine 分别查缓存、数据库和下游 API（第 10 章的"每连接一个 goroutine"模型）。如果其中一个 goroutine 卡在无人接收的 channel 发送上，它不会被 GC 回收、不会被超时救回，就这么一直挂着：每秒 200 个请求、每请求泄漏 1 个，一小时后进程里堆着 72 万个休眠 goroutine 和几 GB 的栈内存，除了 pprof 里 goroutine 数量单调上涨，没有任何报错。前面八章的程序都跑在一条主 goroutine 上，不存在这个问题；本章的所有原语——channel、select、sync、context——说到底都是为同一件事服务：**让每个 goroutine 有明确的退出条件，让每次共享写入有明确的可见性保证**。

## 为什么并发是 Go 的核心能力

**并发（concurrency）是"同时应付多件事"的结构设计，并行（parallelism）是"同时执行多件事"的运行状态**：单核靠快速切换推进并发，多核才可能真正并行。goroutine 是 Go 运行时管理的轻量级执行单元，初始栈只有几 KB 且可按需增长，创建和切换的开销远小于操作系统线程，一个进程轻松承载成千上万个：

```go
go func() {
	fmt.Println("在新的 goroutine 中执行")
}()
```

熟悉 Python asyncio 的读者注意一个本质差异：asyncio 是**协作式**调度，协程必须在 `await` 处主动让出，一个忘记 await 的阻塞调用就冻住整个事件循环，而且 `async`/`await` 会"染色"整条调用链；goroutine 由运行时**抢占式**调度，普通的同步代码天然就是并发代码，不需要任何关键字标记。这也是为什么 Go 的网络库写法比异步框架直白得多。

### GMP：goroutine 为什么便宜

运行时负责把大量 goroutine 调度到少量操作系统线程上执行，这就是 GMP 模型：

- **G（goroutine）**：待执行的并发任务，包含栈、指令指针和上下文，创建销毁都很廉价。
- **M（machine）**：操作系统线程，真正在 CPU 上跑代码的执行体。
- **P（processor）**：调度上下文，数量默认等于 CPU 核数（`GOMAXPROCS`）。每个 P 维护一个本地 G 队列；M 必须绑定 P 才能执行 G。

两个关键机制：**work stealing**——某个 P 的队列空了，就从其他 P 的队列尾部"偷"一半 G 过来，保持所有核都有活干；**sysmon 与抢占**——运行时会抢占运行过久的 G，防止一个死循环独占线程。channel 阻塞、系统调用时，G 会让出 M，其他 G 接着用——所以"每连接一个 goroutine"的 Web 模型（第 10 章）才成立。

- 实践含义：worker pool 的 worker 数量超过 `GOMAXPROCS` 并不会让 CPU 计算更快（只增加切换），但 I/O 密集型任务可以开得多得多；`runtime.GOMAXPROCS(0)` 可查询当前值。

调度的细节由运行时处理，但**数据在多 goroutine 间的可见性必须由程序自己保证**——这就是内存模型要回答的问题。

## 9.1 Go 内存模型：什么时候一个 goroutine 能看到另一个的写入

**Go 内存模型是内存一致性模型，核心概念是 happens before**：e1 happens before e2，则 e1 的结果对 e2 可见；没有 happens-before 关系的两个操作并发发生，互相看不到对方的中间状态。

- happens-before 具有传导性：e3 → e1 → e2，则 e3 → e2。
- 单个 goroutine 内部，即使编译器和 CPU 发生了指令重排，Go 也保证程序行为符合代码顺序；但**多 goroutine 共享数据时 Go 不做任何保证**。

经典反例：

```go
var a, b int

// goroutine A          // goroutine B
a = 1                   // if b == 2 {
b = 2                   //     print(a) // 可能打印 0！
                        // }
```

goroutine B 观察到 `b == 2` 并不意味着它能看到 `a == 1`：没有同步手段时，编译器或 CPU 可能让 b 的写入先对 B 可见。

Go 明确保证 happens-before 的场景：

| 场景 | 保证 |
| --- | --- |
| 包初始化 | 被导入包的 init 先于导入方执行；所有 init 先于 main.main |
| goroutine 创建 | `go f()` 语句先于 f 开始执行（创建前的写入对 f 可见） |
| 缓冲 channel | 第 n 次发送先于第 n 次接收完成 |
| 无缓冲 channel | 接收先于发送完成 |
| 互斥锁 | 第 n 次 `Unlock` 先于之后的 `Lock` 返回 |

注意反方向不成立：**goroutine 的完成不保证先于任何代码**。想等待 goroutine 结束，必须用 channel 或 WaitGroup 显式同步，而不是"睡一会儿"：

```go
// ❌ 错误：用睡眠"等待" goroutine 写完
var done bool
go func() { done = true }()
time.Sleep(time.Second) // 睡够久通常"碰巧"能看到，但没有 happens-before 保证
fmt.Println(done)       // 严格意义上这是数据竞争，-race 会报告

// ✅ 正确：用 channel 建立 happens-before
done := make(chan struct{})
go func() { close(done) }()
<-done // 内存模型保证：close 先于因通道关闭而返回的接收操作
```

`time.Sleep` 版本在开发机上几乎总能"通过"，这正是它危险的地方：它依赖的是调度时机而非语言保证，换台机器、换个负载就露出竞争。

## 9.2 channel：goroutine 间的通信管道

**Go 的并发哲学是"不要通过共享内存来通信，而要通过通信来共享内存"，channel 是它的落地工具**：goroutine 之间并发安全的数据通道，先进先出，相当于一个带同步语义的队列。

```go
ch := make(chan string)   // 无缓冲通道
buf := make(chan int, 3)  // 容量为 3 的缓冲通道

go func() { ch <- "hello" }() // 发送
msg := <-ch                    // 接收
_ = msg

buf <- 1                       // 缓冲未满时不阻塞
n := len(buf)                  // len 返回队列中元素个数，cap 返回容量
_ = n
```

无缓冲通道的发送和接收是同步的：发送方阻塞到有人接收，接收方阻塞到有人发送。缓冲通道在队列未满时发送不阻塞、队列非空时接收不阻塞。选择哪种取决于你要"交接"还是"排队"：无缓冲通道强制收发双方同时到场，缓冲通道允许生产者短暂领先。缓冲容量同时是**背压阈值**——队列满了生产者自然被拖慢，不会无限积压。

| 需求 | 选择 | 理由 |
| --- | --- | --- |
| 严格交接，收发双方必须同时到场 | 无缓冲 | 发送成功即证明接收方已拿到，天然建立 happens-before |
| 生产者短暂领先、削峰 | 缓冲 | 队列吸收突发，容量是显式的背压阈值 |
| 每个 worker 只发一个结果就退出 | 容量 = 结果数的缓冲 | 发送永不阻塞，worker 不必等接收方到场 |
| select 里临时禁用某个分支 | 把该 case 的通道置 nil | nil 通道收发永远阻塞，等于摘掉该分支 |

"每个 worker 只发一个结果"是最常见的缓冲通道用法。摘录自 [goroutine-channel-select/main.go](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/blob/main/labs/part1-golang/chapter09/goroutine-channel-select/main.go) 的 `startWorker`：

```go
func startWorker(source, value string, delay time.Duration) <-chan result {
	ch := make(chan result, 1)
	go func() {
		time.Sleep(delay)
		ch <- result{source: source, value: value}
		close(ch)
	}()
	return ch
}
```

逐行看这个只有七行的函数，它同时演示了三个约定：

- `make(chan result, 1)`：容量 1 是关键。worker 发完结果立刻退出，不必等接收方到场——即使调用方超时放弃接收，goroutine 也不会泄漏。
- 返回类型是 `<-chan result`（只读通道）：调用方在编译期就无法向它发送或关闭它，"谁创建谁关闭"由类型系统强制执行。
- goroutine 内部负责 `close(ch)`：它是这个通道唯一的发送方，符合"由发送方关闭"的规则；接收方还能用 `range` 感知结果流结束。

### 关闭与遍历

```go
close(ch)              // 关闭通道：表示不再发送
v, ok := <-ch          // ok 为 false 表示通道已关闭且无剩余数据
for v := range buf {   // 持续接收，直到通道关闭且数据取完
	_ = v
}
```

关闭的规则必须记牢：

- **由发送方关闭**，接收方不关闭通道；**只关闭一次**，重复关闭会 panic。
- 向已关闭的通道发送会 panic；从已关闭的通道接收不阻塞，立即返回剩余数据或零值。
- 有多个发送方时，不能由其中某一个关闭，要由掌握全部发送者生命周期的一方（例如等待所有 worker 退出的协调者）关闭。

三种状态下的行为汇总：

| 操作 | nil 通道 | 已关闭通道 | 活跃通道 |
| --- | --- | --- | --- |
| `close(ch)` | panic | panic | 成功关闭 |
| `ch <- v` | 永远阻塞 | panic | 成功发送或阻塞 |
| `v, ok := <-ch` | 永远阻塞 | 不阻塞（取剩余值或零值） | 成功接收或阻塞 |

对 nil 通道的收发会永远阻塞——这既是 goroutine 泄漏的常见来源，也可以在 select 里被有意识地利用（把某个 case 的通道置 nil 等于暂时禁用该分支）。

### 底层结构一瞥

channel 由运行时的 `hchan` 结构体实现（来自 Go 源码分析，帮助理解行为，不要求记忆）：

```go
// 源码摘录（runtime/chan.go，有删节）：hchan 的核心字段。
type hchan struct {
	qcount   uint   // 队列中元素个数
	dataqsiz uint   // 循环队列的大小（缓冲容量）
	buf      unsafe.Pointer // 指向循环队列
	closed   uint32 // 关闭标志
	elemtype *_type // 元素类型
	sendx    uint   // 待发送索引（队尾）
	recvx    uint   // 待接收索引（队头）
	recvq    waitq  // 等待接收的 goroutine 队列
	sendq    waitq  // 等待发送的 goroutine 队列
	lock     mutex  // 保护整个通道的互斥锁
}
```

- 缓冲是**循环队列**：`sendx`/`recvx` 分别是队尾、队头指针，走到末尾回绕；用 `qcount` 记录长度来区分队空和队满，`len(ch)` 直接返回 `qcount`。
- 阻塞的 goroutine 被包装成 `sudog` 挂入 `recvq`/`sendq` 双向链表，进入休眠（gopark），让出线程；当发送方发现 `recvq` 里有等待的接收者时，会**把数据直接复制到接收者的栈上**并唤醒它，省掉一次队列中转——无缓冲通道的"交接"语义由此而来。关闭通道时，运行时唤醒所有等待中的收发双方：接收者拿到零值和 `ok=false`，发送者 panic。

### select：多路等待

**select 让"谁先就绪就用谁"变成显式逻辑**。最典型的场景是"快路径与慢路径竞争"：缓存和数据库同时查，谁先回来用谁，但整个等待必须有上限。摘录自 [goroutine-channel-select/main.go](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/blob/main/labs/part1-golang/chapter09/goroutine-channel-select/main.go) 的 `firstResult`：

```go
func firstResult(primary, fallback <-chan result, timeout time.Duration) (result, bool) {
	select {
	case got := <-primary:
		return got, true
	case got := <-fallback:
		return got, true
	case <-time.After(timeout):
		return result{}, false
	}
}
```

逐行看：

- `primary`、`fallback` 任一到货即返回，并带 `ok=true`——两个分支地位平等，写在前面的没有优先级。
- `time.After(timeout)` 分支把等待变成有上限的等待：到期后返回零值和 `ok=false`。
- 返回值用 `(result, bool)` 而不是裸 `result`，把"超时"和"查到一个零值结果"区分开——这正是本章反复强调的语义分类：调用失败与结果为空是两回事。
- 超时只结束**等待**，不会终止 primary/fallback 背后的 worker。它们能安全收尾，靠的正是 `startWorker` 里容量为 1 的缓冲通道：没人接收也能发送完毕、自行退出。如果改成无缓冲通道，超时路径每触发一次就泄漏一个 goroutine。

select 的另外两条边界：

- 多个 case 同时就绪时**伪随机选择**，不保证写在前面的分支优先；测试不能依赖选择顺序。
- `default` 分支让 select 变为非阻塞，适合做"有就取、没有就跳过"的轮询；但轮询会空转烧 CPU，长期等待应该用阻塞 select 或 `time.Timer`。

### fan-in：多个发送方时谁来关闭

前面规则说"多个发送方时由掌握全部生命周期的一方关闭"，fan-in 就是这条规则的标准实现。摘录自 [goroutine-channel-select/main.go](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/blob/main/labs/part1-golang/chapter09/goroutine-channel-select/main.go) 的 `fanIn`：

```go
func fanIn(inputs ...<-chan result) <-chan result {
	out := make(chan result)
	var wg sync.WaitGroup
	wg.Add(len(inputs))

	for _, input := range inputs {
		go func(ch <-chan result) {
			defer wg.Done()
			for got := range ch {
				out <- got
			}
		}(input)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
```

逐行看：

- 每个输入通道配一个转发 goroutine，`for got := range ch` 表示它活到输入被关闭且取空为止——退出条件是输入方给的，不需要计数。
- `wg.Add(len(inputs))` 在启动任何 goroutine **之前**一次完成，这是 WaitGroup 的安全用法（反例见踩坑清单）。
- 专门的协调 goroutine 等 `wg.Wait()` 返回后 `close(out)`：它是唯一掌握"所有发送方都已结束"这一事实的角色，因此只有它有资格关闭 `out`。
- `out` 是无缓冲的：转发 goroutine 阻塞到接收方取走数据，形成天然背压；接收方慢，整个管道就慢，不会隐性积压。
- 匿名函数把 `input` 作为参数传入而不是直接引用循环变量——Go 1.22 之前这是防止所有闭包共享同一循环变量的必要写法（见第 4 章闭包一节），1.22 起语言已修复，显式传参让依赖关系一目了然，仍是好习惯。

配套的 `collectAll` 只是 `for got := range fanIn(inputs...)` 的收集循环，演示了接收方如何仅靠 `range` 感知全部发送方结束，不需要知道有几个输入。

## 9.3 互斥锁、读写锁与条件变量

channel 适合"传递数据"；**保护一小段共享状态的读写，用锁往往更直接**。先把选择问题摆出来：

| 场景 | 首选 | 理由 |
| --- | --- | --- |
| 传递数据、编排生命周期（谁退出、谁关闭） | channel | 所有权随数据移动，竞争难以发生 |
| 保护一小段共享状态（计数器、配置、缓存） | `sync.Mutex` | 临界区够短时最直接、最好读 |
| 读远多于写的共享状态 | `sync.RWMutex` | 多读并发，写独占 |
| 简单计数器、标志位 | `sync/atomic` | 无锁、最轻，但一次只能动一个机器字 |
| key 只写一次读多次的缓存、互不相交的键集合 | `sync.Map` | 读路径无锁 |
| 等待某个条件成立 | channel 优先；`sync.Cond` 用于底层结构 | Cond 的 for+Wait 模式容易写错 |

经验法则和 Go 官方一致：**能用 channel 说清楚生命周期的就用 channel；锁只用来保护"状态本身"，不用来编排流程**。

### sync.Mutex

```go
// 用互斥锁保护共享计数器。
type counter struct {
	mu sync.Mutex
	n  int
}

func (c *counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}
```

注意这个写法的层次：`counter` 不把 `n` 暴露给调用方，只暴露带锁的 `Inc` 方法——**共享状态只通过"自带锁的操作"访问**，锁的范围就不可能出错。worker-pools lab 的 `poolStats`（9.8 节）用的正是这个模式。

- **Lock/Unlock 必须成对**，推荐 `defer` 配对；只加锁不解锁会让其他 goroutine 永久阻塞，多解一次锁会 panic（`sync: unlock of unlocked mutex`）。
- **禁止复制锁**：含锁的结构体必须传指针，复制会连同锁状态一起拷贝，之后 Unlock 的是副本而非原锁，造成死锁；`go vet` 能检测这类错误。**不可重入**：同一 goroutine 连续两次 Lock 会死锁；持有读锁时再申请写锁同样形成循环等待。锁的粒度宁小勿大：只保护共享状态本身，不要在持锁期间做 I/O 或调用可能阻塞的函数。

粒度的 ✅/❌ 对照：

```go
// ❌ 持锁做 I/O：网络耗时期间锁一直被持有，所有读者排队
func (c *config) Fetch(k, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, _ := http.Get(url) // 慢操作把锁拖长，并发度归零
	c.data[k] = readBody(resp)
}

// ✅ 锁只包住共享状态本身
func (c *config) Fetch(k, url string) {
	resp, _ := http.Get(url) // 慢操作在锁外，不阻塞其他读写
	body := readBody(resp)
	c.mu.Lock()
	c.data[k] = body
	c.mu.Unlock()
}
```

### sync.RWMutex

**读多写少用读写锁**：任意时刻要么有任意多个读者，要么有唯一一个写者，读写互斥、写写互斥、多读并发。

```go
// 读写锁保护配置缓存。
type config struct {
	mu   sync.RWMutex
	data map[string]string
}

func (c *config) Get(k string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data[k]
}

func (c *config) Set(k, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[k] = v
}
```

- **Go 的 RWMutex 采用写者优先策略**：一旦有写者申请锁，后续读者必须排队，避免写者饥饿。其实现的精妙之处在于 `Lock()` 会把 `readerCount` 减去一个巨大常量（`rwmutexMaxReaders = 1<<30`）反转为负数——一个原子操作同时完成"标记有写者"和"继续累计排队读者数"两件事；最后一个读者退出时负责唤醒写者，写者 Unlock 时再唤醒所有排队读者。
- 误用场景与 Mutex 相同（成对、禁复制、不可重入），另外注意：`RUnlock` 一个未加读锁的锁会直接崩溃；`RLock` 之后在同 goroutine 里 `Lock` 是经典的循环等待死锁。
- RWMutex 不是免费的：读路径也要做原子加减，临界区极短（如一次 map 读取）时未必比 Mutex 快。读比例不够高或临界区极小时，先 benchmark 再引入。

### sync.Cond

**条件变量用于"等待某个条件成立"**：一组 goroutine 挂起等待，另一组在条件可能变化时发出通知。

```go
// 用 Cond 实现简单的就绪通知。
var (
	mu    sync.Mutex
	cond  = sync.NewCond(&mu)
	ready bool
)

func waitReady() {
	mu.Lock()
	for !ready { // 必须用 for 循环复查条件，防止虚假唤醒
		cond.Wait() // Wait 会原子地释放锁并挂起，被唤醒后重新持有锁
	}
	mu.Unlock()
}

func markReady() {
	mu.Lock()
	ready = true
	mu.Unlock()
	cond.Broadcast() // Broadcast 唤醒全部等待者；Signal 只唤醒一个
}
```

- `Wait` 必须在持锁状态下调用；条件判断必须用 `for` 而非 `if`；`Signal` 唤醒一个等待者（不保证是等待最久的），`Broadcast` 唤醒全部。多数"等待条件"的场景用 channel 表达更直观，Cond 主要用于实现底层同步结构。

## 9.4 sync.WaitGroup 与 sync.Once

### WaitGroup：等待一批任务结束

```go
// 等待三个 worker 完成。
var wg sync.WaitGroup
for i := 0; i < 3; i++ {
	wg.Add(1)
	go func() {
		defer wg.Done() // Done 等价于 Add(-1)
		// ... 干活
	}()
}
wg.Wait() // 阻塞到计数归零
```

三件套口诀：**Add 在 `go` 语句之前完成，Done 用 `defer` 配对，Wait 只由一方调用**。9.2 节的 `fanIn` 和 9.8 节的 worker pool 都是这套口诀的实例。

- **Add 要在 Wait 之前完成**（通常放在启动 goroutine 之前）；Add 与 Wait 并发调用会 panic。
- 计数不能为负，多调 Done 会 panic（`negative WaitGroup counter`）；Wait 返回之前不要复用同一个 WaitGroup，否则 panic（`WaitGroup is reused before previous Wait has returned`）。含 noCopy 标记，不能值传递；把 WaitGroup 传给函数时用指针。

### Once：保证只执行一次

```go
// 懒加载配置。
var (
	once   sync.Once
	config Config
)

func loadConfig() Config {
	once.Do(func() {
		config = readConfigFromDisk() // 无论多少 goroutine 调用，只执行一次
	})
	return config
}
```

`Once` 的全部实现只有十几行：快路径用原子读检查 `done` 标志，已完成则直接返回（无锁）；慢路径加互斥锁后**双检查**（防止多个 goroutine 同时进入慢路径重复执行），f 返回后用 `defer` 原子置位——即使 f panic，done 也会被标记为已执行。典型用途是单例初始化、配置懒加载。

这个"panic 也算执行过"的语义值得记住：f 失败后不会重试。需要"失败可重试"的懒初始化，得自己用 mutex + 状态位实现。

## 9.5 原子操作与 sync.Map

### sync/atomic：硬件级的轻量同步

**`sync/atomic` 提供底层原子内存原语，适合简单计数器、标志位**；官方提醒：除底层特殊场景外，优先使用 channel 或 sync 包。

```go
// 原子计数器。
var hits atomic.Int64 // Go 1.19+ 提供的类型化原子类型，防止非原子访问

hits.Add(1)        // 原子自增
n := hits.Load()   // 原子读
_ = n
```

Go 1.19 之前只有函数式 API，`atomic.AddInt64(&x, 1)` 里的 `&x` 是裸指针，任何一处忘记走原子函数直接读写 `x` 就是竞争，编译器不会报错。类型化的 `atomic.Int64` 把"只能原子访问"变成了类型约束——`hits++` 根本编译不过。新代码一律用类型化 API。

传统函数式 API 覆盖三类操作：

| 类别 | 函数（以 T ∈ int32/int64/uint32/uint64/uintptr 为例） |
| --- | --- |
| 加减 | `AddT(addr *T, delta T) T`（减法传负的 delta） |
| 读/写 | `LoadT(addr *T) T`、`StoreT(addr *T, v T)` |
| 交换 | `SwapT(addr *T, new T) T`、`CompareAndSwapT(addr *T, old, new T) bool` |

- 另有 `atomic.Value` 可对任意**类型一致**的值做原子 Load/Store（首次 Store 后类型不可变，值不可为 nil）。
- 底层上，Add 依赖 CPU 的 `LOCK` 前缀加 `XADD` 指令保证多核下的独占访问；CAS 对应 `LOCK CMPXCHG`，是实现无锁数据结构的基本积木（例如用 CAS 循环实现自旋锁）。atomic 的局限也很明确：一次只能原子操作一个机器字，跨多个变量的一致性仍需锁。

### sync.Map：为特定场景优化的并发 Map

**sync.Map 为两类场景做了空间换时间的优化**：key 只写一次、读多次（只增不删的缓存）；多个 goroutine 操作互不相交的键集合。

**Go 1.24 起，sync.Map 的底层实现换成了 HashTrieMap（并发哈希前缀树）**：读取沿原子指针逐层下行，修改只锁定路径上最近的间接节点，不同分支的写可以使用不同的节点锁，没有全局限级的大锁。你不需要关心树形细节，记住结论即可：读多写少时读路径无锁，写入并发度比"整个 map 一把锁"高。

> 演进备注：Go 1.23 及更早的版本用"read 只读快照 + dirty 加锁写 + miss 计数提升"的双 map 结构。阅读旧文章或旧源码分析（包括一些基于 go1.14 的教程）时会遇到这套机制，知道它已被替换即可；API 和适用场景没有变化。

使用边界：sync.Map 不能值传递；元素类型只能是 `interface{}`（损失类型安全，需断言）；**读少写多或需要强类型时不适用**——此时用 `RWMutex + map` 或对 map 分片加锁更合适。

## 9.6 sync.Pool：临时对象复用

**高频分配临时对象会给 GC 带来压力，`sync.Pool` 是并发安全的临时对象池**：

```go
// 复用缓冲区。
var bufPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) }, // 取不到时现场创建
}

func process(data []byte) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()               // 取出的对象可能带脏数据，先重置
	defer bufPool.Put(buf)    // 用完记得归还
	buf.Write(data)
	// ...
}
```

必须理解的两条边界：

- **池中对象随时可能被回收**：GC 会把 local 池整体移交 victim 区，并在下一轮 GC 清空 victim——对象的存活期最多约两个 GC 周期。不要把 Pool 当缓存用，取不到就重新创建。
- **取出的对象要自己清理**：Put 之前/之后的状态由你负责，忘记 Reset 会把上次的数据泄漏给下一个使用者。

性能敏感路径上（如网络库复用 timer、JSON 编码器复用 buffer）Pool 收益明显；普通业务代码先保证正确性，有 benchmark 证据再引入。

## 9.7 context：取消、超时与值传递

**goroutine 启动后，外部通过 context 通知它停止**。所有 context 构成一棵树，新 context 基于父 context 创建，取消沿树向下级联。

```go
// 标准库 Context 接口定义摘录。
type Context interface {
	Deadline() (deadline time.Time, ok bool) // 截止时间，未设置时 ok=false
	Done() <-chan struct{}                   // 取消/超时/完成时该通道被关闭
	Err() error                              // 未结束返回 nil；否则是 Canceled 或 DeadlineExceeded
	Value(key any) any                       // 沿树向上查找 key 对应的值
}
```

四种实现：

| 类型 | 创建函数 | 用途 |
| --- | --- | --- |
| emptyCtx | `context.Background()` / `context.TODO()` | 空根节点；TODO 用于暂时不确定传什么时 |
| cancelCtx | `context.WithCancel(parent)` | 手动取消，级联取消所有子孙 |
| timerCtx | `context.WithTimeout(parent, d)` / `context.WithDeadline(parent, t)` | 到期自动取消 |
| valueCtx | `context.WithValue(parent, k, v)` | 携带请求级数据 |

典型模式：超时控制。

```go
// 带超时的等待。
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel() // 必须调用：提前结束也要释放定时器和子节点

select {
case <-ctx.Done():
	return ctx.Err() // 超时返回 DeadlineExceeded
case result := <-resultCh:
	return result
}
```

对照 9.2 节 `firstResult` 里的 `time.After`：两者都能给等待加上限，区别在于 `time.After` 只通知等待方，背后的生产者无从得知结果已经没人要了；`context` 则可以把取消信号一路传给生产者（比如 HTTP handler 把 `r.Context()` 传进每个下游调用），让整条链上的 goroutine 一起退出。等待方只管自己退出、生产者收不到通知，正是 goroutine 泄漏的典型成因。

使用规范：

- Context 作为函数**第一个参数**显式传递，命名为 `ctx`；不要存进结构体字段。不传 nil Context，不确定时用 `context.TODO()`。
- 每次 `WithCancel/WithTimeout/WithDeadline` 返回的 `cancel` 都必须被调用（通常 `defer cancel()`），否则关联资源直到父 context 结束才释放。
- `WithValue` 只放请求级元数据（trace ID、认证信息），不放业务参数；key 用不导出的自定义类型，避免包间冲突。
- Context 是并发安全的，可以跨 goroutine 传递；取消是"通知"，收到通知的 goroutine 要自己检查 `ctx.Done()` 并完成清理。

## 9.8 示例：从单次竞争到 worker pool

两个 lab 把上面的原语串成两类典型场景：

| Topic | 场景 | 核心关注点 |
| --- | --- | --- |
| `goroutine-channel-select` | 少量任务竞争结果 | `select`、fan-in、超时/提前返回 |
| `worker-pools` | 固定并发度处理一批任务 | jobs channel、`WaitGroup`、`Mutex`、收尾顺序 |

`goroutine-channel-select` 的三个函数（`startWorker`、`firstResult`、`fanIn`）已分别在 9.2 节展开，它的 main 只是把 cache（10ms）和 database（30ms）两个 worker 放进 `firstResult` 竞争——缓存先到，打印缓存结果。下面完整拆解 worker pool。

**worker pool 用固定数量的 goroutine 消费任务队列**，是把并发度限制在可控范围内的标准结构。先看单个 worker，摘录自 [worker-pools/main.go](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/blob/main/labs/part1-golang/chapter09/worker-pools/main.go)：

```go
func worker(id int, jobs <-chan job, reports chan<- report, stats *poolStats, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		stats.markStarted()
		score := scoreJob(j)
		stats.markCompleted(score)
		reports <- report{jobID: j.id, worker: id, quality: score}
	}
}
```

逐行看：

- `defer wg.Done()`：无论 jobs 何时关闭、循环何时退出，计数一定递减——Add/Done 配对靠 defer 保证，panic 路径也不漏。
- `for j := range jobs`：worker 的退出条件是"jobs 被关闭且取空"。它不知道总共有多少任务，也不需要知道——这正是 channel 编排生命周期的体现。
- `stats` 的全部读写都走 `markStarted()`/`markCompleted()` 方法，锁（`sync.Mutex`）封装在方法内部——9.3 节"共享状态只暴露带锁的操作"的实例，worker 代码里看不到任何锁。
- `reports <-`：reports 是容量等于任务数的缓冲通道（见下），worker 发结果永不阻塞，发完继续取下一个任务。

再看编排者 `runWorkerPool`，同样摘录自 [worker-pools/main.go](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/blob/main/labs/part1-golang/chapter09/worker-pools/main.go)：

```go
func runWorkerPool(workerCount int, items []job) ([]report, statsSnapshot, error) {
	if workerCount < 1 {
		return nil, statsSnapshot{}, errInvalidWorkerCount
	}

	jobs := make(chan job)
	reports := make(chan report, len(items))
	stats := &poolStats{}

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for id := 1; id <= workerCount; id++ {
		go worker(id, jobs, reports, stats, &wg)
	}

	for _, item := range items {
		jobs <- item
	}
	close(jobs)

	wg.Wait()
	close(reports)

	results := make([]report, 0, len(items))
	for item := range reports {
		results = append(results, item)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].jobID < results[j].jobID
	})

	return results, stats.snapshot(), nil
}
```

这个函数的收尾顺序是固定五步，一步都不能换：

1. **启动 worker**：`wg.Add(workerCount)` 在所有 `go` 语句之前一次完成。
2. **投递并 `close(jobs)`**：主 goroutine 是 jobs 唯一的发送方，由它关闭；worker 的 `range` 借此得知"没有更多任务了"。
3. **`wg.Wait()`**：等全部 worker 退出。
4. **`close(reports)`**：此刻已不存在任何发送方，关闭安全——多发送方场景"由掌握生命周期的一方关闭"的又一个实例。
5. **`range` 收空 + 排序返回**：`range` 依赖步骤 4 的关闭才能结束；按 `jobID` 排序把调度造成的不确定顺序重新变成确定性输出。

几个容易忽略的设计决定：

- **jobs 故意用无缓冲**：投递方阻塞到某个 worker 接手，形成背压——任务生产的速度被消费速度自然限制。
- **reports 用满缓冲（容量 = 任务数）**：worker 永远不会卡在发送结果上，`Wait` 一定能返回。如果 reports 缓冲不足又没有并发接收者，worker 阻塞在发送上、`Wait` 永不返回，整个程序挂起（见踩坑清单）。更通用的写法是并发收集结果，并由一个等待所有 worker 退出的 goroutine 负责关闭结果通道。
- **`workerCount < 1` 提前返回错误**：这个校验不是形式主义——若放行，jobs 无人消费，主 goroutine 会永远阻塞在第一次 `jobs <- item` 上，死锁而不报错。
- **排序让结果可断言**：哪些任务由哪个 worker 执行是调度决定的（每次运行可能不同），但排序后的 `[]report` 内容和 `stats` 汇总是确定的——测试只断言确定的部分，这是并发代码可测试的关键。

运行示例（仓库根目录执行）：

```bash
go -C labs run ./part1-golang/chapter09/goroutine-channel-select
go -C labs run ./part1-golang/chapter09/worker-pools
```

`goroutine-channel-select` 预期输出：

```text
cache: cached profile
```

`worker-pools` 预期打印 3 条 job 处理结果和 1 条汇总行（worker 编号可能不同）：

```text
completed 3 jobs, total quality 29
```

## 踩坑清单

### 只启动 goroutine，不安排接收者

```go
func leak() {
	ch := make(chan string)
	go func() {
		ch <- "done" // 反例：如果没人接收，这个 goroutine 会一直卡住。
	}()
}
```

现象：goroutine 永久阻塞在发送上，无法退出——并发代码最大的风险之一就是"看起来已经跑起来了"，其实永远收不了尾（回到本章开头那个每小时 72 万个泄漏 goroutine 的场景，成因就是这种发送）。修复：为这个单结果场景安排接收者，或使用容量为 1 的通道（`startWorker` 的做法）；持续生产者还要监听取消信号，不能只增加有限缓冲。每个 goroutine 都要能回答：什么条件下退出？

### 由错误的一侧关闭通道

```go
func worker(results chan<- Result) {
	// 反例：worker 不是唯一的发送方时，不应该自己关闭 results。
	close(results)
}
```

现象：其他发送方向已关闭的通道发送，panic。修复：移除每个 worker 中的 `close`，把关闭操作放到等待所有发送者结束的位置（`fanIn` 里 `wg.Wait()` 之后的协调 goroutine、`runWorkerPool` 里 `wg.Wait()` 之后的主 goroutine，都是正确位置）；复验时确认每个任务只汇总一次，汇总值不依赖 worker 分配顺序。

### 共享 map 不加保护

```go
// 反例：内建 map 不是并发安全的，并发读写会触发 fatal error: concurrent map writes。
m := make(map[string]int)
go func() { m["a"]++ }()
go func() { m["b"]++ }()
```

现象：`fatal error: concurrent map writes`，或静默丢失更新。修复：按场景选择 `Mutex`/`RWMutex` 保护、改用 channel 传递所有权、或符合场景时使用 `sync.Map`；复验用 `go test -race` 让数据竞争显形——多次运行未报告也不证明无竞争（可在独立副本中临时移除 worker-pools 的 poolStats 锁观察竞争报告，恢复锁后 race 应通过）。

### 测试断言 select 的固定分支

现象：缓存与备用结果同时就绪时，select 伪随机选择，断言"一定返回缓存值"的测试偶发失败。修复：断言结果属于允许集合，或者用受控通道明确安排先后（lab 测试里的 `readyResult` 预填一个结果再关闭通道，就是让分支确定性就绪的手法）；不要用某次 worker 分配顺序或短 demo 的耗时判断并发代码——正确性用 `go test -race`，性能用 benchmark。

### 结果通道无缓冲且先 Wait 后收

现象：worker 卡在发送结果上，`Wait` 永远无法返回，程序挂起。修复：结果通道缓冲长度给足任务数（`runWorkerPool` 的做法），或并发收集结果、由等待所有 worker 退出的 goroutine 负责关闭结果通道。

### 在 goroutine 内部调 wg.Add

```go
var wg sync.WaitGroup
for i := 0; i < 3; i++ {
	go func() {
		wg.Add(1) // 反例：Add 与外面的 Wait 并发执行
		defer wg.Done()
		// ...
	}()
}
wg.Wait() // 可能在任何 Add 之前返回；也可能触发 panic
```

现象：`Wait` 提前返回（主流程以为任务已完成），或运行时报 `WaitGroup is reused before previous Wait has returned` 一类的 panic。原因：Add 必须在 Wait 可观察到之前完成，这是内存模型层面的要求，不是风格建议。修复：`wg.Add(1)` 放在 `go` 语句之前（如 9.4 节的写法），或像 `fanIn` 那样在循环外一次 `wg.Add(n)`。

### time.After 放进高频循环

```go
for {
	select {
	case msg := <-ch:
		handle(msg)
	case <-time.After(time.Minute): // 反例（Go 1.23 之前）：每轮循环新建一个定时器
		refresh()
	}
}
```

现象：内存随循环次数持续增长。原因：Go 1.23 之前，不再被引用的 Timer 要等到期才被 GC 回收；循环每转一圈就堆积一个定时器。修复：循环外用 `time.NewTimer` 创建一次、每轮 `Reset` 复用（Reset 前注意按文档先停掉并排空通道）；Go 1.23 起运行时可以立即回收不再被引用的定时器，这个坑在新版本上消失，但维护旧版本代码或读旧文章时仍需知道。

<a id="concurrency-exercises"></a>

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 9.1 覆盖超时分支 | 在 `goroutine-channel-select` 的测试里补一个 TestTimeoutExercise：primary、fallback 都传不发送任何结果的 `make(chan result)`，把 timeout 设为毫秒级后调用 firstResult | 返回 `ok=false` 且结果为 `result{}` 零值；测试在 `-race` 下通过；对照 main.go 中超时分支的写法与 main_test.go 已有的 "times out" 子测试，并在注释里说明：firstResult 超时返回后，两个 startWorker 的 goroutine 靠什么机制（容量为 1 的缓冲通道）仍能完成发送并退出，而不是泄漏 | `goroutine-channel-select/main_test.go` |
| 9.2 调整 worker 数量 | 在测试或临时 main 中分别用 1、2、3 个 worker 调用 runWorkerPool 处理同一批 items，保留 `workerCount < 1` 的错误处理与汇总输出 | 三种 worker 数下 stats.completed 都等于任务数、stats.total 都是 29、排序后的 reports 内容一致；分配顺序可不同，不断言某个任务必由固定 worker 执行；补充说明 workerCount=0 时为什么必须返回错误而不是静默挂起 | `worker-pools/main.go`、`main_test.go` |

完成后运行受影响示例及测试：

```bash
go -C labs test ./part1-golang/chapter09/goroutine-channel-select/... -count=1
go -C labs test ./part1-golang/chapter09/worker-pools/... -count=1
go test -race ./labs/part1-golang/chapter09/worker-pools/... -count=1
```

更多练习（如检查共享统计的锁保护、用 `collectAll` 收集全部竞争结果）见各 lab 的 README。

## 链接

- [Go 内存模型官方文档](https://go.dev/ref/mem)：happens-before 的权威定义，本章 9.1 节表格中每条保证的出处。
- [pkg.go.dev/sync](https://pkg.go.dev/sync)、[pkg.go.dev/sync/atomic](https://pkg.go.dev/sync/atomic)、[pkg.go.dev/context](https://pkg.go.dev/context)：本章各原语的官方 API 文档，含类型化原子类型（Go 1.19+）等版本沿革说明。
- [并发编程教程（cyub）](https://go.cyub.vip/concurrency/)：本章 9.1–9.7 的主要参考，含各原语的源码级分析：[内存模型](https://go.cyub.vip/concurrency/memory-model/)、[channel](https://go.cyub.vip/concurrency/channel/)、[context](https://go.cyub.vip/concurrency/context/)、[atomic](https://go.cyub.vip/concurrency/atomic/)、[sync.Map](https://go.cyub.vip/concurrency/sync-map/)、[sync.WaitGroup](https://go.cyub.vip/concurrency/sync-waitgroup/)、[sync.Once](https://go.cyub.vip/concurrency/sync-once/)、[sync.Pool](https://go.cyub.vip/concurrency/sync-pool/)、[sync.Cond](https://go.cyub.vip/concurrency/sync-cond/)、[sync.Mutex](https://go.cyub.vip/concurrency/sync-mutex/)、[sync.RWMutex](https://go.cyub.vip/concurrency/sync-rwmutex/)；调度部分见 [G-M-P 调度机制](https://go.cyub.vip/gmp/)。注意该站源码分析基于 go1.14，sync.Map 等实现细节在 Go 1.24+ 已有变化，以 [Go 1.24 Release Notes](https://go.dev/doc/go1.24) 和当前源码为准。
- [并发、并行和协程](https://learnku.com/docs/the-way-to-go/141-concurrency-parallel-and-co-process/3685)
- [使用 select 切换协程](https://learnku.com/docs/the-way-to-go/144-switching-co-process-using-select/3688)
- [通道、超时与计时器](https://learnku.com/docs/the-way-to-go/145-channels-timeouts-and-timers-ticker/3689)
- [限制并发处理的请求数量](https://learnku.com/docs/the-way-to-go/1411-limiting-the-number-of-requests-processed-concurrently/3695)
