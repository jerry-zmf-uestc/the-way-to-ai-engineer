# Agent 工程师面试手册 · 96 题

按问题复习机制、工程取舍与失败边界。课程见 [book](../book/README.md)，实现与实验见 [实验约定](../../docs/rules/topic-contract.md)。题库是主题定义真源，已创建正文才生成链接；十大章节目录预先建立，空目录仅用 .gitkeep 保留，不创建空白文章。

运行、研究、审核及草稿边界见 [内容规范](../../docs/rules/interview-content-contract.md)，整体设计见 [面试方案](../../docs/designs/interview-handbook.md)。不接入课程章号；独立面试站点通过 website.json 控制可见题目，与公众号发布状态分开。

在仓库根目录运行 `node scripts/interview.mjs next-work` 选择下一题，`node scripts/interview.mjs scaffold AE-ID` 创建正文，`node scripts/interview.mjs render` 更新导航。题库与状态直接保存在本目录的 `questions.json`、`state.json`；状态用 `status`，关联缺口用 `coverage` 查看；`make validate-interview` 检查公共结构，`node scripts/interview.mjs delivery-check AE-ID` 单独检查本地交付证据。

<!-- generated: questions.json; edit catalogue then render -->

## Reasoning & Planning（7 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-001 | AE-001 | [ReAct 和 Plan-and-Execute 有什么区别，业务里怎么选？](reasoning-planning/ae-001-react-vs-plan-execute.md) | basic |
| P-002 | AE-081 | Agent 为什么需要 Planning？如何设计一个可靠的 Planner？（待编写） | advanced |
| P-003 | AE-100 | Chain-of-Thought、ReAct、Planning 分别解决什么问题？（待编写） | basic |
| P-004 | AE-101 | 复杂任务为什么不能只靠 Prompt？（待编写） | basic |
| P-008 | AE-031 | Reflection 有什么价值？（待编写） | advanced |
| P-009 | AE-051 | 如何避免 Agent 推理循环和死循环？（待编写） | intermediate |
| P-010 | AE-011 | Workflow 和 Autonomous Agent 应该如何选择？（待编写） | basic |

## Tool Calling（10 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-011 | AE-002 | Function Calling 和模型直接输出 JSON 有什么区别？（待编写） | basic |
| P-012 | AE-012 | 一个好的 Tool Schema 应该怎么设计？（待编写） | intermediate |
| P-013 | AE-022 | 多个相似 Tool 时模型如何正确选择？（待编写） | intermediate |
| P-014 | AE-102 | Tool Description 为什么如此重要？（待编写） | intermediate |
| P-015 | AE-042 | Tool Calling 失败后如何 Retry？（待编写） | intermediate |
| P-016 | AE-040 | Timeout、Retry、Fallback 如何设计？（待编写） | intermediate |
| P-017 | AE-103 | 如何避免 Agent 无限调用 Tool？（待编写） | intermediate |
| P-018 | AE-075 | 高风险 Tool 如何进行人工确认？（待编写） | intermediate |
| P-019 | AE-062 | Tool 权限模型应该如何设计？（待编写） | intermediate |
| P-020 | AE-052 | Tool Result 应该如何反馈给模型？（待编写） | intermediate |

## Context & Memory（10 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-021 | AE-013 | Context 和 Memory 有什么区别？（待编写） | basic |
| P-022 | AE-003 | 为什么不能把所有历史对话都放进 Context？（待编写） | intermediate |
| P-023 | AE-104 | Agent State 应该存哪些内容？（待编写） | intermediate |
| P-024 | AE-023 | Short-term Memory 和 Long-term Memory 有什么区别？（待编写） | basic |
| P-025 | AE-105 | Conversation Memory 应该如何实现？（待编写） | intermediate |
| P-026 | AE-106 | Summary Memory 有哪些优缺点？（待编写） | intermediate |
| P-027 | AE-107 | Semantic Memory 和 Episodic Memory 有什么区别？（待编写） | basic |
| P-028 | AE-053 | Agent 如何实现用户长期记忆？（待编写） | advanced |
| P-029 | AE-033 | Context Compression 应该怎么做？（待编写） | intermediate |
| P-030 | AE-108 | Context Engineering 和 Prompt Engineering 有什么区别？（待编写） | basic |

## RAG（10 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-031 | AE-109 | RAG 解决的核心问题是什么？（待编写） | basic |
| P-032 | AE-110 | RAG 的完整 Pipeline 是什么？（待编写） | intermediate |
| P-033 | AE-014 | Chunk Size 应该如何选择？（待编写） | intermediate |
| P-034 | AE-111 | Embedding 模型应该如何选择？（待编写） | intermediate |
| P-035 | AE-034 | Vector Search 和 Keyword Search 如何组合？（待编写） | intermediate |
| P-036 | AE-044 | Rerank 为什么重要？（待编写） | intermediate |
| P-037 | AE-054 | Query Rewrite 有什么价值？（待编写） | intermediate |
| P-038 | AE-004 | Agentic RAG 和传统 RAG 有什么区别？（待编写） | basic |
| P-039 | AE-084 | Graph RAG 适合什么场景？（待编写） | advanced |
| P-040 | AE-058 | RAG 系统如何进行效果评估？（待编写） | intermediate |

## Eval & Observability（8 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-041 | AE-112 | Agent 为什么比普通应用更需要 Eval？（待编写） | intermediate |
| P-042 | AE-008 | Agent Eval 应该评估哪些指标？（待编写） | basic |
| P-043 | AE-038 | LLM-as-a-Judge 是否可靠？（待编写） | advanced |
| P-044 | AE-028 | Offline Eval 和 Online Eval 有什么区别？（待编写） | basic |
| P-045 | AE-018 | Agent Trace 应该记录哪些信息？（待编写） | intermediate |
| P-046 | AE-068 | 如何 Debug 一个表现不稳定的 Agent？（待编写） | intermediate |
| P-047 | AE-048 | 如何评估 Tool Calling 的正确率？（待编写） | intermediate |
| P-048 | AE-078 | 如何建立 Agent Observability 系统？（待编写） | advanced |

