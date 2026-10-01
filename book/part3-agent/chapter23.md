# 第 23 章：Tool Calling 与最小 Agent Loop

模型从不执行工具——它只输出一段结构化文本说"我想调用 `calculator`，参数是这些"；决定这次调用能不能发生、参数合不合法、结果回填到哪里的，是你的应用。本章把这个边界落成两个可运行的 lab：[tool-calling](../../labs/part3-agent/chapter23/tool-calling) 负责注册表与参数校验，[minimal-agent-loop](../../labs/part3-agent/chapter23/minimal-agent-loop) 负责把"模型 → 工具 → 观察 → 模型"的循环跑起来。

## 工具调用协议：模型提议，应用执行

一次工具调用在消息层面长这样（示意，字段名按 OpenAI / Anthropic 两家协议做了归并）：

| 消息 | 角色 | 内容 |
| --- | --- | --- |
| 用户提问 | `user` | `"6+7 等于几？现在几点？"` |
| 模型提议调用 | `assistant` | `tool_calls: [{id: "c1", name: "add", arguments: {"a":6,"b":7}}]` |
| 应用回填结果 | `tool` | `tool_call_id: "c1"`, `content: "13"` |
| 模型给最终答案 | `assistant` | `"6+7=13"` |

这张表就是整个协议：**`assistant` 消息里的 `tool_calls` 只是意图，`tool` 消息是应用执行后回填的观察（observation）**。模型侧看到的永远是文本；它无法区分"工具真的执行了"和"有人编了一段 tool 消息"。这意味着两件事：

- 执行权完全在应用一侧，协议本身不携带任何权限语义；
- `arguments` 是模型生成的 JSON 文本，属于不可信输入——和网页内容、用户输入同级，执行前必须校验。

[minimal-agent-loop](../../labs/part3-agent/chapter23/minimal-agent-loop) 用三个类型固定这个协议形状：

```go
// 摘录自 minimal-agent-loop/main.go。
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

type Response struct {
	ToolCalls []ToolCall
	Final     string
}

type Model interface {
	Next(ctx context.Context, history []Message) (Response, error)
}
```

`Model` 接口抽象掉具体 Provider：输入完整历史，输出一轮响应——要么是要执行的工具调用，要么是最终答案。测试用脚本化的 `ScriptedModel` 固定输出，避免把概率波动当成回归；这是 Agent 测试的第一条纪律。

## 注册表与参数校验：未知工具拒绝，坏参数不执行

[tool-calling](../../labs/part3-agent/chapter23/tool-calling) 把执行侧收进一个注册表。工具是四元组：名字、描述（给模型看）、参数 schema、handler（真正干活）：

```go
// 摘录自 tool-calling/main.go。
type Tool struct {
	Name        string
	Description string
	Parameters  Schema
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}
```

`Registry.Call` 是唯一的执行入口，校验顺序固定：先查表，再验参数，最后才轮到 handler：

```go
// 摘录自 tool-calling/main.go。
func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownTool, name)
	}
	if err := validateArgs(tool, args); err != nil {
		return "", err
	}
	return tool.Handler(ctx, args)
}
```

这个顺序区分了两层失败，调用方必须能把它们分开：

| 情况 | 返回 | handler 是否执行 |
| --- | --- | --- |
| 工具名不在注册表 | `ErrUnknownTool`，消息里带工具名 | 否 |
| 缺必填参数 / 类型不符 / 未声明参数 | `ErrInvalidArguments`，消息指明工具名、参数名、期望与实际类型 | 否 |
| handler 内部失败（除零、未知运算符） | handler 自己的错误 | 是 |

前两层是**协议校验失败**，说明模型的输出不合规；第三层是**业务执行失败**，参数形状合法但语义不成立。混在一起会让上层无法判断该让模型修正参数还是换方案——测试里用一个计数 handler 断言了"坏参数时 handler 一次都没有运行"。

运行示例（仓库根目录执行）：

```bash
go -C labs run ./part3-agent/chapter23/tool-calling
```

```text
registered=[calculator get_time] calculator=42 get_time=2026-01-01T08:00:00Z rejected="tool calling: unknown tool: \"delete_database\""
```

`get_time` 通过注入时钟保持确定性，demo 和测试都不依赖真实时间；`delete_database` 这行演示的是安全边界的关键动作：模型提议了注册表之外的名字，应用直接拒绝，而不是"想办法执行"。

