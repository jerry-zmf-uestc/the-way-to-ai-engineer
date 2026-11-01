# 第 11 章：工程实践与代码规范

代码能跑只是起点。接手一个陌生仓库时，最先暴露问题的不是算法，而是这些细节：打开 `cmd/`、`internal/`、`pkg/`、`api/`、`deployments/` 一应俱全，点进去一半目录是空的，剩下一半各躺着一个没人敢删的 `main.go`；错误信息全是 `"error"`，定位一次失败要全文搜索；函数名叫 `DoIt`，改动它之前没人知道会破坏谁。本章的判断：**先让职责清楚，再谈目录完整**——命名、错误信息和目录职责决定这份代码能否被别人接手、扩展和审查。

## 目录职责用数据表达

**module 是依赖与版本边界，package 是编译与可见性边界；目录建齐不等于设计合理。** 用读者熟悉的生态类比：go.mod ≈ package.json（声明模块路径与依赖版本），package ≈ Python 的模块文件，而 `internal/` 类似 Python 的 `_private` 前缀约定——区别在于 Go 由工具链**强制**执行，不是靠自觉。目录叫 cmd 不会自动让程序更好测试——只有把参数处理、业务函数和 I/O 分清楚，测试才能替换输入而不必启动整个进程。

规范写在文档里只能靠自觉，写成数据才能被程序消费：可以过滤出"现在就该有"的目录、查询某个目录的职责、在测试里断言职责说明不为空。[project-layout](../../../labs/part1-golang/chapter11/project-layout) 把仓库的目录职责编码成数据，后面才能输出、过滤、测试。摘录自 project-layout/main.go。

```go
type RoleStatus string

const (
	StatusCurrent  RoleStatus = "current"
	StatusDeferred RoleStatus = "deferred"
)

var ErrUnknownDirectory = errors.New("unknown project directory")

type DirectoryRole struct {
	Path           string
	Status         RoleStatus
	Responsibility string
	UseWhen        string
}
```

四个字段各自回答一个问题，缺一不可：

| 字段 | 回答的问题 | 空缺时的后果 |
| --- | --- | --- |
| `Path` | 这个目录在哪 | 条目无法被 `FindRole` 定位 |
| `Status` | 现在就该有，还是等到条件满足再建 | deferred 目录被提前创建，仓库堆满空目录 |
| `Responsibility` | 它装什么、不装什么 | 目录沦为杂物间，什么文件都往里放 |
| `UseWhen` | 什么条件下引入或更新它 | 新人无法判断自己的代码该放哪 |

`ProjectRoles()` 返回全部条目，`StatusCurrent` 的条目是现在就该存在的，`StatusDeferred` 的先记录在案、条件满足才创建。各取一条对照（两段均摘录自 project-layout/main.go）：

```go
		{
			Path:           "chapterN/<name>/cmd",
			Status:         StatusCurrent,
			Responsibility: "章节内分层项目的薄入口",
			UseWhen:        "练手项目需要稳定二进制入口时使用",
		},
```

```go
		{
			Path:           "pkg",
			Status:         StatusDeferred,
			Responsibility: "对外可复用库代码",
			UseWhen:        "出现跨仓库对外复用需求并愿意维护稳定 API 时再使用",
		},
```

注意 `pkg` 的 `UseWhen`：它不是"Go 项目标配目录"，而是**有跨仓库复用需求且愿意维护稳定 API** 时才引入的承诺——`pkg/` 下的代码一旦被别人 import，每次改动都多一层兼容负担。

查询函数用 `%w` 包装哨兵错误，调用方可以用 `errors.Is` 判定"目录未知"这一类失败（摘录自 project-layout/main.go）：

```go
func FindRole(path string) (DirectoryRole, error) {
	clean := strings.Trim(strings.TrimSpace(path), "/")
	for _, role := range ProjectRoles() {
		if role.Path == clean {
			return role, nil
		}
	}

	return DirectoryRole{}, fmt.Errorf("%w: %s", ErrUnknownDirectory, path)
}
```

