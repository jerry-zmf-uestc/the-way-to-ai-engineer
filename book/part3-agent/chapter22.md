# 第 22 章：LLM API、Message 与结构化输出

调用 LLM 的工程难点不是发出一次 HTTP 请求，而是把消息结构、Provider 边界和失败语义固定成不依赖密钥也能测试的代码。本章两个 lab 全程离线：`hello-llm` 固定消息模型与调用边界，`structured-output` 固定结构化输出的校验与重试；真实 Provider 只在显式开关后出现。

## 1. 消息模型：system、user、assistant 各管什么

一次调用的输入是一个消息列表，三种角色职责不同：

- **system**：稳定规则，由应用开发者拥有——输出格式、语气、约束。它在多次调用间复用，不放本次请求的参数。
- **user**：本次请求的参数，通常来自最终用户输入。
- **assistant**：模型的历史回复。多轮对话和结构化输出重试时，由应用把模型上一轮输出回传。

```go
// 摘录自 hello-llm/main.go。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}
```

角色是约定，不是安全边界：所有消息最终拼成文本给模型，用户输入可能包含诱导模型无视 system 规则的内容。**权限和副作用的强制执行点在应用层，不在 prompt 里**——模型输出永远是不可信输入。

## 2. Provider 接口：为什么先写 fake

```go
// 摘录自 hello-llm/main.go。
type Provider interface {
	Generate(ctx context.Context, messages []Message) (Response, error)
}
```

接口只有这一个方法，但先写 `FakeProvider` 再写 HTTP 适配器是有顺序的：fake 确定性、离线、无费用，先把「消息进去、Response 出来、token 有记录」这条链路钉死，调用方代码在 fake 和真实实现之间不需要改变。fake 必须实现同一个接口签名，否则接真实 Provider 时要连调用协议一起改。

```go
// 摘录自 hello-llm/main.go。
type FakeProvider struct{}

var _ Provider = FakeProvider{}

func (FakeProvider) Generate(_ context.Context, messages []Message) (Response, error) {
	promptTokens := 0
	lastUser := ""
	for _, m := range messages {
		promptTokens += estimateTokens(m.Content)
		if m.Role == RoleUser {
			lastUser = m.Content // 取最后一条 user 消息作为本次请求参数
		}
	}
	if lastUser == "" {
		return Response{}, errors.New("hello-llm: no user message")
	}

	reply := "fake reply: " + lastUser
	if strings.Contains(lastUser, "ping") {
		reply = "pong"
	}
	return Response{
		Text: reply,
		Usage: Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: estimateTokens(reply),
		},
	}, nil
}
```

`var _ Provider = FakeProvider{}` 让编译器检查方法集，签名漂移在编译期暴露。相同输入永远产生相同输出，测试可以断言具体内容而不是「包含某些字」。

## 3. HTTP 适配器与失败语义

`OpenAICompatibleProvider` 把同一个接口落到 OpenAI 风格的 `POST {BaseURL}/chat/completions` 上。外部调用必须带超时和响应大小上限——模型 API 可能慢、可能返回异常大的响应，这两个上限是调用预算的一部分：

```go
// 摘录自 hello-llm/main.go。
client := p.Client
if client == nil {
	client = &http.Client{Timeout: 30 * time.Second}
}
resp, err := client.Do(req)
if err != nil {
	return Response{}, fmt.Errorf("hello-llm: request: %w", err)
}
defer resp.Body.Close()

maxBytes := p.MaxBytes
if maxBytes <= 0 {
	maxBytes = 1 << 20
}
data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
// ...
if resp.StatusCode != http.StatusOK {
	// 包装成 *HTTPError，保留状态码供调用方分支
}
```

「调用失败」不是一种情况，至少要分开这些语义：

