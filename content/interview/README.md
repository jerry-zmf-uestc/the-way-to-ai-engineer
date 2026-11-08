# Agent 工程师面试手册 · 50 题

题库title是本期主题，按order组织公众号科普文章，优先保存草稿并回读，最终正文归档后供网站同稿预览。课程见 [book](../book/README.md)，实现与实验见 [实验约定](../../docs/rules/topic-contract.md)。题库是主题定义真源，已创建正文才生成链接；十大章节目录预先建立，空目录仅用 .gitkeep 保留，不创建空白文章。

运行、研究、审核及草稿边界见 [内容规范](../../docs/rules/interview-content-contract.md)，整体设计见 [面试方案](../../docs/designs/interview-handbook.md)，豆包工作创建和更新公众号稿件见 [公众号规则](../../docs/rules/interview-wechat.md)。不接入课程章号；独立面试站点通过 website.json 控制可见题目，与公众号发布状态分开。

在仓库根目录运行 `node scripts/interview/issue.mjs next` 按列表选择下一篇新稿，`node scripts/interview/issue.mjs start AE-ID` 在私有区准备本期；保存核验后归档。旧next-work/scaffold保留历史编辑兼容，不作为公众号新稿入口。`node scripts/interview.mjs render` 更新导航。题库与状态直接保存在本目录的 `questions.json`、`state.json`；状态用 `status`，关联缺口用 `coverage` 查看；`make validate-interview` 检查公共结构，`node scripts/interview.mjs delivery-check AE-ID` 单独检查本地交付证据。

<!-- generated: questions.json; edit catalogue then render -->

## Reasoning & Planning（5 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-001 | AE-001 | [ReAct 和 Plan-and-Execute 有什么区别，业务里怎么选？](reasoning-planning/ae-001-react-vs-plan-execute.md) | basic |
| P-002 | AE-081 | [Agent 为什么需要 Planning？如何设计一个可靠的 Planner？](reasoning-planning/ae-081-planner-design.md) | advanced |
| P-003 | AE-100 | [复杂任务的 Agent 设计：提示工程、工作流编排与自主智能体](reasoning-planning/ae-100-reasoning-strategies.md) | basic |
| P-008 | AE-031 | Reflection 有什么价值？（待编写） | advanced |
| P-009 | AE-051 | Agent 为什么会反复推理或调用工具，怎样停止？（待编写） | intermediate |

## Tool Calling（4 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-011 | AE-002 | Function Calling 和模型直接输出 JSON 有什么区别？（待编写） | basic |
| P-012 | AE-012 | 相似工具太多，怎样让 Agent 选对、填对？（待编写） | intermediate |
| P-019 | AE-062 | 高风险工具怎样做注册、授权和人工确认？（待编写） | intermediate |
| P-020 | AE-052 | 工具结果怎样回传，失败后何时重试或回退？（待编写） | intermediate |

## Context & Memory（4 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-021 | AE-013 | Context、Memory 与 Prompt 在一次请求中怎样配合？（待编写） | basic |
| P-022 | AE-003 | 长对话怎样保留近期消息、摘要和必要历史？（待编写） | intermediate |
| P-023 | AE-104 | [Agent State 应该存哪些内容？](context-memory/ae-104-agent-state.md) | intermediate |
| P-028 | AE-053 | 长期记忆怎样写入、检索、更新和删除？（待编写） | advanced |

## RAG（6 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-031 | AE-109 | 一个 RAG 请求怎样从文档走到有引用的答案？（待编写） | basic |
| P-033 | AE-014 | Chunk 和 Embedding 怎样共同影响召回？（待编写） | intermediate |
| P-035 | AE-034 | 怎样用 Query Rewrite、混合检索和 Rerank 改善难查询？（待编写） | intermediate |
| P-038 | AE-004 | Agentic RAG 和传统 RAG 有什么区别？（待编写） | basic |
| P-039 | AE-084 | Graph RAG 适合什么场景？（待编写） | advanced |
| P-040 | AE-058 | RAG 系统如何进行效果评估？（待编写） | intermediate |