## 最小 agent loop：跑到答案、上限或取消

有了协议形状和执行边界，loop 本身反而朴素。[minimal-agent-loop](../../labs/part3-agent/chapter23/minimal-agent-loop) 的 `Agent.Run` 只有一条主链：问模型 → 有工具调用就执行并回填 → 再问，直到模型给出 `Final`：

```go
// 摘录自 minimal-agent-loop/main.go。
session.Add(Message{Role: "assistant", ToolCalls: resp.ToolCalls})
for _, call := range resp.ToolCalls {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	observation, err := a.Tools.Execute(ctx, call)
	content := observation
	if err != nil {
		// 工具失败不中断 loop：错误序列化成 observation 回填，
		// 让模型自己看到失败并决定重试、换工具或放弃。
		content = "error: " + err.Error()
	}
	session.Add(Message{Role: "tool", ToolCallID: call.ID, Content: content})
}
```

一个 loop 有三种停法，每种都有不同的可观察现象和调用方选择：

| 停法 | 可观察现象 | 传播方式 | 调用方选择 |
| --- | --- | --- | --- |
| 模型给出最终答案 | `Run` 返回 `Result{Answer, Steps}` | 正常返回 | 使用答案，session 可保留续聊 |
| 达到 `MaxSteps` | `ErrMaxStepsExceeded` | `Run` 返回错误 | 增大上限，或检查模型是否陷入循环 |
| 整体超时 / 外部取消 | `context.DeadlineExceeded` / `context.Canceled` | `Run` 返回 context 错误 | 超时调大预算或排查慢工具；取消是正常关停 |

注意表里**没有**"工具失败"这一行。工具失败不是 loop 的停法：错误被序列化成 `tool` 消息回填，模型在下一轮能读到 `error: ...`，由它决定重试、换工具还是如实告知用户。如果 loop 替模型掩盖失败（直接中断或吞掉错误），模型就永远学不到"刚才那次调用没成功"，只会在原地打转。测试 `TestToolFailureFedBackAsObservation` 固定了这条路径：一个 handler 报错、一个工具名未注册，两次失败都以 observation 进入历史，loop 正常跑到最终答案。

`MaxSteps` 和 context 超时是两条独立的防线，缺一不可：`MaxSteps` 防的是模型行为失控（来回调用不收敛），超时防的是单次调用或整个会话挂死。两者都是调用预算，直接对应费用和延迟。

```bash
go -C labs run ./part3-agent/chapter23/minimal-agent-loop
```

```text
answer="6+7=13；当前时间 2026-01-01T08:00:00Z" steps=2 messages=5
```

阅读顺序：先看 main.go 的 `Message`/`ToolCall`/`Response` 类型 → `Agent.Run` 主循环 → `ScriptedModel`，再看 main_test.go 里正常完成、max steps、超时、取消、工具失败回填五类断言。

## 会话状态：消息历史就是全部状态

loop 没有隐藏状态——`Session` 里按序追加的消息就是全部：

```go
// 摘录自 minimal-agent-loop/main.go。
type Session struct {
	Messages []Message
}

func (s *Session) Add(m Message) {
	s.Messages = append(s.Messages, m)
}

// History 返回副本，外部修改不会写回内部历史。
func (s *Session) History() []Message {
	return append([]Message(nil), s.Messages...)
}
```

两个设计细节值得注意：

- **只追加，不改写**。每轮把 assistant 的提议和 tool 的观察都留下来，模型下一轮才能看到自己之前做过什么。这条性质直接带来可恢复性：把 `Messages` 序列化存盘，下次加载后继续 `Run`，会话就接着走。
- **`History()` 返回副本**。模型（或任何消费者）拿到的是快照，外部修改不会污染内部历史——和第 28 章 store 入库时复制向量是同一个防御动作。

多步工具调用的排错也落在这里：Agent 行为异常时，先打印 session 的角色序列和每条 tool 消息内容，多数"模型犯傻"其实是 observation 没回填、回填错了 `ToolCallID`，或历史被中途截断。

## 安全边界：权限由注册表决定，不由 prompt 决定

"我在 system prompt 里写了不许删除数据"不是权限控制。模型输出不可信，而工具返回的内容同样不可信——observation 里可能混着网页文本或第三方数据，其中的指令（prompt injection）会被模型当成新命令。所以强制执行点只能在代码里：

