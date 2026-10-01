# 用 Eino 组织一个 RAG 应用

RAG 项目的难点不在组件本身，而在谁把组件接成一条链路：本章用一个统一入口完成 query embedding → 上下文检索 → context assembly → grounded answer，并把检索顺序和输出格式稳定到可以被测试钉住。

## 由应用入口组织组件调用

**项目章的核心是统一入口，而不是再拆一组互不相连的小 demo。** `Project` 持有 embedder、model 和知识库文档，由它安排各组件的调用顺序：

```go
// 摘录自 eino-rag-project/main.go，省略 package/import 与字段类型声明。
func NewProject(embedder FakeEmbedder, model FakeAnswerModel) *Project {
	return &Project{
		embedder: embedder,
		model:    model,
		docs: []knowledgeDoc{
			{
				ID:     "basics",
				Source: "labs/part3-agent/chapter24/eino-basics",
				Text:   "Eino basics define prompt messages, context propagation, and model boundaries.",
			},
			{
				ID:     "vector",
				Source: "labs/part3-agent/chapter26/eino-vector-store",
				Text:   "Eino embedding plus a vector store retrieves the most relevant context for a query.",
			},
			{
				ID:     "project",
				Source: "labs/part3-agent/chapter26/eino-rag-project",
				Text:   "A RAG project assembles retrieved context before generating a grounded answer.",
			},
		},
	}
}
```

- 四个部件的分工：`Project` 是项目入口；`Answer` 执行 query → retrieve → answer 主链路；`ProjectAnswer` 保存问题、检索上下文和最终回答；`Render` 生成稳定输出，方便测试和阅读。
- 前两章分别介绍组件，本章要回答的是整体流程由谁执行。

## `Answer` 把 retrieve、prompt assembly、generate 串成主链路

**入口方法按固定顺序连接组件，组件各自的职责不变。** message、embedding 和 vector store 仍完成原有工作，`Answer` 只负责编排：

```go
// 摘录自 eino-rag-project/main.go。
func (p *Project) Answer(ctx context.Context, query string) (ProjectAnswer, error) {
	texts := make([]string, 0, len(p.docs))
	for _, doc := range p.docs {
		texts = append(texts, doc.Text)
	}

	vectors, err := p.embedder.EmbedStrings(ctx, texts) // 知识库向量化
	if err != nil {
		return ProjectAnswer{}, err
	}
	indexed := make([]knowledgeDoc, len(p.docs))
	for i, doc := range p.docs {
		indexed[i] = doc
		indexed[i].Vector = vectors[i]
	}

	queryVector, err := p.embedder.EmbedStrings(ctx, []string{query}) // 问题向量化
	if err != nil {
		return ProjectAnswer{}, err
	}

	contexts, err := retrieveTopContexts(queryVector[0], indexed, 2) // top-k 检索
	if err != nil {
		return ProjectAnswer{}, err
	}

	reply, err := p.model.Generate(ctx, buildAnswerMessages(query, contexts)) // 组装 prompt 并生成
	if err != nil {
		return ProjectAnswer{}, err
	}

	return ProjectAnswer{
		Question: query,
		Contexts: contexts,
		Text:     reply.Content,
	}, nil
}
```

- 本例每次 `Answer` 都重新计算知识库向量，方便在一个函数里观察完整过程，但重复请求会重复索引。真实知识库通常在入库时生成向量、查询阶段只生成问题向量；拆分时需要记录文档版本与 embedding 模型版本，避免旧索引和新查询落在不同向量空间。

## 检索顺序和渲染格式都必须稳定，测试才站得住

**结果也是接口的一部分**：top-k 顺序不稳定、输出格式经常漂移，单元测试、golden output 和文档示例会一起变脆。

```go
// 摘录自 eino-rag-project/main.go。
func retrieveTopContexts(query []float64, docs []knowledgeDoc, topK int) ([]RetrievedContext, error) {
	type scored struct {
		doc   knowledgeDoc
		score float64
	}

	scoredDocs := make([]scored, 0, len(docs))
	for _, doc := range docs {
		score, err := cosine(query, doc.Vector)
		if err != nil {
			return nil, err
		}
		if score <= 0 {
			continue // 零分和负相关不进上下文
		}
		scoredDocs = append(scoredDocs, scored{doc: doc, score: score})
	}
	if len(scoredDocs) == 0 {
		return nil, ErrNoContext
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		if scoredDocs[i].score == scoredDocs[j].score {
			return scoredDocs[i].doc.ID < scoredDocs[j].doc.ID // 同分按 ID，顺序确定
		}
		return scoredDocs[i].score > scoredDocs[j].score // 相关性降序
	})

	if len(scoredDocs) > topK {
		scoredDocs = scoredDocs[:topK] // top-k 收口
	}

	contexts := make([]RetrievedContext, 0, len(scoredDocs))
	for _, item := range scoredDocs {
		contexts = append(contexts, RetrievedContext{Source: item.doc.Source, Text: item.doc.Text})
	}
	return contexts, nil
}

func (a ProjectAnswer) Render() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Question: %s\n", a.Question)
	builder.WriteString("Retrieved context:\n")
	for i, ctx := range a.Contexts {
		fmt.Fprintf(&builder, "%d. [%s] %s\n", i+1, ctx.Source, ctx.Text)
	}
	fmt.Fprintf(&builder, "Answer: %s", a.Text)
	return builder.String()
}
```