运行 `go -C labs run ./part1-golang/chapter11/project-layout`，输出当前应存在目录的职责清单（deferred 条目留在数据里供 `FindRole` 查询，不出现在清单中）：

```text
chapterN/<topic> [current]: 学习示例按章节和 topic 归档; use when 每天产出可运行 Go topic 时更新所属章节目录
docs [current]: 学习计划、工程规则和目录约束; use when 结论已被完成示例或 spec 支撑后沉淀规则
records [current]: 飞书不可用时的本地学习记录兜底; use when 文档读写或私聊失败时追加脱敏复盘
scripts [current]: 自动化入口和本地验证辅助脚本; use when 需要稳定复用的仓库级操作，不承载业务代码
.specs [current]: SDD topic 的 requirements、design、tasks 和 acceptance; use when CLI、小服务、并发或 mock 类任务开始前复制模板
chapterN/<name>/cmd [current]: 章节内分层项目的薄入口; use when 练手项目需要稳定二进制入口时使用
chapterN/<name>/internal [current]: 章节内分层项目的私有实现; use when 入口背后的业务逻辑需要拆分和测试时使用
```

常见的"标配目录"在这份数据里大多是 deferred。它们的引入条件值得记住：

| 目录 | 职责 | 何时引入 |
| --- | --- | --- |
| `pkg` | 对外可复用库代码 | 出现跨仓库对外复用需求，并愿意维护稳定 API |
| `api` | OpenAPI、proto 等接口契约 | HTTP 服务进入契约化或多客户端协作阶段 |
| `configs` | 示例配置和默认配置模板 | 服务示例开始读取外部配置文件 |
| `deployments` | 部署描述和运行环境配置 | 服务需要 Docker、Kubernetes 或平台部署说明 |
| `vendor` | 第三方依赖源码快照 | 有明确离线构建要求；默认依赖 go mod，不提交 vendor |

**要不要拆、拆成什么样，由项目形态决定，不由模板决定。** [module-layout](../../../labs/part1-golang/chapter11/module-layout) 把四种常见形态的布局建议同样编码成数据（摘录自 module-layout/main.go 中 `RecommendLayout` 的 server 分支）：

```go
	case ShapeServer:
		return LayoutRecommendation{
			Shape:       shape,
			Summary:     "put server entrypoints in cmd/ and private logic in internal/",
			Directories: []string{"cmd/<service>/main.go", "internal/<domain>/", "go.mod"},
			Reason:      "servers are usually self-contained binaries, so private implementation belongs under internal/",
		}, nil
```

四种形态的选择依据：

| 项目形态 | 起步布局 | 理由 |
| --- | --- | --- |
| package（可导入库） | `go.mod` + 根包 | 根包名与模块名一致时最方便被 import |
| command（单个命令） | `go.mod` + 根目录 `main.go` | 单命令在出现更多二进制或包之前不需要 `cmd/` |
| mixed（命令 + 库） | `cmd/<name>/` + 顶层包 + `internal/` | `cmd/` 让二进制可发现，`internal/` 让私有支撑包可放心重构 |
| server（自包含服务） | `cmd/<service>/` + `internal/<domain>/` | 服务通常是独立部署的二进制，私有实现收进 `internal/` |

运行 `go -C labs run ./part1-golang/chapter11/module-layout` 查看 server 形态的完整建议。共同的原则：**每个目录存在的那一刻就要能说出它装什么、为什么现在需要它**。

## cmd + internal：入口和实现分开

**拆分只改目录，不改外部行为：状态码、响应字段和错误路径必须在重构前后一致。** 入口只装配依赖并启动，业务逻辑落在 internal 包里，接收可替换的输入，便于测试。[json-api](../../../labs/part1-golang/chapter11/json-api) 是从单包 API 升级到分层项目的完整示例。先看入口，它薄到只有装配（摘录自 json-api/cmd/main.go）：

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part1-golang/chapter11/json-api/internal/jsonapi"
)

