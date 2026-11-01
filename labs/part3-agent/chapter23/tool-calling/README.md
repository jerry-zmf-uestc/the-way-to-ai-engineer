# 示例： tool calling

## Goal

Build the execution half of tool calling: a tool registry that owns tool
permissions, plus a schema check that runs before any handler. Non-goals: no
real LLM provider, no network access, and no full agent loop (that lives in
[minimal-agent-loop](../minimal-agent-loop)).

## Prerequisites

- Chapter 4 (functions, closures) and Chapter 5 (error handling) in Part 1.
- Chapter 9 (`context`) for the handler signature.

## Concepts

- The model only *proposes* a tool call; the registry decides whether it runs.
- `Registry.Call` is the single enforcement point: unknown tools are rejected
  with `ErrUnknownTool`, arguments failing the schema return
  `ErrInvalidArguments`, and in both cases the handler never runs.
- Schema validation covers missing required arguments, wrong types, and
  undeclared arguments; error messages name the tool, the argument, and the
  expected versus actual type.
- The two demo tools (`calculator`, `get_time`) are deterministic fakes:
  `get_time` takes an injected clock so tests never depend on wall time.

## Run

```bash
gofmt -l labs/part3-agent/chapter23/tool-calling
go -C labs vet ./part3-agent/chapter23/tool-calling
go -C labs test ./part3-agent/chapter23/tool-calling -race -count=1
go -C labs run ./part3-agent/chapter23/tool-calling
```

Expected output:

```text
registered=[calculator get_time] calculator=42 get_time=2026-01-01T08:00:00Z rejected="tool calling: unknown tool: \"delete_database\""
```

## Code Map

- [main.go](main.go): the schema subset, the registry with its single `Call`
  entry point, argument validation, and two deterministic demo tools.
- [main_test.go](main_test.go): covers successful calls, unknown-tool
  rejection, invalid arguments never reaching the handler, readable validation
  messages, handler-level semantic errors (division by zero, unknown op), and
  duplicate registration.

## Exercises

- Variant: register a third tool `string_length` that takes one required
  string argument; assert that a numeric argument is rejected and the error
  message reports `expected string`.
- Failure path: call `calculator` with `{"op": "divide", "a": 1, "b": 0}` and
  confirm the error comes from the handler, not from schema validation — the
  two failure layers stay distinguishable.

### 数值有效性与结果边界

ArgNumber接受int与有限float64，不把NaN、Infinity作为普通number传给Handler。TestNonFiniteNumbersCannotEnterHandler使用计数Handler验证拒绝发生在执行前，并验证MaxFloat64仍是合法输入。typeName对这类失败显示non-finite number，避免“expected number, got number”的诊断歧义。

合法输入仍可能计算溢出。四则运算统一使用formatFinite，非有限结果返回ErrNonFiniteResult与空文本；TestFiniteArgumentsDoNotMakeOverflowSuccessful覆盖加、减、乘、除的溢出，不把“+Inf”作为成功观察。除零仍在业务层拒绝，整数转float64的精度限制未因有限性检查而消失。

迁移练习：为需要精确整数的工具设计独立参数表示和范围；验证超出精确范围不能静默舍入。保留输入错误不执行与结果错误已调用的区别，不能只看最终字符串是否非空。

### 输入体积与成功结果

[payload.go](payload.go)和[payload_test.go](payload_test.go)补调用前/后的边界。注册名128字节、描述1024字节、最多32参数定义，键128字节且UTF8合法；Call同样检查名字、参数数/键、字符串4096字节及编码。原字段累计预检后才做类型与JSON编码，序列化参数最多16KiB。控制字符转义可能超过这个上限，不能只统计原始字符数。

```bash
go -C labs test ./part3-agent/chapter23/tool-calling -list 'TestToolPayload|TestToolSuccess|TestToolDefinitions'
go -C labs test ./part3-agent/chapter23/tool-calling -run 'TestToolPayload|TestToolSuccess|TestToolDefinitions' -race -count=1 -v
```

