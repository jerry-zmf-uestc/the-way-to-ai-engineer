# 第 26 章：Eino 消息与模型边界

学 embedding、向量检索和完整 RAG 之前，先固定 Eino 的三条边界：message 管输入语义、model 是可替换接口、应用入口管校验与编排。本章用一个离线 demo（`labs/part3-agent/chapter26/eino-basics`）把它们跑通，无需模型密钥。

## 1. 用 message 组织输入

**system message 承担稳定规则，user message 承担本次请求参数。**

```go
// 摘录自 eino-basics/main.go。
func buildPromptMessages(topic, audience string) []*schema.Message {
	return []*schema.Message{
		schema.SystemMessage("You explain Eino component boundaries with concise Go examples."),
		schema.UserMessage(fmt.Sprintf("topic=%s audience=%s", topic, audience)),
	}
}
```

- 这段代码的价值不在拼字符串，而在先把输入语义分层：以后无论接模板、接 RAG、接真实模型，输入边界都应该先落到消息列表上。

## 2. fake 对齐真实模型的接口形状

**fake 必须实现 `model.BaseChatModel` 的真实签名**，否则接真实模型时要连调用协议一起改：

```go
// 摘录自 eino-basics/main.go。
type FakeChatModel struct{}

var _ model.BaseChatModel = FakeChatModel{} // 接口断言：编译器检查方法集

func (FakeChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if len(input) == 0 {
		return nil, errors.New("eino basics: empty prompt")
	}

	return schema.AssistantMessage("Eino basics demo: "+input[len(input)-1].Content, nil), nil
}

func (m FakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reply, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}

	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Send(reply, nil)
	writer.Close()
	return reader, nil
}
```

- Stream 与 Generate 共享消息协议，只是消费方式不同：StreamReader 要读到结束并关闭，生产端也要结束写入，否则对端一直等待。
- 测试通过只证明消息形状和调用关系符合本例约定，不证明真实模型会遵从 system 指令。接口以 `labs/go.mod` 锁定的版本为准，升级前先编译接口断言和相关测试。

## 3. 应用入口收拢校验与调用

**输入校验发生在模型调用之前**，`runDemo` 是最小应用边界：校验、prompt 组装、模型调用都从这里发出：

```go
// 摘录自 eino-basics/main.go。
func runDemo(ctx context.Context, input DemoInput) (string, error) {
	if strings.TrimSpace(input.Topic) == "" || strings.TrimSpace(input.Audience) == "" {
		return "", ErrInvalidInput
	}

	reply, err := (FakeChatModel{}).Generate(ctx, buildPromptMessages(input.Topic, input.Audience))
	if err != nil {
		return "", err
	}
	return reply.Content, nil
}
```

上游业务不用知道 message 是怎么拼的，下游模型也不用知道参数来自 HTTP、CLI 还是测试。在仓库根目录运行：

```bash
go -C labs run ./part3-agent/chapter26/eino-basics
```

```text
Eino basics demo: topic=retrieval pipeline audience=Go reader
```

对照源码和测试（`main.go`、`main_test.go`）时，重点看 `buildPromptMessages` 的消息分层、`FakeChatModel` 的 Generate 行为和 `runDemo` 的输入校验。

## 踩坑清单

### transport、prompt、model 焊死在一起

```go
func handler(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	prompt := "topic=" + topic
	reply := callModel(prompt) // 反例：HTTP 层直接拼 prompt、直接调模型
	_, _ = w.Write([]byte(reply))
}
```

现象：短期跑得快，等到补测试、换模型、加上下文时，没有一个稳定位置可以挂载这些变化。修复：保留 buildPromptMessages、model、runDemo 三层边界，入口只传已校验数据，空输入在模型调用前被拒绝。

### fake 只写随意方法，不对齐真实接口

```go
type BadFakeModel struct{}

func (BadFakeModel) Reply(prompt string) string { // 反例：不实现 model.BaseChatModel
	return "ok"
}
```

现象：能返回文字，但接入 `Generate`、`Stream`、message list 和 option 参数时要修改调用接口，无法只替换实现。修复：按真实签名实现 Generate/Stream，并加 `var _ model.BaseChatModel = ...` 断言让编译器检查方法集。

### 一开始就依赖真实远程模型和凭证

```go
func main() {
	client := mustNewRemoteModel(os.Getenv("OPENAI_API_KEY")) // 反例：最小 demo 依赖网络与密钥
	_, _ = client.Generate(context.Background(), nil)
}
```

现象：注意力被"怎么跑通"带走，离线环境连测试都起不来。修复：默认注入本地 FakeChatModel，真实 Provider 留给显式集成验证；离线测试成功只证明本地协议，不证明模型效果。

### StreamReader 没读到结束

现象：消费方中途放弃读取，或生产端没有 `Close`，对端一直等待。修复：消费方读到结束并关闭，生产端写完必须 Close——Generate 和 Stream 共享协议，但消费方式的责任各负一半。

### 以为传入 ctx 就响应了取消

现象：本例 fake 的 Generate 不检查取消信号，"传入了 ctx"与"组件检查了取消"是两项不同能力。修复：具体实现要主动响应 `ctx.Done()`；fake 未模拟的网络抖动、部分输出后失败、复杂取消，接真实适配器时单独验证。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 26.1 增加 tone 参数 | 给 `buildPromptMessages` 增加 tone 参数，决定它进 system（长期规则）还是 user（本次请求）消息；同步 `DemoInput`、`runDemo` 调用与 `TestBuildPromptMessages` | 新参数进入选定消息，角色与原字段保留；改签名后调用方和测试同步编译通过 | `eino-basics/main.go`、`main_test.go` |
| 26.2 检查应用入口校验 | 给 `TestRunDemoRejectsMissingFields` 补 `"   "` 和 `"\t"` 用例，断言 `errors.Is(err, ErrInvalidInput)`；沿 `runDemo` 确认校验发生在 `Generate` 之前 | 空 topic、空 audience 和纯空白都在模型调用前返回 `ErrInvalidInput`；校验不移进 fake 内部 | `eino-basics/main_test.go` |

完成后运行：

```bash
go -C labs test ./part3-agent/chapter26/eino-basics/... -count=1
```

更多练习（修改回复格式并同步输出测试、检查 Stream 与 Generate 语义一致）见各 lab 的 README。

## 链接

- [Eino v0.9.13 消息类型](https://pkg.go.dev/github.com/cloudwego/eino@v0.9.13/schema)：Message、角色和流式数据结构的定义。
- [Eino v0.9.13 模型接口](https://pkg.go.dev/github.com/cloudwego/eino@v0.9.13/components/model)：核对 BaseChatModel 的 Generate、Stream 及调用参数。

接口行为以 `labs/go.mod` 锁定的 Eino v0.9.13 为准；在线文档可能对应更新版本，遇到差异先核对锁定版本。

[返回全书目录](../README.md) · [上一篇：Kitex 与 RPC 契约](../part2-backend/chapter17.md) · [下一篇：Embedding 与向量检索](chapter28.md)