func main() {
	os.Exit(run(os.Stdout))
}

func run(stdout io.Writer) int {
	service := jsonapi.NewEchoService()
	handler := jsonapi.NewServer(service)
	fmt.Fprintln(stdout, jsonapi.DemoRequest(handler))
	return 0
}
```

两个细节值得学：`main` 本身不承载逻辑，真正的入口是 `run(stdout io.Writer) int`——输出目的地是参数而不是写死的 `os.Stdout`，退出码是返回值而不是中途 `os.Exit`，所以 `cmd/main_test.go` 可以传进 buffer 直接断言输出。业务逻辑全部落在 internal 包里（摘录自 json-api/internal/jsonapi/service.go，节选）：

```go
// ErrEmptyMessage reports an echo request that contains no useful content.
var ErrEmptyMessage = errors.New("message is required")

// EchoService owns the private learning logic behind the HTTP endpoint.
type EchoService struct{}

// NewEchoService constructs the service without exposing mutable state.
func NewEchoService() EchoService {
	return EchoService{}
}

// Echo validates and normalizes the message before building the response.
func (EchoService) Echo(message string) (EchoResponse, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return EchoResponse{}, ErrEmptyMessage
	}

	return EchoResponse{
		Echo:   message,
		Length: utf8.RuneCountInString(message),
	}, nil
}
```

导出标识符都有以名字开头的文档注释（`ErrEmptyMessage reports...`、`Echo validates...`）——这是 Go 的惯例，godoc 和 IDE 直接把它们当文档展示。`Echo` 不碰 HTTP：校验、归一化、构造响应都是纯逻辑，`service_test.go` 不起服务就能测；HTTP 的编解码、状态码映射留在同包的 `server.go` 里，那是"入口侧"的另一半职责。

运行 `go -C labs run ./part1-golang/chapter11/json-api/cmd` 可以看到分层后的行为与单包版完全一致：

```text
POST /api/echo -> 200 {"echo":"hello, json api","length":15}
```

`internal/` 的导入限制由 Go 工具链**强制**执行：`internal/jsonapi` 只能被 `json-api/` 目录树内部的代码导入，外部模块 import 它会直接编译报错——这是编译错误，不是 lint 建议。相对的，`pkg/` 不是必须创建的保留目录，没有任何工具链语义。检查目录职责时也检查反向依赖：业务包不应为了输出日志而依赖 cmd。

[review-cli](../../../labs/part1-golang/chapter11/review-cli) 是同一模式的命令行版本，它展示了入口的另一项职责：**把内部错误分类映射成进程退出码**（摘录自 review-cli/cmd/main.go，节选）：

```go
	report, err := review.Summarize(items)
	if err != nil {
		fmt.Fprintf(stderr, "review-cli: %v\n", err)
		if isValidationError(err) {
			return 2
		}
		return 1
	}
```

`isValidationError` 用 `errors.Is` 依次匹配 `ErrNoItems`、`ErrInvalidItem`、`ErrInvalidTopic`、`ErrInvalidScore` 四个哨兵；`internal/review` 包里的 `InputError` 类型实现 `Unwrap` 返回底层分类，所以入口不需要理解错误的文字内容，只按类型决策。退出码约定随之清晰：

| 退出码 | 含义 | 触发条件 |
| --- | --- | --- |
| 0 | 成功 | 汇总完成，报告写入 stdout |
| 1 | 运行期失败 | 非输入类错误（如输出写入失败） |
| 2 | 用法/输入错误 | flag 解析失败，或输入未通过校验 |

脚本调用方靠退出码区分"用户输错了"和"程序出问题了"——前者提示改输入，后者才值得报警。运行验证两种路径：

```bash
go -C labs run ./part1-golang/chapter11/review-cli/cmd -item "goroutine=4" -item "channel=2" -item "slice=5"
# items=3 average=3.67 status=needs-review weakest=channel（退出码 0）