计数Handler验证大字符串、转义膨胀、坏UTF8、字段数/键/累计体积超限时零执行。4096字节合法字符串与结果仍通过；4097字节或非法UTF8成功结果返回ErrToolResult，调用次数保留。Handler返回text+error时不交付残余text，原error原因保持。取消仍优先；单次结果检查不能撤销下游读取、调用消耗或分配。

迁移练习：把大检索结果改为带来源、Complete和游标的结构化返回或Workspace引用，自行补部分结果与取消/超限测试；不直接裁剪字符串后返回nil。原string_length与业务错误练习保留，字节边界不替代Token、精度或完整任务评估。

## Sources & License

Original code for this course, MIT licensed. No third-party material.

## 注册表与参数校验：未知工具拒绝，坏参数不执行

[tool-calling](README.md) 把执行侧收进一个注册表。工具是四元组：名字、描述（给模型看）、参数 schema、handler（真正干活）：

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
func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := payloadFits(name, args); err != nil {
		return "", err
	}
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownTool, name)
	}
	if err := validateArgs(tool, args); err != nil {
		return "", err
	}
	if err := encodedArgsFit(args); err != nil {
		return "", err
	}
	result, err := tool.Handler(ctx, args)
	if stop := ctx.Err(); stop != nil {
		return "", stop
	}
	if err != nil {
		return "", err
	}
	if !resultFits(result) {
		return "", ErrToolResult
	}
	return result, nil
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


## 执行边界的反例

### 从未经筛选的 handler 集合直接执行工具

```go
// 故意失败的示意：未经授权筛选、也不检查 key 是否存在。
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


## 扩展练习：string_length

注册 `string_length`，只接受一个必填 string 参数，返回长度；先决定按字节还是 Unicode 码点计数，并写明契约。测试正常输入、数字输入、缺字段和多余字段。非法输入时错误应包含 `expected string`，且 handler 不得执行。不要增加不可逆工具。

## 定义与取消边界

注册时复制Properties与Required，拒绝不支持的类型以及未声明的必填字段。运行前完成注册；不支持并行Register，Handler自己的共享状态也需独立同步。TestRegistrationOwnsDefinition检查注册后修改原定义不改变契约。

Call在执行前后检查context。TestCancellationBeforeAndAfterHandler检查开始前不执行与返回后丢弃迟到结果；取消不能撤销动作。Schema只支持string/number，不是完整JSON Schema。Registry限制单次参数/结果体积，Runtime限制总调用与费用，下游读取仍要控制响应大小。

正文入口：[Tool：受控执行模型提议](../../../../book/part3-agent/building/chapter23-actions.md#topic-agent-tool)。

<a id="book-agent-tool-最佳实践"></a>

## 最佳实践

## 最佳实践

### 工具保持职责明确

先设计一个可直接测试的普通函数，再把它注册成工具。能用确定性代码完成的校验、过滤和排序，由应用完成；模型负责选择与解释，不重复承担这些可验证规则。

### 明确参数类型，不悄悄容错

JSON数字默认解码为float64，直接构造的Go参数也可能为int。实验显式支持int与有限float64；字符串数字、json.Number、数组和嵌套对象没有自动转换。增加支持时同步契约、解析与测试，不用万能转换函数吞掉问题。金额、超大整数和精确计数还需要专门定义表示方式，不能默认float64足够。

### 修复与重试分别管理

坏参数需要模型修复；网络故障可能需要应用重试；业务拒绝可能应该立即停止。这三种路径都要有上限。工具带副作用时不能默认安全重试；本课程工具只读，也要考虑调用费用、超时和重复压力。


<a id="book-agent-tool-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### Handler因类型断言panic

先直接调用执行入口复现坏参数，检查形状校验是否在Handler之前。补齐校验后，用计数Handler证明坏输入没有执行；再检查Provider是否把JSON参数转换成约定的Go类型。

### 注册成功，调用时规则却变化

检查是否保存了调用方提供的可变map或slice。注册时复制定义，运行期间停止修改；复现“注册后修改原Schema”，确认已注册工具仍按原契约校验。若需要热更新，应引入版本化清单与明确切换时刻。

### 同名工具结果交错

先核对调用ID，再看并行执行和回填顺序。名字只说明能力类型，不能定位一次调用。修复ID关联后，制造一快一慢两次调用，检查各结果仍与原参数绑定。
