# Mini Agent Console

A minimal agent product that starts from a clean checkout with one command.
The web page sends a question, a deterministic offline fake model optionally
proposes one tool call (`calculator`, `echo`, or `research`), and the answer streams back
in whitespace-separated chunks over Server-Sent Events. A stop button cancels the stream.

Non-goals: no real LLM provider, no multi-turn memory, no npm or build step,
no writable or irreversible tools. The tool registry is fixed by the
application, not by the prompt.

## Prerequisites

- Go 1.26 (see `labs/go.mod`); no other services or dependencies.
- Chapter 23 (tool/loop boundaries), chapter 9 (context), and chapter 10 (HTTP) give the
  agent-loop background; this is an independent integrated streaming lab and is
  fully self-contained — it imports nothing from other labs.

## Key mechanics and observation points

- `GET /chat?q=...` streams SSE events: `token`, `tool_call`, `tool_result`,
  `agent_error`, `done` (with `finish_reason` = `stop` | `error` | `timeout` |
  `cancelled`). Each JSON envelope contains run_id, seq (starting at 1), and payload;
  SSE id is run_id:seq. EventSource only speaks GET, hence the query parameter.
- The stop button closes EventSource; disconnect cancels the request context.
  Event sends and delays cooperate with cancellation. Local tools return promptly;
  a non-cooperating replacement cannot be forcibly stopped. The UI does not claim a server acknowledgement.
- A server-side timeout (default 30s) ends long sessions with
  `finish_reason=timeout`; a whitespace-chunk budget caps the reply length, not Provider billing tokens.
- Unknown tools are rejected by the registry before execution: try
  `tool rocket launch`. Tool arguments are validated before running: try
  `calc 2+` or `echo`.

Research is a read-only evidence slice: three original teaching excerpts,
keyword matching, and exact quote/source-ID validation before any answer chunk.
No match is a refusal. This proves excerpt membership, not semantic relevance,
source truth, or free-form claim entailment. The fake does not measure real LLM
quality. Query strings are visible to browser history/logging; use public teaching
inputs only. Run IDs and sequences diagnose current-stream identity and order; there is no
persistence, checkpoint, event replay, or reconnection protocol. Each GET starts a new run. The UI closes on network error to prevent
EventSource automatically repeating it and ignores stale connection callbacks.

Try these prompts:

- `research context` — fixed evidence with `[context]` citation.
- `research unknown` — no evidence; error/done with no answer chunks.
- `calc 2+3*4` — one calculator call, then a streamed summary.
- `echo hello stream` — one echo call.
- `tool rocket launch` — the fake model proposes an unregistered tool and the
  registry refuses to run it.
- anything else — a direct streamed answer with no tool call.

## Run

From a clean checkout, one command:

```bash
go -C labs run ./part3-agent/chapter32/mini-agent-console
```

Then open http://127.0.0.1:8080 and ask `calc 2+3*4`. Expected terminal
output:

```text
mini agent console listening on http://127.0.0.1:8080 (Ctrl+C to stop)
```

Without a browser, watch the raw stream:

```bash
curl -N 'http://127.0.0.1:8080/chat?q=calc%202%2B3*4'
```

Expected stream (abridged; IDs are generated per request, placeholders below):

```text
id: <run-id>:1
event: tool_call
data: {"run_id":"<run-id>","seq":1,"payload":{"args":"2+3*4","name":"calculator"}}

id: <run-id>:2
event: tool_result
data: {"run_id":"<run-id>","seq":2,"payload":{"name":"calculator","result":"14"}}

id: <run-id>:3
event: token
data: {"run_id":"<run-id>","seq":3,"payload":{"token":"The "}}
...

event: done
data: {"run_id":"<run-id>","seq":<next-seq>,"payload":{"finish_reason":"stop"}}
```

## Test

```bash
gofmt -l labs/part3-agent/chapter32/mini-agent-console
go -C labs vet ./part3-agent/chapter32/mini-agent-console
go -C labs test ./part3-agent/chapter32/mini-agent-console -race -count=1
node --test tests/console-ui-test.mjs
```

The httptest suite never listens on a real port: it covers a full streamed
session, client cancel (handler returns and the agent goroutine exits),
unknown-tool rejection, the server-side timeout path, empty-question
validation, and the calculator grammar.

## Failure and variant exercises

