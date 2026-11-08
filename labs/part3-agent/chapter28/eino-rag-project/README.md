# 示例： eino rag project

## Goal

Use a single runnable project to connect query embedding, retrieval, context
assembly, and grounded answer generation.

## Concepts

- `Project` is the application boundary.
- `Answer` coordinates retrieval and answer generation.
- `ProjectAnswer.Render` gives the stable golden output shape the tests rely on.

## Run

```bash
gofmt -l labs/part3-agent/chapter28/eino-rag-project
go -C labs vet ./part3-agent/chapter28/eino-rag-project
go -C labs test ./part3-agent/chapter28/eino-rag-project -race -count=1
go -C labs test ./part3-agent/chapter28/eino-rag-project -cover
go -C labs build -o /tmp/eino-rag-project ./part3-agent/chapter28/eino-rag-project
go -C labs run ./part3-agent/chapter28/eino-rag-project
```

Expected output:

```text
Question: How does rag use context?
Retrieved context:
1. [labs/part3-agent/chapter28/eino-vector-store] Eino embedding adapters define batch input, context propagation, and component boundaries.
2. [labs/part3-agent/chapter28/eino-rag-project] A RAG project assembles retrieved context before generating a grounded answer.
Answer: Use the retrieved context above as the grounding material.
```

## Code Map

- [main.go](main.go): defines the project boundary, local knowledge base,
  retrieval flow, and answer rendering.
- [main_test.go](main_test.go): verifies retrieval presence, fixed fake output,
  and stable project-shaped demo rendering.


## Continue

This is an independent component-composition experiment. After reading this topic, go
back to [Retrieval](../../../../content/book/part3-agent/building/chapter27-rag.md#topic-ch28-vector-retrieval) and
[RAG](../../../../content/book/part3-agent/building/chapter27-rag.md#topic-ch28-rag-app) if you want to inspect the
retrieval and grounded-answer layers separately.

## 原组装练习

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 让调用方选择top-k | 配置或参数接收top-k，传给retrieveTopContexts，同步demo | 1、2、超出文档数与非法值有断言，不越界且同分排序稳定 | main.go、main_test.go |
| 按证据改写回复 | FakeAnswerModel从input读取证据构造回复，必要时调整buildAnswerMessages分隔 | 仅引用提供的内容，可定位来源；fake不证明真实模型无幻觉 | main.go、main_test.go |

扩充知识库并观察top-2命中与排序；先核对检索、消息、输出各层，再接入真实Provider。当前固定回复只证明组装形状，不证明断言支持。原代码原创，MIT。
