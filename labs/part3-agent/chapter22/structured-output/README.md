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

- Book chapter: `book/part3-agent/building/chapter22-architecture.md`; the `Provider` shape mirrors
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