## Eval & Observability（4 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-041 | AE-112 | Agent 上线前后怎样评估？（待编写） | intermediate |
| P-043 | AE-038 | LLM-as-a-Judge 是否可靠？（待编写） | advanced |
| P-045 | AE-018 | 怎样用 Trace 和可观测性排查不稳定的 Agent？（待编写） | intermediate |
| P-047 | AE-048 | 如何评估 Tool Calling 的正确率？（待编写） | intermediate |

## Agent Runtime / Harness（5 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-049 | AE-005 | Agent Runtime、Harness 和 Framework 各负责什么？（待编写） | basic |
| P-051 | AE-015 | 怎样实现一个最小 Agent Loop？（待编写） | intermediate |
| P-052 | AE-025 | Runtime State 和 Session 各存什么？（待编写） | intermediate |
| P-055 | AE-055 | Agent 中断后怎样用 Checkpoint 安全恢复？（待编写） | intermediate |
| P-059 | AE-065 | Coding Agent 为什么需要 Sandbox？（待编写） | advanced |

## Multi-Agent（3 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-061 | AE-009 | 单 Agent 还是多 Agent，增加角色何时有收益？（待编写） | basic |
| P-063 | AE-039 | 多 Agent 怎样分配控制权？（待编写） | basic |
| P-066 | AE-118 | 多 Agent 怎样通信、共享状态和避免重复工作？（待编写） | intermediate |

## MCP / External Tools（5 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-070 | AE-006 | 接入一个 MCP Server 时，各角色和三类原语怎样协作？（待编写） | basic |
| P-071 | AE-120 | [MCP 和 Function Calling 有什么区别？](mcp-external-tools/ae-120-mcp-vs-function-calling.md) | basic |
| P-074 | AE-036 | MCP 的 Transport 层是如何工作的？（待编写） | intermediate |
| P-075 | AE-056 | MCP Server 应该如何进行权限控制？（待编写） | advanced |
| P-077 | AE-096 | MCP、A2A、Tool Calling 三者是什么关系？（待编写） | advanced |

## Production Engineering（6 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-078 | AE-010 | Demo Agent 和 Production Agent 最大区别是什么？（待编写） | basic |
| P-079 | AE-035 | 依赖抖动时怎样设超时、重试、熔断和降级？（待编写） | intermediate |
| P-083 | AE-060 | 怎样把 Agent 权限控制和安全审计串起来？（待编写） | intermediate |
| P-084 | AE-089 | 怎样管理 Agent 的 Token 和任务成本预算？（待编写） | intermediate |
| P-086 | AE-090 | Agent 如何处理高并发？（待编写） | advanced |
| P-087 | AE-050 | Agent 异步任务怎样处理取消、重试和幂等？（待编写） | advanced |

## Agent System Design（8 题）

| 来源编号 | 永久 ID | 题目 | 难度 |
| --- | --- | --- | --- |
| P-090 | AE-007 | Deep Research Agent 怎样从单 Agent 演进到多 Agent？（待编写） | system-design |
| P-091 | AE-124 | 怎样设计一个能安全改代码的 Coding Agent？（待编写） | system-design |
| P-092 | AE-037 | 如何设计一个 Browser Agent？（待编写） | system-design |
| P-093 | AE-017 | 如何设计一个企业知识库 Agent？（待编写） | system-design |
| P-094 | AE-027 | 如何设计一个 Data Analyst Agent？（待编写） | system-design |
| P-095 | AE-077 | 如何设计一个 Agent Memory System？（待编写） | system-design |
| P-097 | AE-087 | 如何设计一个 Agent Eval Platform？（待编写） | system-design |
| P-098 | AE-057 | 如何设计一个支持 MCP 的 Agent Runtime？（待编写） | system-design |