| 情况 | 可观察现象 | 传播方式 | 调用方选择 |
| --- | --- | --- | --- |
| 200 且 JSON 合法 | `Response` 带文本与服务端 usage | 正常返回 | 继续业务流程 |
| 非 200（500、429） | Provider 错误体 | `*HTTPError`，含 `StatusCode` | 429 限流可退避重试，5xx 视预算决定 |
| 超时 / 网络错误 | 无响应 | 包装的底层 error，**不是** `*HTTPError` | 检查超时配置与网络，不当作 Provider 拒绝 |
| 响应超过大小上限 | 读取被截断 | 普通 error | 调大上限或拒绝该 Provider |
| 200 但 JSON 非法 | 无法解析 | 普通 error | 记录原始响应，按 Provider 故障处理 |

测试用 `httptest` 服务器覆盖正常、500、超时三条路径，不需要真实 Provider。注意超时路径的断言：它必须**不是** `*HTTPError`，否则调用方会把网络问题误当限流处理。

## 4. token 与成本记录

`Response` 除了文本还带 `Usage{PromptTokens, CompletionTokens}`。真实 Provider 的 usage 由服务端返回，是计费依据；fake 的 usage 只能本地估算：

```go
// 摘录自 hello-llm/main.go。
// estimateTokens 用「约 4 个字符 1 个 token」的经验法则粗估，
// 只用于演示成本记录，真实计费以 Provider 返回的 usage 为准。
func estimateTokens(s string) int {
	return utf8.RuneCountInString(s)/4 + 1
}
```

成本估算的价格表硬编码在源码里，注释明确标注是示例价——它不是任何真实 Provider 的报价，只用来把「token 用量 → 钱」的量级感觉建立起来。在仓库根目录运行：

```bash
go -C labs run ./part3-agent/chapter22/hello-llm
```

```text
reply: pong
tokens: prompt=10 completion=2
estimated cost: $0.00000270 (example price, not a real bill)
```

## 5. 结构化输出：校验失败必须可见

让模型返回 JSON 只是第一步；真正的工程闭环是「schema 校验 → 可见失败 → 带反馈重试」。`structured-output` 期望模型返回答案、置信度、情感枚举和引用列表：

```go
// 摘录自 structured-output/main.go。
type Answer struct {
	Answer     string   `json:"answer"`
	Confidence float64  `json:"confidence"`
	Sentiment  string   `json:"sentiment"`
	Citations  []string `json:"citations"`
}
```

`parseAnswer` 先解析进 `map[string]any` 再逐项校验，而不是直接 `Unmarshal` 进结构体——直接反序列化时「类型不对」和「字段缺失」很难分开报告，而重试反馈需要逐项问题清单。校验覆盖字段存在、JSON 类型、枚举成员、取值范围，并拒绝未知字段。失败返回可见的结构化错误，保留原始输出：

```go
// 摘录自 structured-output/main.go。
type StructuredError struct {
	Kind   ErrorKind // invalid_json（语法）与 schema_violation（字段）分开
	Issues []string
	Raw    string
}
```

语法错误与 schema 违例要分开：前者说明模型没按格式输出，后者说明格式对了但内容违约，两者的 prompt 修复策略不同。校验失败时，`GenerateAnswer` 把坏输出和问题清单作为新消息回传，最多重试一次：

```go
// 摘录自 structured-output/main.go。
messages = append(messages,
	Message{Role: "assistant", Content: raw},
	Message{Role: "user", Content: "Your output failed validation: " +
		strings.Join(serr.Issues, "; ") + ". Return only valid JSON matching the schema."},
)
```

三个设计决定值得注意：重试次数有硬上限（2 次），结果里的 `Attempts` 让重试可观察；Provider 自身的错误（网络、配额）直接上抛不参与重试，那是上一节的失败语义；重试仍失败时返回包裹了 `StructuredError` 的错误，调用方拿到的是完整证据而不是零值。运行：

```bash
go -C labs run ./part3-agent/chapter22/structured-output
```

```text
accepted after 2 attempt(s)
answer: 循环变量每轮新建
confidence: 0.90 sentiment: positive
citations: go.dev/doc/go1.22
```

## 6. E2E 门控：真实 Provider 只在显式开关后

`hello-llm` 的 `TestE2ELiveProvider` 默认 `t.Skip`，只有同时显式设置 `E2E_LIVE=1`、`LLM_BASE_URL` 和 `LLM_API_KEY` 才会发出真实请求：