go -C labs run ./part1-golang/chapter11/review-cli/cmd
# review-cli: review: no items（退出码 2）
```

## 规范与优化取舍

**规范是降低协作成本的工具；优化类写法先用 benchmark 证明值得，再保留。**

[effective-go](../../../labs/part1-golang/chapter11/effective-go)（`go -C labs run ./part1-golang/chapter11/effective-go`）演示三条官方规范要点：命名（包级哨兵错误小写开头 `errEmptyQuantity`，不导出就不伪装成 API）、零值可用（`retryPolicy{}` 不初始化也能正常工作，`normalizedAttempts` 把零值归一成 1——结构体的零值应当即合法状态）、early return（`parseQuantity` 用连续的守卫语句先排除空输入、解析失败、非正数，主路径保持最左缩进）。[code-review-comments](../../../labs/part1-golang/chapter11/code-review-comments) 演示写法为什么影响 review 成本：先检查 `ctx.Err()` 再开始工作、缺失的必需检查项 `sort.Strings` 排序后再报错（输出确定，测试和日志比对都容易）。

三个 Uber 风格示例（uber-errors-wrap、uber-slice-preallocate、uber-string-builder）则是另一类：错误包装、切片容量预分配和 strings.Builder 都是**具体取舍**，不是见到字符串就必须用 Builder；先写 benchmark，再决定优化是否值得保留。

**错误包装：给调用方留下判定入口。** 摘录自 uber-errors-wrap/main.go：

```go
// LoadConfig reads a config file and wraps low-level errors with %w so that
// callers can keep using errors.Is / errors.As to inspect the failure.
func LoadConfig(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 同时保留 sentinel 与底层错误，调用方两种判定方式都能用。
			return nil, fmt.Errorf("load config %q: %w", path, errors.Join(ErrConfigNotFound, err))
		}
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}
	return data, nil
}
```

`errors.Join` 把自己的哨兵 `ErrConfigNotFound` 和底层 `os.ErrNotExist` 绑在一起，调用方两种判定都能命中。对照写法：

```go
// ❌ %v 吞掉错误链：调用方拿不到 os.ErrNotExist，无法区分"不存在"和"权限不足"
return nil, fmt.Errorf("load config %q: %v", path, err)

// ✅ %w 保留错误链：errors.Is(err, os.ErrNotExist) 仍然成立
return nil, fmt.Errorf("load config %q: %w", path, err)
```

**切片预分配：有上界就一次给够。** 摘录自 uber-slice-preallocate/main.go：

```go
func FilterEven(src []int) []int {
	out := make([]int, 0, len(src))
	for _, v := range src {
		if v%2 == 0 {
			out = append(out, v)
		}
	}
	return out
}
```

偶数最多 `len(src)` 个，所以容量上界已知。对照反例：

```go
// ❌ 容量从零起步，append 反复扩容、拷贝、产生垃圾
var out []int

// ✅ 按上界预分配，全程零扩容
out := make([]int, 0, len(src))
```

**字符串拼接：循环里用 Builder 并预估容量。** 摘录自 uber-string-builder/main.go：

```go
	var b strings.Builder
	// 预估容量，避免 Builder 多次扩容。
	b.Grow(len(tags) * 8)

	for i, t := range tags {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(t)
	}
	return b.String()
```

循环里 `out += t` 每次都分配新字符串（string 不可变），是经典的 O(n²) 分配。但注意这条规范的边界：固定几个字符串用 `+`、已有切片用 `strings.Join` 都更直白——Builder 的价值在**循环次数多且在热路径**时。用 benchmark 验证而不是靠感觉：

```bash
go test -bench=. -benchmem ./labs/part1-golang/chapter11/uber-string-builder
```

输出的 `ns/op` 数值随机器变化，没有跨环境意义；值得盯的是 `B/op` 和 `allocs/op` 两列——这个实现稳定为 1 次分配。给优化"记账"的标准流程：先写正确版本，benchmark 建立基线，改动后对比，收益不显著就回退到直白的写法。

## 调试与分析工具

写完代码之后的排障也有标准工具链，先知道入口，用到时再深入：

```bash
# 测试与静态检查（在仓库根目录执行）
go test -race ./labs/part1-golang/chapter09/...   # 数据竞争检测
go test -cover ./labs/part1-golang/chapter04/...  # 覆盖率
go -C labs vet ./...                                 # 静态检查

