# Golang Agent

先用本地 fake 跑通消息和检索，再把模型调用放入清楚的应用接口。

1. [LLM API 与结构化输出](chapter22.md)：消息模型、Provider 适配与 JSON 契约。
2. [Tool Calling 与最小 Agent Loop](chapter23.md)：工具注册表、参数校验与循环边界。
3. [Embedding 与向量检索](chapter28.md)：批量向量化、维度、复制和排序。
4. [RAG 应用](chapter28/rag.md)：问题、检索上下文、消息组装与回答输出。

本地确定性测试检查组件行为，不评判真实模型回答质量。阅读时分别记录“程序是否按约定执行”和“答案是否有证据支持”。

[返回全书目录](../README.md)
