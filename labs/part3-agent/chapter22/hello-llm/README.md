# 示例：hello-llm

## Goal

Understand the minimal LLM call chain in Go: the message model (system /
user / assistant roles), a `Provider` interface with a deterministic offline
fake, an OpenAI-compatible HTTP adapter tested against `httptest`, and
token/cost bookkeeping. Real provider calls stay behind the explicit
`E2E_LIVE` gate and never run in default tests.

Non-goals: streaming, tool calling, and retry policies — those belong to
later chapters.

## Prerequisites

- Book chapter: `content/book/part3-agent/building/chapter22-architecture.md`.
- Go toolchain per `labs/go.mod`; all labs share that single module.

## Concepts

- `Message` + `Role`: system carries stable rules, user carries the current
  request, assistant carries prior model output.
- `Provider` interface: callers depend on `Generate(ctx, []Message)`, so the
  fake and the HTTP adapter are interchangeable without caller changes.
- `FakeProvider`: deterministic rule-based replies with estimated token
  usage, so tests assert exact output offline.
- `OpenAICompatibleProvider`: POSTs OpenAI-style `/chat/completions` with a
  timeout, a response size cap, and `HTTPError` so callers can branch on the
  status code.
- `estimateCost`: hardcoded example prices, clearly marked as not a real
  bill.

## Run

```bash
gofmt -l labs/part3-agent/chapter22/hello-llm
go -C labs vet ./part3-agent/chapter22/hello-llm
go -C labs test ./part3-agent/chapter22/hello-llm -race -count=1
go -C labs test ./part3-agent/chapter22/hello-llm -cover
go -C labs build -o /tmp/hello-llm ./part3-agent/chapter22/hello-llm
go -C labs run ./part3-agent/chapter22/hello-llm
```

Expected output:

```text
reply: pong
tokens: prompt=10 completion=2
estimated cost: $0.00000270 (example price, not a real bill)
```

`TestE2ELiveProvider` skips by default. To run it explicitly (real cost,
never in default CI):

```bash
E2E_LIVE=1 LLM_BASE_URL=https://your-provider.example.com LLM_API_KEY=... LLM_MODEL=... \
  go -C labs test ./part3-agent/chapter22/hello-llm -run TestE2ELiveProvider -count=1
```

## Code Map

- [main.go](main.go): message model, `Provider` interface, `FakeProvider`,
  `OpenAICompatibleProvider`, cost estimation, and the demo `main`.
- [main_test.go](main_test.go): fake determinism, httptest-backed success /
  HTTP 500 / timeout paths, and the E2E gate that skips by default.

## Variation Exercises

- Add a fake rule (e.g. messages containing "weather" get a fixed reply) and
  extend `TestFakeProviderDeterministic` to assert it.
- Add a 429 test case to the httptest server and assert `HTTPError.StatusCode`
  lets the caller distinguish rate limiting from a 500.


## Continue

- [structured-output](../structured-output): make the provider return JSON,
  validate it against a schema, and retry with feedback on visible failures.

## Sources

Original code for this course, MIT licensed. Request/response shape follows
the OpenAI chat completions API conventions.

## 调用边界与故障定位

请求最多64 KiB，文本生成请求发送max_tokens=256，响应默认最多1 MiB。非JSON的429/502仍返回HTTPError；200但坏JSON、无choices、空文本或finish_reason=length返回协议错误。超时保留底层错误链。字段属于本adapter的Chat Completions方言，接入服务时核对支持情况。

新增响应边界测试TestProviderResponseBoundaries；TestFakeHonorsCancellation检查开始前取消。默认测试不需要密钥。真实E2E只验证一次受限调用，不代表模型效果或最终账单。

练习22.1：本地服务器返回429，分别使用JSON和纯文本错误体，断言errors.As保留HTTPError及429。随后说明哪些失败可以重试，并给出总尝试预算。

<a id="book-ch22-llm-api-最佳实践"></a>

## 最佳实践

## 最佳实践

### 先验证协议，再比较模型

先用fake验证消息和业务分支，用httptest验证adapter，再显式启用真实调用。fake返回正确结构，只能证明程序按契约工作；模型换了版本、采样参数或输入分布，回答质量仍需独立评估。

### 记录消耗时标明数据来源

真实usage来自Provider响应，本地字符估算只用于教学趋势。费用计算还依赖价格版本、缓存、失败请求和其他计费项；不能将示例价格转换结果当作实际账单。调用失败却没有usage时，应保留“消耗未知”，不能凭空填零。

### 不在Provider里悄悄重试

adapter保留失败原因，Runtime明确决定能否重试、次数、退避和预算。结构修复属于输出契约处理，也要记录尝试次数，避免同一失败被多个层次重复重试。


<a id="book-ch22-llm-api-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 429被显示成JSON解析失败

先看HTTP状态，再看错误体。若错误体是HTML或纯文本，修复分类顺序；用同状态、不同格式的本地响应验证HTTPError仍保留429。不要先调提示词。

### JSON能解析，业务却拿到空字段

直接反序列化到结构体可能把缺字段与零值混在一起。先检查必填字段是否存在，再检查类型和范围；拒绝违约输出，而非吞掉错误继续。输出格式问题解决后，才检查引用与事实。

### 取消后页面仍显示答案

检查取消是否进入HTTP请求，以及返回后是否再次检查context。模型成功返回与用户仍需要结果不是同一条件；丢弃迟到结果，但保留已知消耗。取消不能保证Provider撤销已完成计费。
