# 示例：structured-output

## Goal

Make a provider return JSON you can actually depend on: define a schema
(answer + confidence + sentiment enum + citations), parse and validate with
a minimal hand-rolled checker (presence, types, enum, range), surface bad
output as a visible structured error, and retry once with the failure fed
back to the provider. Fully offline; no model key needed.

Non-goals: a full JSON Schema engine, streaming partial JSON repair, and
provider error retries (provider errors propagate directly — different
failure semantics).

## Prerequisites

- [模型接入的Provider与消息契约](../hello-llm/README.md#provider与消息接口)。

- Book chapter: `content/book/part3-agent/building/chapter22-architecture.md`; the `Provider` shape mirrors
  `hello-llm` but is deliberately duplicated so labs stay self-contained.

## Concepts

- `Answer` schema with an enum field and a bounded `confidence` — validation
  checks presence, JSON types, enum membership, range, and rejects unknown
  fields.
- `StructuredError` separates `invalid_json` (syntax) from
  `schema_violation` (field problems) and keeps the raw output, so failures
  are observable instead of swallowed into a zero value.
- `GenerateAnswer`: on validation failure, appends the bad output plus the
  issue list as new messages and retries once; `Result.Attempts` makes the
  retry observable.
- `FakeProvider` replays scripted outputs and records every request, so
  tests cover valid output, bad JSON, schema violations, and retry-then-
  succeed paths deterministically.

## Run

```bash
gofmt -l labs/part3-agent/chapter22/structured-output
go -C labs vet ./part3-agent/chapter22/structured-output
go -C labs test ./part3-agent/chapter22/structured-output -race -count=1
go -C labs test ./part3-agent/chapter22/structured-output -cover
go -C labs build -o /tmp/structured-output ./part3-agent/chapter22/structured-output
go -C labs run ./part3-agent/chapter22/structured-output
```

Expected output (the scripted fake fails validation once, then succeeds):

```text
accepted after 2 attempt(s)
answer: 循环变量每轮新建
confidence: 0.90 sentiment: positive
citations: go.dev/doc/go1.22
```

## Code Map

- [main.go](main.go): `Answer` schema, `parseAnswer` validator,
  `StructuredError`, scripted `FakeProvider`, retrying `GenerateAnswer`, demo.
- [main_test.go](main_test.go): valid output, syntax error, six schema
  violation cases, retry-then-succeed (asserting the feedback messages), and
  giving up after the bounded retry.

## Variation Exercises

- Add a `language` enum field to `Answer`, extend `parseAnswer`, and add a
  violation case to `TestParseAnswerSchemaViolations`.
- Script the fake with two bad outputs and assert `GenerateAnswer` stops
  after exactly two provider calls (no unbounded retry).


## Sources

Original code for this course, MIT licensed. No third-party code.

## 输出与取消边界

GenerateAnswer最多调用2次，输出超过64 KiB直接失败，取消发生在调用前或返回后都终止。TestCanceledAndOversizeOutputRejected检查迟到结果不被接受、超大输出不进入修复循环。Raw保存的原文仅用于本地固定数据；真实数据反馈或日志需要脱敏与截断。

练习22.2：给Answer增加language枚举（zh/en），同步prompt与parseAnswer；补缺字段、错误枚举与合法输出测试，再验证有限修复。不要直接填入现成答案。

## 结构化结果的契约与有限修复

### 语法、字段与业务三层校验

假设下游需要回答、置信度、情感与引用列表。输出“像JSON”不够，必须分别检查语法与字段。配套的[Answer定义](main.go)为：

```go
type Answer struct {
	Answer     string   `json:"answer"`
	Confidence float64  `json:"confidence"`
	Sentiment  string   `json:"sentiment"`
	Citations  []string `json:"citations"`
}
```

应用把输出先解析为`map[string]any`，才能区分缺字段、类型错误、枚举错误与范围错误。对本章的少量固定字段，手写校验足以展示机制；需要嵌套对象和组合规则时，应使用经过验证的Schema实现，不能把这个实验当成完整JSON Schema引擎。

校验按三层推进：

1. **语法层**：是否为可解析的JSON，是否有多余尾部内容。
2. **契约层**：必填字段、类型、枚举、范围、额外字段是否符合约定。
3. **业务层**：引用是否真实存在，资料是否支持断言，任务是否完成。

`confidence: 0.9`只代表模型输出了一个合法数字，不代表答案有90%的正确概率；引用列表通过类型校验，也不代表资料支持答案。这些判断留给证据校验与Eval。

Provider提供的结构化输出约束可减少格式错误，但应用仍需处理拒绝、截断、契约差异与业务有效性。JSON模式与Schema约束能力也不能混为一谈，具体以对应Provider文档为准。

### 有界修复与反馈输入

遇到字段违约时，只重发原请求并没有补充新信息。应用可以将错误结果与具体问题一起反馈，配套实现保留这段修复消息：

```go
		messages = append(messages,
			Message{Role: "assistant", Content: raw},
			Message{Role: "user", Content: "Your output failed validation: " +
				strings.Join(serr.Issues, "; ") + ". Return only valid JSON matching the schema."},
		)
```

每次修复又发生一次模型调用，仍占用时间与费用。实验只允许首次调用加一次修复；超大输出直接拒绝，context取消直接终止，网络错误不归入格式修复。应避免把原始输出中的秘密或长文本直接反馈，真实系统可改用字段路径和稳定错误码。