# 运行时分析：benchmark + pprof
go test -bench=. -benchmem ./labs/part1-golang/chapter11/uber-string-builder
go test -cpuprofile cpu.out ./labs/part1-golang/chapter11/uber-string-builder
go tool pprof -top cpu.out                        # 查看热点（分析完删除 cpu.out，不提交）
```

按症状选工具：

| 症状 | 工具 | 入口 |
| --- | --- | --- |
| 并发代码偶发失败、结果不确定 | race detector | `go test -race` |
| 改了逻辑不确定测试覆盖到没有 | cover | `go test -cover` |
| 可疑写法（printf 参数不匹配、不可达的代码等） | vet | `go vet` |
| 需要在测试里设断点、单步、查看变量 | delve | `dlv test` |
| 疑惑变量为什么分配在堆上、GC 压力大 | 逃逸分析 | `go build -gcflags='-m'` |
| 程序慢、疑似 goroutine 泄漏 | pprof | `go tool pprof` / `net/http/pprof` |

- **delve（dlv）**：Go 的调试器，`dlv test` 可以在测试里设断点、单步、查看变量；IDE 的调试功能底层也是它。
- **逃逸分析（escape analysis）**：`go build -gcflags='-m' ./...` 会报告变量"逃逸到堆上"的位置——栈分配快且自动回收，逃逸分析帮助理解为什么某些写法会增加 GC 压力。深入的内存与 GC 分析留给后端部分的性能章节。
- **pprof**：CPU/内存/goroutine 三类 profile 是定位性能与 goroutine 泄漏的标准手段；HTTP 服务挂 `net/http/pprof` 即可在线采样。

## 踩坑清单

### 先照搬大而全的骨架，再回头找职责

```text
project/
  cmd/
  internal/
  pkg/
  api/
  scripts/
  deployments/
  docs/