- **注册表就是权限清单**。能发生什么动作，取决于 `Register` 了什么，不取决于 prompt 怎么写。未知名字在 `Call` 入口直接拒绝，测试必须覆盖这一支。
- **不可逆工具不进注册表**。预订、支付、发送消息这类动作当前一律不注册；Agent 默认只读。
- **校验在 handler 之前**。schema 保证形状（类型、必填、无多余字段），handler 仍要校验语义（除零、非法运算符）——两层都不能省。
- **预算也是安全边界**。`MaxSteps`、超时、消息历史上限共同限制失控时的最大损失。

## 踩坑清单

### 把模型给的工具名直接当 map key 执行

```go
// 反例：不查注册表，模型说什么就执行什么。
handler := handlers[call.Name]
result, _ := handler(call.Args)
```

现象：模型幻觉出一个不存在的工具名时，轻则 nil handler panic，重则（如果 map 里恰好有同名内部函数）执行了从未打算暴露的操作。修复：统一走 `Registry.Call`，查不到就返回结构化的 `ErrUnknownTool`；测试断言未知名字被拒绝且 handler 未执行。

### 参数不校验就交给 handler

```go
// 反例：handler 里直接类型断言，模型给了字符串就 panic。
a := args["a"].(float64)
```

现象：模型把 `{"a": "6"}` 这样的字符串数字传进来，运行期 panic。修复：schema 校验通过前 handler 不运行，错误信息带工具名、参数名、期望与实际类型（`argument "a": expected number, got string`），这样的 observation 回填后模型才有足够信息自我修正。

### 工具一失败就中断 loop

```go
// 反例：把工具错误当 loop 错误返回，模型永远不知道发生了什么。
if err != nil {
	return Result{}, err
}
```

现象：一次除零或一次未知工具就让整个会话失败，重试还是同样的剧本。修复：错误序列化成 `"error: ..."` 写进 tool 消息，loop 继续；用脚本化模型断言下一轮历史里能看到失败内容，且最终答案照常产生。

### 没有步数上限和超时

现象：模型在两个工具之间来回振荡，或某个工具调用挂死，会话无限制烧 token、占连接。修复：`MaxSteps` 和 `context.WithTimeout` 同时设置并分别测试——断言超限返回 `ErrMaxStepsExceeded` 且模型恰好被调用了上限次，断言超时后 `Run` 迅速返回 `context.DeadlineExceeded` 而不是等慢工具跑完。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 23.1 注册第三个工具 | 给 tool-calling 增加 `string_length`：一个必填 string 参数，返回长度；含注册、schema 和测试 | 正常调用返回长度；传入数字被拒绝，错误信息含 `expected string` 且 handler 未执行 | `tool-calling/main.go`、`main_test.go` |
| 23.2 预测 max steps 中断点 | 把 minimal-agent-loop 的 `MaxSteps` 调为 1，模型首轮回复是工具调用；先预测 `Run` 的返回值与 session 内容，再写测试断言 | 返回 `ErrMaxStepsExceeded`；session 中已留有首轮回填的 observation，说明中断发生在执行之后、下一轮提问之前 | `minimal-agent-loop/main_test.go` |

完成后运行：

```bash
go -C labs test ./part3-agent/chapter23/... -race -count=1
```

更多练习（除零与未知运算符的错误分层、同一工具连续两轮的 observation 顺序）见各 lab 的 README。

## 链接

- [pkg.go.dev/context](https://pkg.go.dev/context)：loop 超时与取消的官方语义，特别是 `Err()` 与 `DeadlineExceeded`/`Canceled` 的区分。
- [pkg.go.dev/encoding/json](https://pkg.go.dev/encoding/json)：工具参数以 JSON 文本到达，解码成 `map[string]any` 后数字一律是 `float64`——本章 schema 校验接受 `int` 正是为了兼容手工构造的调用方。
- [OpenAI Function Calling 指南](https://platform.openai.com/docs/guides/function-calling) 与 [Anthropic Tool Use 文档](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/overview)：两家主流协议的 `tool_calls` / `tool_result` 字段定义，可对照本章的归并示意。

[返回全书目录](../README.md) · [上一篇：LLM API、Message 与结构化输出](chapter22.md)
