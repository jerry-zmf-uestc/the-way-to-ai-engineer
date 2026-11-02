# ARCHITECTURE

本文档简述 `the-way-to-ai-engineer` 的系统边界。产品目标、课程路线、章节状态、仓库结构和完整实施方案统一维护在[项目总体方案](docs/designs/project-plan.md)，本文件不再维护平行设计。

## 1. 产品概览

项目通过三部分课程建立从 Go 语言到高性能后端、再到 Agent 应用开发的连续能力。正文与独立实验按章节组织，四类作品级 Lab 分别验证推理与恢复、Memory/RAG、多 Agent 调度以及 Eval/可观测性；它们不是一个持续演进平台。

关键假设：

- Go 是后端和 Agent Runtime 的主语言。
- TypeScript/React 用于 Agent Console、Trace、Replay 和 Eval 界面。
- 外部地图、天气、搜索和 LLM 都必须位于可替换适配器之后。
- 默认开发和测试不依赖真实外部服务。
- 当前 Astro 站点已具备最小生成和构建链路，完整 Agent Runtime、多租户认证和生产部署仍未实现。

面试合集位于 content/interview/，按十大章节保存单题正文；题库与状态直接保存在 content/interview，设计、规则、模板、工具和测试纳入主项目。按问题关联同一仓库的课程与 Lab，研究和逐篇审核保存在仓库外。使用独立Astro配置构建面试站点，共享Pages的 /interview/ 子路径；发行预览和平台回执保存在仓库外，授权仅到保存草稿。

## 2. 目标分层

```text
content/book ──────── 解释概念与学习顺序
  │
  ├── labs ────────── 独立、可运行、可测试的机制实验
  │
  └── 作品级 Lab ───── 推理 / Memory-RAG / 多 Agent / Eval
```

目标应用中的依赖方向：

```text
transport → application → domain
    │              │
    └── adapters ← ports
          │
          ├── MySQL / Redis
          ├── Map / Weather / Search
          └── LLM / MCP / Vector Store
```

领域层不得依赖具体框架或供应商 SDK。

## 3. 信任边界

当前是独立教学示例，默认验证不依赖真实外部服务；后续引入应用运行时后必须显式处理以下边界：

| 边界 | 主要风险 | 目标控制 |
| --- | --- | --- |
| 浏览器/CLI → API | 未验证输入、越权 | Schema 校验、资源所有权检查、限流 |
| API → 数据库/Redis | 数据泄漏、跨用户访问 | 服务端授权、参数化查询、最小权限 |
| 服务 → 外部 Provider | 凭据泄漏、超时、费用 | 服务端变量、超时、重试预算、熔断 |
| Agent → Tool | Prompt 注入、越权副作用 | 工具白名单、结构化参数、审批与审计 |

认证、session 和 claim 尚未设计；在引入多用户数据前必须新增专门 spec，并在现有安全规则中明确资源所有权检查。

## 4. 数据与副作用

- 课程正文、实验元数据和校验夹具属于仓库数据；本项目不管理学习记录。
- 旅行助手后续可能存储用户偏好、旅行计划和评估结果；当前尚无数据库或表。
- Agent 只生成建议。预订、支付、消息发送等不可逆操作不在当前范围。
- 真实 API 调用属于受控副作用，必须通过显式 E2E 开关触发。

## 5. 已知风险与假设

| 项目 | 当前证据 | 处理方式 |
| --- | --- | --- |
| Agent 实验来源和许可证不同 | [第三方说明](THIRD_PARTY_NOTICES.md) | 保留 `meta.yml`，不无来源复制 |
| 外部事实可能过期或不准确 | [Agent 安全边界](docs/designs/project-plan.md#agent-safety) | 标注来源与时间，模型输出不能直接产生副作用 |
| 课程章节和 Lab 尚未全部完成 | [章节状态](docs/designs/project-plan.md#chapter-map) | 区分代码、内容、真实集成和读者验收 |

## 6. 条件能力说明

- 当前无邮件发送，因此不创建 `emails.md`。
- 当前无定时或后台任务，因此不创建 `cron.md`。
- 当前无公开可索引应用页面，因此不创建 `seo.md`。
- 项目不包含学习推送、复盘同步或定时学习自动化；Agent 工具与 Memory 约束见总体方案和安全规则。

## 7. 相关文档

- [项目总体方案](docs/designs/project-plan.md)
- [内容来源](THIRD_PARTY_NOTICES.md)
- [安全与工具权限](docs/rules/security.md)
- [变量与秘密](docs/rules/security.md#variables)
- [测试与验收要求](AGENTS.md#testing-requirements)
- [章节写作规范](docs/rules/book-chapter-contract.md)
- [验证范围](docs/designs/project-plan.md#author-review)
