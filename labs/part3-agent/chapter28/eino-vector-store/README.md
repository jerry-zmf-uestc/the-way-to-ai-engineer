# 示例： eino vector store

## Goal

Understand how an Eino embedder and a vector store cooperate to build the
retrieval half of a RAG system.

## Concepts

- `HashEmbedder` demonstrates the Eino embedding interface without remote APIs.
- `MemoryStore` owns dimension checks and result ordering.
- `VectorProject` is the application boundary that coordinates indexing and
  search.

## Run

```bash
gofmt -l labs/part3-agent/chapter28/eino-vector-store
go -C labs vet ./part3-agent/chapter28/eino-vector-store
go -C labs test ./part3-agent/chapter28/eino-vector-store -race -count=1
go -C labs test ./part3-agent/chapter28/eino-vector-store -cover
go -C labs build -o /tmp/eino-vector-store ./part3-agent/chapter28/eino-vector-store
go -C labs run ./part3-agent/chapter28/eino-vector-store
```

Expected output:

```text
indexed=3 query="relevant rag context" top_hit=rag score=0.82
```

## Code Map

- [main.go](main.go): defines a deterministic Eino-shaped embedder, the in-memory
  vector store, and the project boundary that coordinates indexing and search.
- [main_test.go](main_test.go): verifies embedding shape, ranking behavior,
  empty-result versus invalid-request semantics, partial state after a query
  embedding failure, and the bounded demo summary.


## Continue

This chapter stops at retrieval infrastructure. The next chapter turns these
boundaries into a project-shaped RAG demo:
[eino-rag-project](../eino-rag-project).

## 原章节练习

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 28.1 metadata过滤 | 给MemoryStore.Search增加过滤参数，在算分之前筛选；同步IndexAndSearch及调用点，空过滤保留原行为 | 不同过滤条件返回不同ID集合，被排除资料不得出现 | main.go、main_test.go |
| 28.2 top-k边界 | 分别传1、2、大于文档数和非正值 | 数量及错误符合契约，同分ID排序稳定 | main_test.go |

更多练习：替换语料与查询，使用可解释词重合数据；HashEmbedder不是训练得到的语义Embedding。

本实验代码原创，MIT；框架接口使用锁定的Eino依赖。