- 知识库是有序切片，直接遍历不会无缘无故随机变序；不稳定的根源是没有按 query 相关性排序和裁剪。top-k 只限制条数，长文仍可能超出上下文预算，需要另行限制片段长度与总 token 数。
- `FakeAnswerModel` 返回固定说明文字，不能证明回答由证据推导，也不能验证幻觉率。当前测试检查检索结果、消息内容和渲染形状；接入真实模型后，还要独立检查引用是否支持答案、无命中如何回答、检索文本中的指令是否被错误执行。

在仓库根目录运行（Go 工具链与依赖版本以 `labs/go.mod` 为准，当前锁定 Eino v0.9.13）：

```bash
go -C labs run ./part3-agent/chapter28/eino-rag-project
```

```text
Question: How does rag use context?
Retrieved context:
1. [labs/part3-agent/chapter24/eino-basics] Eino basics define prompt messages, context propagation, and model boundaries.
2. [labs/part3-agent/chapter26/eino-rag-project] A RAG project assembles retrieved context before generating a grounded answer.
Answer: Use the retrieved context above as the grounding material.
```

读源码按 `NewProject` → `Answer` → `ProjectAnswer.Render` 的顺序；测试（`main_test.go`）重点看 retrieve context、stable render、golden shape 三组断言。继续扩展时，可以把知识库装载、retrieval 策略和 model adapter 从 `main.go` 中分别拆出。

## 踩坑清单

### 项目章里继续拆成互不相连的小 demo

```go
func demo1() {}
func demo2() {}
func demo3() {}
```

现象：分散的 demo 只能分别展示知识点，沿一次调用看不到完整流程。修复：用 `Project.Answer` 统一入口组织已有组件、保留各自接口以便替换——复验标准是一次请求能同时观察到检索上下文与最终回答，组件错误能返回调用方。

### retrieval 不做稳定排序和 top-k 裁剪，结果一直飘

```go
func retrieveTopContexts(query []float64, docs []knowledgeDoc) ([]RetrievedContext, error) {
	contexts := make([]RetrievedContext, 0, len(docs))
	for _, doc := range docs {
		contexts = append(contexts, RetrievedContext{
			Source: doc.Source,
			Text:   doc.Text,
		})
	}
	return contexts, nil // 反例：谁先遍历到谁就先返回，也没有 top-k 收口。
}
```

现象：按知识库切片的输入顺序返回，没有计算 query 相关性，也没有限制条数；输入顺序改变时输出就跟着变。修复：按 cosine 分数降序、同分按文档 ID、top-k 限制条数——复验时用不同相关性及同分资料检查：高相关优先、同分顺序固定、结果不超过 top-k。

### 直接打印内部结构，输出接口越来越不稳定

```go
fmt.Println(answer)
fmt.Println(contexts)
fmt.Println(reply)
```

现象：短期很快，长期是把内部结构泄露给外部读者和测试，内部类型一改输出就漂。修复：对外输出统一收进 `ProjectAnswer.Render()`，测试同时检查文本与来源——复验标准是内部结构调整不任意改变公开输出，输出变更必须同步契约与断言。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 1 让调用方选择 top-k | 通过参数或 `Project` 配置接收 top-k，传给 `retrieveTopContexts`，保留原流程并同步 `runDemo` | top-k 取 1、2、超出文档数与非法值都有断言；不越界、不改变同分排序 | `eino-rag-project/main.go`、`main_test.go` |
| 2 按证据改写回复 | `FakeAnswerModel.Generate` 从 input 读取证据构造 `schema.AssistantMessage`，必要时调整 `buildAnswerMessages` 的证据分隔，不添加外部事实 | 回复只引用 Retrieved context 中的内容且能定位来源；测试说明 fake 不能证明真实模型无幻觉 | `eino-rag-project/main.go`、`main_test.go` |

完成后运行：

```bash
go -C labs test ./part3-agent/chapter28/eino-rag-project/... -count=1
```

已有测试通过只说明已覆盖的用例符合预期，新增边界仍要补断言。更多练习（扩充知识库观察 top-2 命中与排序变化）见各 lab 的 README。

## 链接

- [Eino v0.9.13 Retriever 接口](https://pkg.go.dev/github.com/cloudwego/eino@v0.9.13/components/retriever)：对照检索查询、文档返回及组件接口，区分本例自定义检索实现与框架契约。
- 回顾 [Eino 消息与模型边界](../chapter26.md) 和 [Embedding 与向量检索](../chapter28.md)，对照框架边界和检索基础设施各自承担的职责。

[返回全书目录](../../README.md) · [上一篇：Embedding 与向量检索](../chapter28.md)
