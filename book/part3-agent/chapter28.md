# 第 28 章：Embedding 与向量检索

把文本转成向量、放进可检索的存储，关键不在单个组件，而在边界划分：维度约束收在 store，索引与查询的调用次序收在项目入口。

[eino-vector-store](../../labs/part3-agent/chapter28/eino-vector-store) 把这条链路拆成三个边界：

| 部件 | 作用 |
| --- | --- |
| `HashEmbedder` | 用 Eino embedding 接口形状演示批量向量化 |
| `MemoryStore` | 负责维度校验、写入和 top-k 检索 |
| `VectorProject` | 协调文档 indexing 与 query search 的应用边界 |

## 写法与行为

### 1. embedding 接收批量输入

**embedding 接口一次接收整批文本**，归一化、稳定 bucket 映射、单位化都在实现内部完成，调用方不需要分别重复这些步骤。

```go
// 摘录自 eino-vector-store/main.go。
func (h HashEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	dims := h.Dimensions
	if dims <= 0 {
		dims = 8
	}

	out := make([][]float64, 0, len(texts))
	for _, text := range texts {
		normalized := normalizeText(text)
		if normalized == "" {
			return nil, ErrEmptyText
		}

		vector := make([]float64, dims)
		for _, token := range strings.Fields(normalized) {
			vector[stableBucket(token, dims)]++
		}
		out = append(out, unit(vector))
	}
	return out, nil
}
```

- HashEmbedder 按词的哈希桶计数生成向量再单位化，没有学习语言语义：维度较少时不同词可能落在同一个桶，按空白分词对中文连续文本的局限尤其明显。演示中的相似度只用于理解向量计算与排序，"文字听起来相关"不保证高分。

### 2. store 检查维度并保存独立数据

**维度校验只在 store 一处做**；写入时复制向量和 metadata，检索按"分数降序、同分按 ID"的固定规则排序。

```go
// 摘录自 eino-vector-store/main.go。
func (s *MemoryStore) Upsert(doc storedDoc) error {
	if strings.TrimSpace(doc.ID) == "" {
		return ErrEmptyID
	}
	if err := s.checkVector(doc.Vector); err != nil {
		return fmt.Errorf("upsert %q: %w", doc.ID, err)
	}

	copied := doc
	copied.Vector = append([]float64(nil), doc.Vector...)
	copied.Metadata = copyMetadata(doc.Metadata)
	s.docs[doc.ID] = copied
	return nil
}

func (s *MemoryStore) Search(query []float64, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		return nil, ErrLimitInvalid
	}
	if err := s.checkVector(query); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	hits := make([]SearchHit, 0, len(s.docs))
	for _, doc := range s.docs {
		score, err := cosine(query, doc.Vector)
		if err != nil {
			return nil, err
		}
		hits = append(hits, SearchHit{ID: doc.ID, Text: doc.Text, Score: score})
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})

	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
```

- 复制入库数据不等于支持并发读写，也不提供持久化。查询维度、零向量、`limit<=0`、重复 ID 的覆盖语义和同分排序都属于检索契约，替换存储前应逐项比较。

### 3. 项目入口串起 indexing 和 retrieval

**索引与查询的调用次序由项目入口安排**：先批量 embed 文档并写入 store，再 embed query 执行 search。

```go
// 摘录自 eino-vector-store/main.go。
func (p *VectorProject) IndexAndSearch(ctx context.Context, docs []DocInput, query string, topK int) ([]SearchHit, error) {
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Text)
	}

	vectors, err := p.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, err
	}

	for i, doc := range docs {
		if err := p.store.Upsert(storedDoc{
			ID:       doc.ID,
			Text:     doc.Text,
			Metadata: doc.Metadata,
			Vector:   vectors[i],
		}); err != nil {
			return nil, err
		}
	}

	queryVector, err := p.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}

	return p.store.Search(queryVector[0], topK)
}
```

- IndexAndSearch 顺序写入文档，中间失败可能留下已写入的部分文档，不是批量原子事务。换用远端 embedder 后，先验证返回向量数量与输入数一致，再按索引访问。top-k 控制结果条数，不控制总 token 数或单篇长度。

## 检索成功还不能证明答案可靠

`Search` 返回 `nil` error，只能说明输入满足本地契约、相似度计算完成。它没有证明命中的文本与问题真正相关，也没有证明结果足够完整，更没有证明后续回答忠于这些文本。

| 情况 | 返回行为 | 已经证明 | 仍未证明 |
| --- | --- | --- | --- |
| 空 store + 合法 query | 空切片、`nil` error | 检索过程可以执行 | 知识库有内容 |
| `topK` 大于文档数 | 返回全部文档、`nil` error | 没有超过现有结果 | 已达到调用方想要的数量 |
| 哈希桶碰撞 | 可能得到较高分 | 两个计数向量方向接近 | 文本语义相关 |
| `topK <= 0` | `ErrLimitInvalid` | 请求违反接口契约 | 存储或 embedding 故障 |
| query 维度不同 | 包装后的 `ErrDimensionMismatch` | 查询和索引不兼容 | 哪一侧使用了错误模型版本 |
| query embedding 失败 | 直接返回 embedding error | 查询没有执行 | 前面的文档写入已回滚 |

最后一行很容易误判。`IndexAndSearch` 先写文档，再生成 query 向量；因此空 query 会在文档已经写入后返回 `ErrEmptyText`。调用方如果看到 error 就整批重试，当前 store 会按相同 ID 覆盖；换成追加写入或会计费的远端存储，重试可能产生重复数据或额外成本。