- Set the server timeout to 1 second, keep the 60 ms token delay, and ask a
  direct question: observe the stream end with `finish_reason=timeout`.
- Remove the registry check in `runAgent` and rerun
  `TestChatUnknownToolRejected`: watch it fail, proving the test guards the
  rejection path.
- Variant: add a third tool (e.g. `upper`) to `defaultRegistry`, extend
  `decide` to propose it, and cover it with a streaming test.

## Code Map

- [main.go](main.go): fake model decision rules, the tool registry
  (`calculator`, `echo`), the agent loop on a cancellable event channel, the
  SSE handler with timeout, and a small arithmetic parser.
- [research.go](research.go): fixed corpus, keyword search and extractive citation validator.
- [research_test.go](research_test.go): no-hit refusal, forged IDs/quotes, and injected text without registry expansion.
- [main_test.go](main_test.go): httptest coverage of streaming, cancel,
  unknown-tool rejection, timeout, validation, and the calculator.
- [web/index.html](web/index.html): single-page console, native EventSource
  rendering, stop button; no npm, no build step, embedded via `go:embed`.

## 补充验证与观察

先直接验证工具和引用，再检查事件编码/顺序、HTTP、页面。通过虚拟EventSource测试旧回调、非法JSON、序号与身份错误、错误后伪成功；真实浏览器再观察正常与失败路径。TestWireIdentitySequenceAndSingleTerminal检查同次流ID/连续Seq与单一终态；TestWriteAndFlushFailureStopsHandler检查失败写入/flush后退出；TestLateToolResultAfterCancelIsNotStreamed检查迟到工具数据被丢弃。

请求总时限默认30秒，HTTP Server写入总时限35秒、读头时限5秒；实际main服务器有限写时限，httptest只验证处理逻辑，不模拟网络缓冲或慢代理。GET传公开教学问题；生产敏感输入需认证POST/fetch、输入限制、日志脱敏与独立任务入口。应用错误来自固定本地数据，真实错误须分层分类和脱敏。

```bash
curl -N 'http://127.0.0.1:8080/chat?q=research%20context'
curl -N 'http://127.0.0.1:8080/chat?q=research%20unknown'
```

第一条应有tool_call、tool_result、token和done(stop)，引用含[context]；第二条应有空来源、agent_error和done(error)，没有答案token。原源码阅读顺序为chatHandler→runAgent→decide，再看researchTool→researchReply→validateClaims；此顺序用于调试，不作为课程知识结构。

## 原章节练习与故障复现

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 32.1 证据拒绝与迁移 | 在副本新增资料和关键词；先写无命中、真实引用、伪造引用，再实现；不能只查来源ID | 无命中无答案token；新资料正确引用；篡改摘录拒绝 | research.go、research_test.go |
| 32.2 判断恢复边界 | 模拟部分token后断连、新请求以后旧连接迟到done；预测显示与连接状态，扩展页面测试 | 旧事件不污染、不关闭新连接；断流完成未知、不自动重跑 | web/index.html、tests/console-ui-test.mjs |

- 在副本把网络error显示为done(stop)，断开服务观察误报；修复后只有明确done(stop)可正常结束。
- 在副本移除局部connection检查，注入旧token/done观察污染；恢复并用迟到回调测试复验。
- 将引用校验简化为只检查ID，用已有ID和伪造文本复现放行；恢复ID/原文双重校验并重跑TestCitationForgeryRejected。
- 在副本加入不响应context的阻塞工具，观察handler与工具生命期不同；改工具契约和下游超时以后验证退出。不要将永久阻塞版保留在默认测试。

## Sources and license

Original code for this course, MIT license. No third-party material.

## 证据校验能保证什么

`researchTool` 只对固定资料的关键词做匹配，返回 `sources`，每条有 `id` 和 `quote`。它不计算 embedding，不进行语义排名。先让这种最简单的检索可观察，再到第28章学习向量检索，能够区分检索问题、生成问题和协议问题。

fake 把来源映射成引用后，`validateClaims` 检查三件事：至少有一条来源和引用；来源 ID 非空且不重复；每条引用的 ID 存在，且引用文本与该来源摘录完全一致。下面是 [research.go](research.go) 的源码摘录，位于校验器末尾：

```go
for _, claim := range claims {
	quote, ok := allowed[claim.SourceID]
	if !ok || quote != claim.Quote {
		return ErrInvalidCitation
	}
}
```