## Agent Runtime / Harness（12 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-049 | AE-005 | 什么是 Agent Runtime？（待编写） | basic |
| P-050 | AE-113 | Agent Runtime 和 Agent Framework 有什么区别？（待编写） | basic |
| P-051 | AE-015 | 一个 Agent Loop 是如何运行的？（待编写） | intermediate |
| P-052 | AE-025 | Agent Runtime 应该管理哪些状态？（待编写） | intermediate |
| P-053 | AE-114 | Session 应该如何设计？（待编写） | intermediate |
| P-054 | AE-072 | Agent Runtime 如何管理 Tool？（待编写） | intermediate |
| P-055 | AE-055 | Agent 如何处理中断和恢复？（待编写） | intermediate |
| P-056 | AE-045 | Agent Runtime 如何实现 Checkpoint？（待编写） | advanced |
| P-057 | AE-115 | Agent Harness 是什么？（待编写） | basic |
| P-058 | AE-116 | Runtime、Harness、Model 三者是什么关系？（待编写） | basic |
| P-059 | AE-065 | Coding Agent 为什么需要 Sandbox？（待编写） | advanced |
| P-060 | AE-095 | 如何设计一个最小可用 Agent Runtime？（待编写） | advanced |

## Multi-Agent（9 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-061 | AE-009 | 单 Agent 和 Multi-Agent 应该怎么选？（待编写） | basic |
| P-062 | AE-098 | Agent 数量是不是越多越好？（待编写） | advanced |
| P-063 | AE-039 | Multi-Agent 有哪些典型架构？（待编写） | basic |
| P-064 | AE-019 | Supervisor 模式是如何工作的？（待编写） | basic |
| P-065 | AE-117 | Handoff 和 Tool Call 有什么区别？（待编写） | basic |
| P-066 | AE-118 | 多 Agent 之间如何通信？（待编写） | intermediate |
| P-067 | AE-029 | 多 Agent 如何共享 Context 和 State？（待编写） | intermediate |
| P-068 | AE-049 | 多 Agent 如何避免重复工作？（待编写） | advanced |
| P-069 | AE-119 | 什么情况下 Multi-Agent 反而会降低系统效果？（待编写） | advanced |

## MCP / External Tools（8 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-070 | AE-006 | MCP 到底解决了什么问题？（待编写） | basic |
| P-071 | AE-120 | MCP 和 Function Calling 有什么区别？（待编写） | basic |
| P-072 | AE-026 | MCP Server 和 MCP Client 分别是什么？（待编写） | basic |
| P-073 | AE-016 | MCP Tools、Resources、Prompts 有什么区别？（待编写） | basic |
| P-074 | AE-036 | MCP 的 Transport 层是如何工作的？（待编写） | intermediate |
| P-075 | AE-056 | MCP Server 应该如何进行权限控制？（待编写） | advanced |
| P-076 | AE-121 | MCP 和传统 REST API 有什么区别？（待编写） | basic |
| P-077 | AE-096 | MCP、A2A、Tool Calling 三者是什么关系？（待编写） | advanced |

## Production Engineering（12 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-078 | AE-010 | Demo Agent 和 Production Agent 最大区别是什么？（待编写） | basic |
| P-079 | AE-035 | Agent 如何设计 Timeout？（待编写） | intermediate |
| P-080 | AE-122 | Agent 如何设计 Retry？（待编写） | intermediate |
| P-081 | AE-123 | Agent 如何实现 Circuit Breaker？（待编写） | advanced |
| P-082 | AE-094 | Agent 如何实现 Graceful Degradation？（待编写） | intermediate |
| P-083 | AE-060 | Agent 如何进行权限控制？（待编写） | intermediate |
| P-084 | AE-089 | Agent 如何进行成本控制？（待编写） | intermediate |
| P-085 | AE-030 | Agent 如何控制 Token 使用？（待编写） | intermediate |
| P-086 | AE-090 | Agent 如何处理高并发？（待编写） | advanced |
| P-087 | AE-050 | Agent 如何设计异步任务？（待编写） | advanced |
| P-088 | AE-092 | Agent 如何保证幂等性？（待编写） | advanced |
| P-089 | AE-080 | Agent 如何进行安全审计？（待编写） | advanced |

## Agent System Design（10 题）

| 题序 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-090 | AE-007 | 如何设计一个 Deep Research Agent？（待编写） | system-design |
| P-091 | AE-124 | 如何设计一个 Coding Agent？（待编写） | system-design |
| P-092 | AE-037 | 如何设计一个 Browser Agent？（待编写） | system-design |
| P-093 | AE-017 | 如何设计一个企业知识库 Agent？（待编写） | system-design |
| P-094 | AE-027 | 如何设计一个 Data Analyst Agent？（待编写） | system-design |
| P-095 | AE-077 | 如何设计一个 Agent Memory System？（待编写） | system-design |
| P-096 | AE-067 | 如何设计一个 Multi-Agent Research System？（待编写） | system-design |
| P-097 | AE-087 | 如何设计一个 Agent Eval Platform？（待编写） | system-design |
| P-098 | AE-057 | 如何设计一个支持 MCP 的 Agent Runtime？（待编写） | system-design |
| P-099 | AE-047 | 如何设计一个类似 Claude Code / Codex 的 Coding Agent？（待编写） | system-design |