下面的测试不是为了追求更多覆盖率，而是把这个失败传播路径固定下来：

```go
// 摘录自 eino-vector-store/main_test.go。
func TestQueryEmbeddingFailureDoesNotMeanIndexRollback(t *testing.T) {
	store := NewMemoryStore()
	project := NewVectorProject(HashEmbedder{Dimensions: 8}, store)

	_, err := project.IndexAndSearch(context.Background(), []DocInput{
		{ID: "rag", Text: "rag retrieves context"},
	}, "", 1)
	if !errors.Is(err, ErrEmptyText) {
		t.Fatalf("IndexAndSearch() error = %v, want ErrEmptyText", err)
	}
	if len(store.docs) != 1 {
		t.Fatalf("stored documents = %d, want 1", len(store.docs))
	}
}
```

生产实现通常会把 indexing 和 retrieval 拆成两个入口：索引任务负责幂等写入、批次状态和失败恢复；在线查询只读取已经发布的索引。这个拆分不是为了目录更漂亮，而是避免一次请求同时承担两套生命周期。

## 从底层到回答逐层排错

RAG 返回了一个看似合理但依据错误的答案时，直接调整 prompt 通常太早。按下面的顺序运行，能逐层排除问题：

1. **Embedder**：对固定文本调用 `EmbedStrings`，检查向量数量、维度、空输入和单位化行为。
2. **Store**：绕开文本和模型，直接写入手工向量；验证维度拒绝、top-k、同分排序和空库行为。
3. **Project**：运行 `IndexAndSearch`，确认 indexing 与 query embedding 的调用次序，以及中途失败留下的状态。
4. **RAG 组装**：检查命中文本、分数、来源和最终送给模型的上下文，不能只看最终回答。
5. **真实模型**：最后才接远端 embedding 和生成模型，同时记录模型版本、请求 ID、延迟与费用。

本章 Lab 覆盖前三层。它故意不连接真实模型，因此可以稳定判断错误属于向量链路还是模型行为；真实语义召回率需要另建带标注查询和相关文档的评测集，不能从这三个示例文档外推。

```bash
go -C labs run ./part3-agent/chapter28/eino-vector-store
```

```text
indexed=3 query="relevant rag context" top_hit=rag score=0.82
```

阅读顺序：先看 main.go 的 `HashEmbedder` → `MemoryStore` → `VectorProject.IndexAndSearch`，再看 main_test.go 里 batch embedding、top result 和 demo 输出三类断言。

## 踩坑清单

### 维度约束分散在多个地方

```go
func handler() {
	if len(vector) != 8 {
		panic("bad vector")
	}
	store.Upsert(storedDoc{ID: "rag", Vector: vector}) // 反例：调用方自己记维度，store 也没有统一锁住约束。
}
```

现象：维度检查散在多处，从 8 维改成 16 维要逐一查找所有判断，容易遗漏。修复：维度检查收进 store，调用方不再各自维护规则；错误维度被拒绝且不写入，合法维度的索引与查询通过。

### store 直接持有外部切片引用

```go
func (s *MemoryStore) Upsert(doc storedDoc) error {
	s.docs[doc.ID] = doc // 反例：Vector 和 Metadata 都还是外部对象的引用。
	return nil
}
```

现象：入库时一切正常，但调用方后续复用 `doc.Vector` 或 `doc.Metadata`，store 里的数据被"远程修改"。修复：写入时分别复制切片和 map；在测试中入库后修改原切片和 map，断言检索结果不随外部修改变化。

### 只返回命中 ID，检索失去可解释性

```go
type Hit struct {
	ID string
}
```

现象：结果只有 ID，无法回答"为什么命中的是它""它和 query 有多接近"，也没法决定下一层 answer generation 该拿什么上下文。修复：结果保留 ID、Text、Score 和来源信息；用固定输入检查 top-k，断言返回条数上限与同分顺序稳定。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 28.1 加入 metadata 过滤 | 给 `MemoryStore.Search` 增加 metadata 过滤参数，先筛 metadata 再算分数；同步 IndexAndSearch 和所有调用点，空过滤器保留原行为 | 同一 query 加不同过滤条件，返回文档集合按条件变化；测试断言命中和被排除的 ID | `eino-vector-store/main.go`、`main_test.go` |
| 28.2 测试不同 top-k | 用现有 `IndexAndSearch` 分别传 topK=1 和 2，再覆盖请求数大于文档数和非正值 | top-1 与 top-2 数量不同，同分仍按 ID 排序 | `eino-vector-store/main_test.go` |

完成后运行：

```bash
go -C labs test ./part3-agent/chapter28/eino-vector-store/... -count=1
```

更多练习（替换语料和查询、固定可解释的词重合样例）见各 lab 的 README。

## 链接

- [Eino v0.9.13 Embedding 接口](https://pkg.go.dev/github.com/cloudwego/eino@v0.9.13/components/embedding)：核对批量文本输入、向量输出和接口选项。
- [Eino v0.9.13 检索接口](https://pkg.go.dev/github.com/cloudwego/eino@v0.9.13/components/retriever)：区分检索接口与具体存储实现的职责。
- [用 Eino 组织一个 RAG 应用](chapter28/rag.md)：把本章的检索基础设施收进一个完整的项目入口。

[返回全书目录](../README.md) · [上一篇：Eino 消息与模型边界](chapter26.md) · [下一篇：用 Eino 组织一个 RAG 应用](chapter28/rag.md)
