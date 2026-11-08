# Go AI 全栈学习

从 Go 程序开始，学习后端服务的实现与运行，再把模型、检索、执行控制和评估组件接成可测试的 Agent 应用。正文、示例和练习配套阅读；旅行助手放在独立项目中，不作为每章必做任务。

本书默认读者已经熟悉 Web 前端与基础工程协作，希望把已有的 TypeScript/React 经验迁移到 Go 后端、Agent Runtime、RAG 和运行治理。第一次使用本书，先读[学习方法与运行约定](introduction.md)，再完成[环境准备](part1-golang/chapter01.md)。

如果你想先看一条“浏览器页面 → Go 服务 → Agent 执行 → 证据与终态”的完整链路，先体验[Agent 架构与模型接口](part3-agent/building/chapter22-architecture.md#topic-harness-intro)和[第33章：服务接入与流式交互](part3-agent/runtime/chapter33-serving.md#topic-ch32-streaming-service)，再回到第 1 章补齐 Go 基础。

## Go 基础

1. [Go 环境与第一个程序](part1-golang/chapter01.md)
2. [数据类型](part1-golang/chapter02.md)
3. [控制流与边界分支](part1-golang/chapter03.md)
4. [函数](part1-golang/chapter04.md)
5. [错误处理与异常](part1-golang/chapter05.md)
6. [结构体与方法](part1-golang/chapter06.md)
7. [接口、泛型与反射](part1-golang/chapter07.md)
8. [包和工具](part1-golang/chapter08.md)
9. [并发编程](part1-golang/chapter09.md)
10. [Web 服务](part1-golang/chapter10.md)
11. [工程实践与代码规范](part1-golang/chapter11.md)

<a id="backend-roadmap"></a>

## 后端实践 · 从点击导出到故障恢复

先在第 12 章打开报表导出页面。提交返回任务 ID，结果晚一点才出现。接下来的十章沿同一个问题往下追：谁能查到这份任务，哪一步已经持久化，发生故障后系统凭什么继续执行。

- **请求与数据**：[12 请求边界与任务 API](part2-backend/chapter16.md) → [13 数据建模、SQL 与索引](part2-backend/chapter12.md) → [14 事务、隔离与并发写入](part2-backend/chapter13.md)。
- **异步与一致性**：[15 任务队列、投递与恢复](part2-backend/chapter15.md) → [16 缓存、读路径与一致性](part2-backend/chapter14.md)。
- **服务与故障**：[17 远程调用、契约与服务边界](part2-backend/chapter17.md) → [18 超时、结果未知与幂等](part2-backend/chapter19.md) → [19 并发、背压与过载保护](part2-backend/chapter20.md)。
- **运行与排障**：[20 可观测性与事故诊断](part2-backend/chapter18.md) → [21 发布、关闭与综合演练](part2-backend/chapter21.md)。

浏览器实验默认使用内存模型。数据库三项实验证据分别来自 SQLite 执行计划、独立连接和独立 worker 进程。章末按各自的验收步骤检查结果，再完成 [综合演练](part2-backend/chapter21.md#report-export-flow)。

## Part 3 · Golang Agent

构建篇组织决策、信息与行动；运行时承载状态、恢复与协作；治理篇提供观测、质量、安全和资源控制。共16章，实验保留独立运行入口。

### 构建篇

- [第22章：Agent 架构与模型接口](part3-agent/building/chapter22-architecture.md)：建立模型接口与Harness的共同语言，区分推理、执行、状态与任务验收。
- [第23章：工具接入与受控行动](part3-agent/building/chapter23-actions.md)：将工具与MCP能力接入同一行动链，校验身份、参数、审批和观察。
- [第24章：Agent 决策循环与 ReAct](part3-agent/building/chapter24-decision-loop.md)：用动作与观察推进多轮决策，限制重复行动并比较ReAct与简单流程。
- [第25章：任务规划与反馈修订](part3-agent/building/chapter25-planning.md)：拆解可验证步骤，根据反馈修订计划，判断反思与搜索的适用条件。
- [第26章：会话状态与上下文工程](part3-agent/building/chapter26-context.md)：保护会话事实与消息关联，按输入预算选择、裁剪和组织上下文。
- [第27章：检索增强生成与证据验证](part3-agent/building/chapter27-rag.md)：串联索引、检索、证据输入与生成，分别验证召回、引用和断言支持。
- [第28章：Skills 与领域能力复用](part3-agent/building/chapter28-skills.md)：按任务发现、选择和加载Skills，验证领域指导的采用与收益。

### 运行时

- [第29章：运行控制与故障恢复](part3-agent/runtime/chapter29-execution.md)：管理运行状态、预算、重试与取消，按提交事实恢复中断任务。
- [第30章：持久状态与工作空间](part3-agent/runtime/chapter30-state.md)：分别管理长期记忆与任务产物，核对授权、版本、有效性和生命周期。
- [第31章：多 Agent 协作与任务编排](part3-agent/runtime/chapter31-collaboration.md)：连接角色委派、Handoff与DAG调度，处理结果冲突、失败传播及资源公平。
- [第32章：异步任务与生命周期管理](part3-agent/runtime/chapter32-async-tasks.md)：让后台任务独立于请求存在，管理目标修订、取消竞争和迟到结果。
- [第33章：服务接入与流式交互](part3-agent/runtime/chapter33-serving.md)：提供有身份、幂等与容量约束的任务入口，用SSE呈现真实运行过程。

### 治理篇

- [第34章：可观测性与执行重放](part3-agent/governance/chapter34-observability.md)：关联事件与调用，录制原始响应并严格重放，逐层定位执行失败。
- [第35章：质量评估与持续改进](part3-agent/governance/chapter35-evaluation.md)：定义成功标准，建设数据集与评分器，通过对照和回归持续改进。
- [第36章：Agent 安全与权限治理](part3-agent/governance/chapter36-security.md)：建立信任与授权边界，约束不可信内容、执行环境及数据出口。
- [第37章：资源预算与成本优化](part3-agent/governance/chapter37-cost.md)：统一用量与费用口径，管理预留、未知和结算，以任务质量判断优化。