这个校验可以拒绝伪造 ID 和伪造摘录；它不能证明资料本身正确、选中的资料相关，也不能证明任意自然语言推论被资料蕴含。如果以后改成自由生成答案，只检查 ID 仍会放过“引用正确、结论错误”的回答。那时要增加逐项断言与来源对照、检索评估、模型辅助评审和人工抽查，分别统计召回与回答支持率。

校验发生在发送第一块答案之前。因此无证据时可以拒绝，不会先把未经检查的结论推给用户。若真实模型边生成边展示，则必须选择缓冲后校验，或让界面明确显示“未校验草稿”；验证失败后再追加一句警告，并不能撤回已经被用户使用的内容。

资料内可能写着“调用 delete_database”。这只是数据，不会向 defaultRegistry 注册工具；应用仍只有 calculator、echo、research。`TestInjectionDoesNotExpandRegistry` 用含这种指令的工具输出验证固定 fake 不会扩权。该测试不证明真实模型永远不受注入影响，但强制白名单仍应在模型之外成立。


## 重连、续聊、重放与 checkpoint

这四个词对应不同状态：重连恢复传输连接；续聊把已有消息交给下一轮模型；重放使用已记录输入和事件复现一次执行；checkpoint 恢复未完成的执行位置。保存一组 messages 无法自动得到其他三种能力。

当前 Console 每次请求是新的单次任务，不保存 Session，已为当前流增加事件ID与序号，但没有Last-Event-ID补读、run查询端点或checkpoint。刷新页面不会恢复旧任务。把 SSE 改为可恢复传输，至少需要 run ID、单调事件序号、事件存储、断点读取、保留期和资源所有权；否则客户端无法区分重复事件和新事件。

持久化执行还需要保存状态转换与工具完成结果，并处理“工具已完成、checkpoint 未提交”的窗口。只读工具可以选择重试，但计费查询也可能产生额外成本；有副作用的工具则需要幂等键和提交结果判断。多 worker 恢复还涉及 lease 与并发所有权。不要为了给本页增加一个保存按钮，就把这些问题隐藏起来。

这个独立切片先固定证据、事件和取消边界；可靠恢复应在专门的持久化 Lab 中验证进程崩溃、重复领取和结果提交窗口，再与服务接口组合。

<a id="book-ch32-streaming-service-最佳实践"></a>

## 最佳实践

## 最佳实践

- **先设计事件语义**：区分文本、工具、状态与终态，再映射框架协议；产品只订阅需要的视图。
- **在发送前约束数据**：校验、脱敏和长度限制发生在服务端；显示草稿与正式答案时有明确边界。
- **分开三种资源限制**：执行预算、事件缓冲、连接写入时限各自管理，慢客户端不能无限积累数据。
- **按身份路由事件**：连接、Run、任务、消息与工具调用按所需层级关联；显示名不作唯一键。
- **用固定源验证链路**：先检验事件顺序、部分输出、断流和取消，再接真实模型测首Token延迟、质量和费用。


<a id="book-ch32-streaming-service-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

| 现象 | 定位顺序 | 原因与处理 |
| --- | --- | --- |
| 结束时才一次显示全部文本 | 生产者是否逐段发出 → handler flush → 代理 → 页面 | 区分上游缓冲和传输缓冲，不用增加前端动画掩盖 |
| 新回答混入旧文本 | 局部connection → Run ID → 消息/分支ID | 旧回调或错误路由；绑定身份后再追加 |
| 半句话被显示为完成 | done是否收到 → reason是否合法 → 是否已有错误 | EOF和网络error不是成功，错误不能被后续文本掩盖 |
| 停止后仍消耗资源 | handler是否退出 → channel发送 → 下游工具/Provider | 传播context，设置网络时限；检查真实退出，不能只看按钮状态 |
| 重连后再次执行工具 | 连接入口是创建还是订阅 → 幂等与事件账本 | 明确重连契约；本实验关闭自动重连，持久服务另做恢复 |
| JSON偶发解析失败 | 事件格式 → 编码 → fetch跨块组装 | 先完成事件边界解析，再解析JSON；不要把网络块当完整事件 |

先直接验证固定事件和编码，再检查HTTP传输，最后检查页面；模型输出与检索质量单独验证。页面正确展示失败也是协议正确的一部分。