```go
// 摘录自 hello-llm/main_test.go。
if os.Getenv("E2E_LIVE") == "" {
	t.Skip("set E2E_LIVE=1 with LLM_BASE_URL and LLM_API_KEY to run against a real provider")
}
```

这不是可有可无的仪式感：默认测试进 CI，真实调用会产生费用、依赖网络、可能把密钥写进日志。密钥只从环境变量注入，不进源码、不进测试快照、不进错误信息。离线 fake 与 httptest 测试通过只证明本地协议正确，不证明真实模型效果——两件事要分开验证、分开记录。

## 踩坑清单

### 把所有指令都塞进 system

```text
反例：system = "你是助手。用户问：{question}。顺便，如果用户提到天气就……"
```

现象：稳定规则和本次参数混在一起，多轮对话时 system 每轮都要重拼，缓存失效，也难以判断哪条指令生效。修复：system 只放跨调用复用的规则，本次请求的参数放 user 消息。

### 坏 JSON 吞掉继续

```go
var out Answer
_ = json.Unmarshal([]byte(raw), &out) // 反例：忽略错误
return out, nil
```

现象：模型返回了一段散文，下游拿到零值 `Answer{}`，表现为「模型说不知道」而不是「输出违约」，排查时先怀疑模型能力而非解析层。修复：解析失败返回带原始输出的可见错误（如 `StructuredError`），让失败在日志和返回值里都看得到。

### 重试原样重发

现象：第一次输出违约，第二次把同样的消息原样再发一遍，模型大概率再犯同样的错；没有上限的重试循环还会悄悄烧掉配额。修复：把坏输出和逐项问题清单反馈给模型（第 5 节），重试次数设硬上限，并把尝试次数记录在结果里。

### 默认测试访问真实 API

```go
func TestGenerate(t *testing.T) {
	key := os.Getenv("LLM_API_KEY")
	resp, _ := realProvider.Generate(context.Background(), msgs) // 反例：CI 里直接打真实服务
	_ = resp
}
```

现象：CI 因网络抖动随机失败，每次运行产生真实费用，密钥可能进构建日志。修复：默认测试用 fake 和 httptest；真实 Provider 只走 `E2E_LIVE` 显式门控，不进默认 CI。

### 把 token 估算当账单

现象：用「4 字符 ≈ 1 token」的估算值做成本报表，和 Provider 账单对不上。修复：估算只用来看量级和回归趋势，真实计费和限流以 Provider 返回的 usage 字段为准。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 22.1 区分限流与故障 | 给 httptest 服务器增加返回 429 的用例，断言 `errors.As` 得到 `*HTTPError` 且 `StatusCode` 为 429 | 新用例通过；429 与 500 断言路径不同，调用方能按状态码选择退避重试还是放弃 | `hello-llm/main_test.go` |
| 22.2 扩展输出 schema | 给 `Answer` 增加 `language` 枚举字段（`zh`/`en`），同步 `parseAnswer` 校验与 system prompt，并补 schema 违例用例 | 缺字段、错枚举各自产生可见 issue；合法输出与重试路径的测试仍然通过 | `structured-output/main.go`、`main_test.go` |

完成后运行：

```bash
go -C labs test ./part3-agent/chapter22/... -race -count=1
```

更多练习（fake 规则扩展、双坏输出验证重试上限）见各 lab 的 README。

## 链接

- [encoding/json](https://pkg.go.dev/encoding/json)：Unmarshal 的类型映射规则，理解为什么先解析进 `map[string]any` 才能逐项报告问题。
- [net/http](https://pkg.go.dev/net/http) 与 [net/http/httptest](https://pkg.go.dev/net/http/httptest)：Client 超时、Response 处理与本地测试服务器的官方文档。
- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat)：请求/响应形状与 usage 字段的参考定义，本章适配器按其风格构造请求。

[返回全书目录](../README.md) · [上一篇：配置、日志、探针与优雅关闭](../part2-backend/chapter21.md) · [下一篇：Tool Calling 与最小 Agent Loop](chapter23.md)