```

现象：仅把目录建齐，无法说明设计是否合理。原因：把社区模板当成了设计本身，目录是抄来的而不是从需求长出来的。修复：逐项回答"现在为什么需要这个目录"——写明现有入口、业务和存储的位置，没有使用理由的目录不创建；每个保留目录都要能对应具体源码与调用方，不用目录数量作为验收指标。

### 简单示例过早拆成多层抽象

```go
// 反例：只是一个 hello-style topic，却提前拆 controller/service/repository。
func Run() {
	controller.Handle(service.New(repo.New()))
}
```

现象：主题还只是练一个语法点，这种抽象只增加阅读成本——每一层都只是转发，读者要跳三个文件才能看到真正干活的代码。原因：把"分层"当成目的而不是手段，层的存在没有对应的职责。修复：在独立副本中缩短无实际职责的转发；需要分层时参考 json-api 的薄入口与内部包，重构前后 POST /api/echo 的状态码、字段和失败路径保持一致，测试仍能独立调用业务。

### 用字符串匹配判断错误类型

```go
// 反例：判断错误靠匹配文案
if strings.Contains(err.Error(), "config not found") {
	// ...
}
```

现象：一开始能工作，某天有人把错误文案改成 `"missing config"`，这处判断静默失效——不报错，只是逻辑再也不进分支。原因：`err.Error()` 是给人看的文案，可以随时改；程序间的契约应该是哨兵错误或类型。修复：定义 `ErrConfigNotFound` 哨兵，产生侧用 `%w` 包装（见 uber-errors-wrap），消费侧用 `errors.Is(err, ErrConfigNotFound)` 判定。

### 没有 benchmark 就先优化

现象：听说 `strings.Builder` 快，于是拼接两个字符串也建 Builder；听说预分配好，于是给固定 3 个元素的切片也算容量。代码变绕了，性能没有可测量的变化。原因：优化的收益只在热路径上显著，凭感觉优化等于把阅读成本花在看不到收益的地方。修复：先写直白正确的版本；怀疑慢就 `go test -bench=. -benchmem` 建基线，改动后对比 `ns/op` 与 `allocs/op`；收益不显著就回退。规范类写法（命名、early return、错误包装）不需要这个门槛——它们的收益在协作成本上，不在纳秒上。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 11.1 补一项延后目录职责 | 在 ProjectRoles 返回的切片中新增一项 deferred 记录，填写职责与引入条件，避免重复已有条目；测试中检查新条目的状态与两项说明 | 新增条目状态为 deferred，职责与"何时引入"说明完整；`FindRole` 能按路径查到它，`CurrentRoles` 的输出不包含它 | `project-layout/main.go`、`main_test.go` |
| 11.2 把单包 API 拆成入口和内部实现 | 参考 json-api/cmd，入口只保留依赖装配和 Run 调用；从 practice-json-api 抽出业务函数和 handler，保留原测试断言；先对比两种布局再实际编译、运行，不直接导入另一个 package main | 分层后的 demo 仍输出 POST /api/echo -> 200，业务不堆在入口；handler 的空消息 400、错误 JSON 400、非 POST 405 三条失败路径与单包版一致 | 练习副本的 `cmd/main.go`、`internal/jsonapi/` |

完成后运行受影响示例及测试：

```bash
go -C labs test ./part1-golang/chapter11/code-review-comments/... -count=1
go -C labs test ./part1-golang/chapter11/effective-go/... -count=1
go -C labs test ./part1-golang/chapter11/json-api/... -count=1
go -C labs test ./part1-golang/chapter11/module-layout/... -count=1
go -C labs test ./part1-golang/chapter11/practice-json-api/... -count=1
go -C labs test ./part1-golang/chapter11/project-layout/... -count=1
go -C labs test ./part1-golang/chapter11/review-cli/... -count=1
go -C labs test ./part1-golang/chapter11/uber-errors-wrap/... -count=1
go -C labs test ./part1-golang/chapter11/uber-slice-preallocate/... -count=1
go -C labs test ./part1-golang/chapter11/uber-string-builder/... -count=1
```

已有测试通过只说明已覆盖的用例符合预期；练习中新增的边界仍要补充断言。更多练习（为自己的项目写职责表等）见各 lab 的 README。

## 链接

- [Effective Go](https://go.dev/doc/effective_go)：Go 官方的写法规范，命名、错误处理与包组织的权威依据。
- [Go Modules 参考：项目布局](https://go.dev/doc/modules/layout)：module 与目录布局的官方说明。
- [cmd/go 文档：Internal Directories](https://pkg.go.dev/cmd/go#hdr-Internal_Directories)：`internal/` 导入限制的官方定义，看清工具链强制的确切边界。
- [Uber Go Style Guide](https://github.com/uber-go/guide)：uber-errors-wrap 等三个示例来源的工程规范，每条规则附正反例。
- [pprof 官方文档](https://pkg.go.dev/net/http/pprof)与 [Delve](https://github.com/go-delve/delve)：性能分析与调试工具入口。
- The Way to Go 中文章节：[自定义包和可见性](https://learnku.com/docs/the-way-to-go/95-custom-packages-and-visibility/3629)、[使用 godoc 查看自定义包](https://learnku.com/docs/the-way-to-go/96-uses-godoc-for-custom-packages/3631)、[自定义包目录结构、go install 与 go test](https://learnku.com/docs/the-way-to-go/98-custom-package-directory-structure-go-install-and-go-test/3633)、[可见性模式](https://learnku.com/docs/the-way-to-go/173-the-visibility-pattern/3728)。

[返回全书目录](../README.md) · [上一篇：Web 服务](chapter10.md) · [下一篇：MySQL、SQL、索引与 GORM](../part2-backend/chapter12.md)
