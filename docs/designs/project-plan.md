# the-way-to-ai-engineer 项目总体方案

本文档是项目愿景、课程结构、职业路线、仓库架构、章节状态、交付方式和长期边界的唯一设计入口。它描述当前有效方案，不保存迁移过程、已经完成的阶段计划或互相竞争的目录草案。

具体写作、Lab 和安全要求分别由 `docs/rules/` 维护；实际命令以根目录 `Makefile`、`package.json` 和 `labs/go.mod` 为准。

## 1. 产品定位

项目以 Go 为核心 Runtime 和后端语言，构建一条从语言基础、高性能后端到 Agent 系统工程的连续学习路径。正文负责解释机制和设计取舍，独立 Lab 负责运行、测试、故障复现和指标观察，Astro 站点负责发布阅读体验。

第一目标用户是已有软件工程经验、正在补充 Go、后端和 Agent 工程能力，希望形成可验证作品证据的开发者。TypeScript/React 用于 Console、Trace、Replay 和 Eval 界面；Python 只承担必要的 AI 生态互操作和框架对照，不重复实现 Go Runtime。

六个月目标是达到高级 Agent Fullstack / Runtime Engineer 的可面试水平，并形成 AI 架构设计基础。课程完成不能替代真实生产流量、事故处理、跨团队协作和长期演进经验，因此不承诺仅凭学习项目达到 Production-proven AI Architect 或 Staff/Principal 水平。

核心价值：

- 把分散的 Go、后端和 Agent 知识组织为一条递进路线；
- 每个关键概念都有可运行、可测试、可解释的独立实验；
- 默认使用 fake、固定数据和显式失败路径，真实 Provider 通过受控 E2E 启用；
- 用 Trace、Eval、故障注入、成本和架构决策证明能力，而不是只展示成功演示；
- 教材、代码、来源、发布和安全边界能够公开审计。

明确不做：

- 不设置毕业设计，不维护一个跨越全部章节持续演进的平台；
- 不创建与 `book/` 平行的 `course/` 正文；
- 不用前端视觉完成度代替 Runtime、可靠性和评估质量；
- 不把模型预训练、完整 RL Infra 或 Kubernetes 平台工程设为主线；
- 不实现学习推送、打卡、自动复盘或个人学习记录管理；
- 不实现旅行预订、支付、消息发送等不可逆业务工具。

## 2. 目标岗位能力

课程以高级 Agent 工程和 Agent 架构类岗位为能力基线，最终应覆盖以下八个方面。

| 能力域 | 必须形成的能力 | 可验证证据 |
| --- | --- | --- |
| Go 与系统基础 | 数据结构、并发、HTTP、测试、性能分析和工程组织 | race、benchmark、profile、边界测试 |
| 高性能后端 | SQL、事务、Redis、消息队列、RPC、容错、背压和观测 | 集成测试、压测、故障与恢复报告 |
| LLM 系统基础 | Token、Embedding、Attention、Transformer、KV Cache、采样、SFT、RLHF/DPO 的系统影响 | 机制实验、模型能力对照和架构说明 |
| Agent 推理 | ReAct、Plan-and-Execute、Reflection/Reflexion、Self-Ask 与有界搜索 | 同任务集下的成功率、步骤、延迟和成本对比 |
| Runtime 与工具 | Run/Step/Event、Tool Registry、参数校验、歧义处理、重试、取消和恢复 | 确定性 Trace、失败分类和恢复测试 |
| Memory 与 RAG | 工作上下文、Session、关系型长期记忆、向量记忆、多路召回、HyDE、重排和引用 | 隔离测试、Recall、引用有效率和检索延迟 |
| 多 Agent | 角色、路由、任务 DAG、Handoff、汇总、冲突、死锁、资源竞争和公平调度 | 循环检测、配额、背压、失败传播和对比 Eval |
| 评估与生产工程 | Trace、Replay、任务完成度、工具准确率、延迟、Token、成本、安全、SLO 和 Runbook | 版本化数据集、报告、部署与故障演练 |

### 框架策略

Go 主线采用自研最小 Runtime 和 Eino：先理解状态、调用和错误语义，再使用框架。为满足主流框架定制能力，增加一个小型 Python 对照 Lab，用相同任务集比较 Eino、自研 Runtime 与 LangGraph 或 LangChain；重点检查状态模型、扩展点、Memory、Trace、权限和失败行为，不复制开发三套产品。

### 算法策略

算法不单独堆成刷题课程，而是进入真实机制：调度使用堆、队列、DAG 和拓扑排序；缓存使用 LRU/LFU；分布式主题使用一致性哈希、限流和退避；规划使用有界图搜索；RAG 解释向量相似度和近似检索。面试所需的通用数据结构与算法仍需学习者在课程外持续练习。

### 能力边界

独立 Lab 可以证明机制实现和工程判断，不能证明系统已经承受真实业务规模。课程结束后若要竞争生产型架构岗位，还需要至少一种外部证据：真实 AI 产品经历、持续开源维护、真实用户使用、线上故障与容量记录，或跨团队架构落地。

## 3. 课程与作品结构

课程只有一个正文入口 `book/`，与 `labs/` 一一关联：

```text
book/                       labs/
连续、自然的技术正文   →   独立运行的机制实验或模板应用
                                  │
                                  ├── unit / integration / race
                                  ├── eval / adversarial cases
                                  └── trace / benchmark / failure evidence
```

三个 Part 的职责：

| Part | 章节 | 目标 |
| --- | --- | --- |
| Part 1 · Golang | 1–11 | 写出结构清晰、错误显式、并发可控并具有测试的 Go 程序与 HTTP 服务 |
| Part 2 · 高性能后端 | 12–21 | 理解数据、缓存、通信、容错、高并发和可观测性如何组成可靠服务 |
| Part 3 · Golang Agent | 22–33 | 构建、约束、测试、评估并服务化 Go Agent 与多 Agent Runtime |

课程不维护统一结业平台。需要组合机制时，使用以下四类相互独立的作品级 Lab，每个都能从干净环境单独启动：

1. **Agent 推理与恢复**：ReAct、Plan-and-Execute、Reflection、参数修复、重试、取消和预算对比。
2. **Memory 与高级 RAG**：短期上下文、PostgreSQL、Redis、Vector Store、混合召回、HyDE、重排和引用评估。
3. **多 Agent 调度**：角色路由、DAG、Handoff、冲突、死锁、配额、背压和失败传播。
4. **Eval 与可观测性**：Trace、Replay、版本化数据集、质量/效率/成本指标和回归门禁。

GoTrip 只作为可选旅行助手案例，用于说明工具白名单、外部事实、预算和用户偏好如何组合，不承担课程主线、毕业设计或持续集成职责。

<a id="chapter-map"></a>

## 4. 章节地图与真实状态

下表是正式章号、正文、核心 Lab 和四维状态的唯一人工维护视图。`book/chapters.json` 只管理站点页面元数据、阅读关系和发布状态，不重复维护实现、代码、内容与读者验证状态。

状态含义：

- `implementation_status`：`planned / partial / available`；
- `code_status`：`pending / passed / failed`；
- `content_status`：`pending / reviewed / needs-revision`；
- `reader_validation_status`：`pending / validated / needs-revision`。

### Part 1 · Go 基础

| Ch | Title | Book | Core labs | implementation_status | code_status | content_status | reader_validation_status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Go 环境与第一个程序 | `book/part1-golang/chapter01.md` | hello-world | available | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 2 | 数据类型 | `book/part1-golang/chapter02.md` | values-variables-constants、strings-runes、arrays-slices、maps、json | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 3 | 控制流与边界分支 | `book/part1-golang/chapter03.md` | control-flow | available | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 4 | 函数 | `book/part1-golang/chapter04.md` | defer-panic、function-values | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 5 | 错误处理与异常 | `book/part1-golang/chapter05.md` | functions-errors | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 6 | 结构体与方法 | `book/part1-golang/chapter06.md` | structs-methods-interfaces | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 7 | 接口、泛型与反射 | `book/part1-golang/chapter07.md` | reflection-basics、generics-basics | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 8 | 包和工具 | `book/part1-golang/chapter08.md` | packages-tools | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 9 | 并发编程 | `book/part1-golang/chapter09.md` | goroutine-channel-select、worker-pools | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 10 | Web 服务 | `book/part1-golang/chapter10.md` | http-middleware-client、http-server | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 11 | 工程实践与代码规范 | `book/part1-golang/chapter11.md` | code-review-comments、effective-go、json-api、module-layout、practice-json-api、project-layout、review-cli、uber-errors-wrap、uber-slice-preallocate、uber-string-builder | available | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |

### Part 2 · 后端进阶

| Ch | Title | Book | Core labs | implementation_status | code_status | content_status | reader_validation_status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 12 | MySQL、SQL、索引与 GORM | `book/part2-backend/chapter12.md` | gorm-mysql、repository-pattern | partial | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 13 | 事务、隔离级别与锁 | `book/part2-backend/chapter13.md` | transactions | partial | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 14 | Redis 缓存、限流与分布式锁 | `book/part2-backend/chapter14.md` | cache、rate-limit、distributed-lock | planned | pending | pending | pending | — |
| 15 | 消息队列与异步任务 | `book/part2-backend/chapter15.md` | queue、outbox、worker | planned | pending | pending | pending | — |
| 16 | Hertz API 工程 | `book/part2-backend/chapter16.md` | hertz-kitex-demo、hertz-kitex-real-chain、hertz-routing | available | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 17 | Kitex RPC 与契约演进 | `book/part2-backend/chapter17.md` | kitex-idl-handler、kitex-rpc | available | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 18 | 服务发现、配置与微服务边界 | `book/part2-backend/chapter18.md` | discovery、config | planned | pending | pending | pending | — |
| 19 | 幂等、重试、超时、熔断与降级 | `book/part2-backend/chapter19.md` | idempotency、retry、circuit-breaker | planned | pending | pending | pending | — |
| 20 | 高并发、pprof、背压与压测 | `book/part2-backend/chapter20.md` | pprof、load-test、backpressure | planned | pending | pending | pending | — |
| 21 | 日志、指标与分布式追踪 | `book/part2-backend/chapter21.md` | service-runtime | partial | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |

### Part 3 · Agent 开发

| Ch | Title | Book | Core labs | implementation_status | code_status | content_status | reader_validation_status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 22 | LLM API、原理、Message 与结构化输出 | `book/part3-agent/chapter22.md` | hello-llm、structured-output | partial | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 23 | Tool Calling 与最小 Agent Loop | `book/part3-agent/chapter23.md` | tool-calling、minimal-agent-loop | partial | passed | pending | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 24 | ReAct、Plan-and-Execute 与 Reflection | `book/part3-agent/chapter24.md` | react、plan-execute、reflection | planned | pending | pending | pending | — |
| 25 | 从零实现 Agent Runtime | `book/part3-agent/chapter25.md` | model、tool-registry、runner | planned | pending | pending | pending | — |
| 26 | Eino 基础、工作流与框架对照 | `book/part3-agent/chapter26.md` | eino-basics | partial | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 27 | 短期、关系型与向量记忆 | `book/part3-agent/chapter27.md` | short-memory、user-memory | planned | pending | pending | pending | — |
| 28 | Embedding、Vector Store 与高级 RAG | `book/part3-agent/chapter28.md` | eino-rag-project、eino-vector-store | available | passed | reviewed | pending | code: [checks](../../Makefile); content: [author-review](project-plan.md#author-review) |
| 29 | 上下文工程 | `book/part3-agent/chapter29.md` | context-builder、compression | planned | pending | pending | pending | — |
| 30 | MCP 与可插拔工具协议 | `book/part3-agent/chapter30.md` | mcp-client、mcp-server | planned | pending | pending | pending | — |
| 31 | 多 Agent 协作、冲突与调度 | `book/part3-agent/chapter31.md` | async-agent、orchestrator | planned | pending | pending | pending | — |
| 32 | 状态恢复、流式输出与服务化 | `book/part3-agent/chapter32.md` | mini-agent-console | planned | passed | pending | pending | code: [checks](../../Makefile) |
| 33 | Agent 评估、安全、成本与回归 | `book/part3-agent/chapter33.md` | eval-harness、trace、cost | planned | pending | pending | pending | — |

planned 行中的路径表示正式规划位置，不声称文件已经存在。代码 `passed` 时核心 Lab 必须存在；`available` 或 `reviewed` 时正文必须存在。章节、Lab 或状态变化必须同步本表。

<a id="author-review"></a>

### 作者审校与读者验证

自动检查只证明结构、链接、代码和部分行为，不证明内容足够清晰，也不证明学习者已经掌握。`content_status` 只能由作者按章节审查表确认；`reader_validation_status` 只能记录真实读者从干净环境完成后的反馈。目前所有章节的读者验证均为 pending。

第 12、13、21、22、23、26 标为 partial，分别仍需补充真实数据库/锁、完整观测、LLM 原理、工具失败语义和框架工作流。第 28 章已有基础向量检索实验，但 HyDE、多路召回、重排和引用评估仍属于后续深度。

<a id="six-month-roadmap"></a>

## 5. 六个月学习与作品路线

24 周路线按能力依赖组织，不要求一周对应一章，也不要求 Lab 复用上一周的工作区。

| 周次 | 阅读与实验 | 主要证据 |
| --- | --- | --- |
| W01 | Token、Transformer 推理、结构化输出 | 最小机制实验；坏 JSON 与 schema 失败可见 |
| W02 | Provider、Streaming、取消与成本 | 正常、断流、超时；token/cost 记录 |
| W03 | Tool Calling、Session 与参数校验 | 未知工具拒绝；坏参数不执行 |
| W04 | Mini Agent Console | 从干净环境启动；端到端链路可测 |
| W05 | Run、Step、Event、Session 状态模型 | fake 模型下 Trace 确定可断言 |
| W06 | ReAct 与 Plan-and-Execute | 同任务集比较成功率、步骤和成本 |
| W07 | Reflection、Self-Ask 与错误恢复 | 循环上限、修复、重试和取消可测 |
| W08 | 自研 Runtime、Eino 与 LangGraph/LangChain 对照 | 同 Eval 下说明默认行为和扩展边界 |
| W09 | Context Builder 与短期记忆 | selection、trim、compression 信息损失 |
| W10 | 关系型、Redis 与向量长期记忆 | 用户隔离、过期、删除和恢复 |
| W11 | 高级 RAG | 多路召回、HyDE、重排、引用有效率 |
| W12 | MCP、工具发现与权限 | 多 Server、命名冲突、断连和白名单 |
| W13 | Skill、Policy、HITL 与 Sandbox | 权限不扩大；危险操作中断；资源销毁 |
| W14 | Coding Agent 模板 | 隔离环境内编辑、测试、构建和攻击用例 |
| W15 | Task DAG、角色、Router 与 Handoff | 循环依赖、最小上下文和失败传播 |
| W16 | 多 Agent Scheduler | 冲突、死锁、配额、背压、公平性和取消 |
| W17 | Event、Trace 与 Replay | 跨组件关联；固定输入重放失败 |
| W18 | Eval 数据集与 Runner | development、regression、adversarial 分离 |
| W19 | 工具、RAG 与多 Agent 指标 | 完成度、准确率、恢复率、步骤效率 |
| W20 | Model Router 与调优 | fallback、熔断、提示/结构化输出对照，累计不少于 100 cases |
| W21 | PostgreSQL Persistence 与 Checkpoint | migration、幂等、中断恢复 |
| W22 | Queue、Worker、Heartbeat 与高并发 | Worker 崩溃、重复投递、lease 过期可恢复 |
| W23 | 多租户、安全与可观测性 | RBAC、审计、配额、secret/PII 脱敏 |
| W24 | 部署、压测、成本与故障案例 | SLO、容量、成本、恢复和 Runbook |

每周完成当前章节和独立 Lab：从干净目录启动，复现正常路径和至少一个失败路径，完成练习，记录指标和一项架构取舍。周次、验收人、修改文件和 CI 矩阵不写进章节正文。

六个月结束时至少应展示：真实模型与工具的受控 E2E、确定性 Runtime、四类作品级 Lab、100 个以上版本化 Eval cases、暂停/取消/恢复/重放、延迟与成本报告、负载与故障实验、Threat Model、SLO、Runbook，以及关键设计的替代方案与适用边界。

<a id="repository-structure"></a>

## 6. 仓库和依赖架构

```text
the-way-to-ai-engineer/
├── book/                         # 唯一正文真源
│   ├── chapters.json             # 页面元数据、顺序、发布状态与 Lab 映射
│   └── partN-*/chapterNN.md
├── labs/                         # 独立机制实验和模板应用
│   ├── go.mod                    # 全部 Go Lab 的 module 边界
│   ├── go.sum
│   ├── internal/                 # 仅供多个 Lab 使用，按需创建
│   └── partN-*/chapterNN/<topic>/
├── public/                       # 原样发布的静态资源
├── src/                          # Astro 展示层
│   ├── content/chapters/         # 构建生成，禁止手改
│   ├── components/
│   ├── layouts/
│   ├── pages/
│   └── styles/
├── scripts/                      # 内容生成、验证、发布与安全入口
├── tests/                        # 跨章节和检查器回归夹具
├── docs/{designs,rules,templates}/
├── package.json                  # Astro 与内容工具
├── package-lock.json
├── astro.config.mjs
├── Makefile                      # 全仓编排入口
└── AGENTS.md                     # 贡献与执行约束入口
```

根目录是课程仓库编排层，不是 Go module。根 Node 工程只管理站点和内容工具；全部 Go Lab 使用 `labs/go.mod`。根目录不创建 `go.mod` 或 `go.work`，topic 不重复创建 module。未来若批准独立应用，它拥有自己的 module，不导入 `labs/internal`；只有出现真实跨 module 联调需求时才评估 `go.work`。

依赖规则：

| From | May depend on | Must not depend on |
| --- | --- | --- |
| `book` | Lab 的仓库相对链接、`public` 资源 | 生成内容、本机绝对路径、私有规格 |
| `scripts/prep-book.mjs` | `book/chapters.json`、正文、Lab 元数据 | 改写或润色教学内容、联网抓取正文 |
| `src` | 生成 collection、公开元数据、`public` | 成为正文或课程状态的真源 |
| `labs` | 标准库、`labs/go.mod` 依赖、稳定 `labs/internal` | 其他 topic 的私有实现、上一章运行状态 |
| 独立应用 | 自己的 domain、ports 和 adapters | `labs/internal`、模型驱动的领域规则 |

复杂 Lab 可以包含 Go、Python 和 TypeScript 子目录，但只创建实际需要的层次。单机制 Lab 保持 `README.md + meta.yml + source + tests` 的最小形态；不为目录整齐提前创建 `pkg`、`internal`、`web` 或微服务。

## 7. 内容、元数据与站点

### 单一真源

```text
book/chapters.json ─┐
book/**/*.md ───────┼─> scripts/prep-book.mjs ─> src/content/chapters/ ─> Astro ─> dist/
labs/**/meta.yml ───┘
public/**/* ──────────────────────────────────────────────────────────────┘
```

- `book/**/*.md` 是正文真源；
- `book/chapters.json` 管理页面 ID、路径、标题、Part、顺序、描述、发布状态、前置页面和关联 Lab；
- 本文档的章节地图管理课程实现和审校状态；
- `labs/**/meta.yml` 管理实验来源、入口、行为和验证命令；
- `src/content/chapters/`、`.astro/`、`dist/` 和 `node_modules/` 是生成或本地产物，不提交。

生成器只做可预测转换：校验清单和路径、移除布局重复渲染的首个 H1、重写 Pages base 下的资源路径、注入 schema frontmatter。它不得润色、重排或删除普通段落。只有 `published: true` 的页面生成 route。

### 正文质量

正文从真实技术压力进入，按理解顺序组织，不套“本周学什么、为什么学、改哪里、如何验收”的项目管理模板。代码、关键输出和观察点就近出现；完整程序、源码摘录、示意片段和故意失败例必须标明。

复杂章节至少给出：版本化事实与来源、相邻概念边界、可脱离模型或网络随机性的底层验证、失败类型及传播/恢复、安全强制执行点，以及按层排错路径。练习使用任务注释和关键代码表达，不用大段操作说明，也不直接填满答案。

### 站点体验

Astro 站点提供首页、按 Part 分组的章节卡片、窄栏中文排版、代码高亮、响应式目录、前后导航、明暗主题、Lab 卡片和“在 GitHub 编辑”。Markdown 必须在 GitHub 中仍可独立阅读，站点组件不能成为理解正文的前提。

当前站点已经具备最小生成与构建链路，`book/chapters.json` 只发布引言和第 28 章垂直切片。后续页面只有通过内容审校后才开放；搜索、PDF、视频和社交入口不是首要质量门槛。

<a id="agent-safety"></a>

## 8. Agent 架构和安全边界

Agent Runtime 的核心模型为 `Run → Step → Event`，围绕 Session、Model、Tool Registry、Tool Executor、Memory、Retriever、Scheduler、Checkpoint 和 Trace 组织。Provider、数据库、工具和框架 SDK 位于 adapter 之后，业务规则不由模型输出直接决定。

关键边界：

| 边界 | 风险 | 强制控制 |
| --- | --- | --- |
| 客户端 → API | 非法输入、越权 | schema、身份与资源所有权、限流 |
| Runtime → Model | prompt 泄漏、费用、不可控输出 | 最小上下文、预算、超时、结构化校验 |
| Agent → Tool | 注入、未知工具、坏参数、副作用 | 注册白名单、参数 schema、审批、审计 |
| Memory/RAG → 用户 | 跨用户泄漏、过期或错误事实 | tenant 隔离、授权、来源、时间和删除策略 |
| Worker → Queue/DB | 重复执行、丢失和提交结果不确定 | 幂等键、lease、checkpoint、outbox |
| 服务 → Provider | 凭据泄漏、SSRF、超时 | 服务端秘密、host allowlist、响应上限、熔断 |

默认 Agent 只读。未知工具、非法参数、超出轮次/Token/费用、跨用户 Memory、引用不可验证或输出 schema 失败时，不得执行持久化或外部写操作。客户端检查只改善体验，最终授权和参数校验必须在服务端。

真实 Provider E2E 只有在显式开关、服务端凭据、host allowlist 和调用预算齐备时运行；默认 CI 使用 fake 和固定数据。日志和 Trace 记录模型、prompt 版本、工具名、参数摘要、状态和耗时，不记录秘密、完整敏感输入或不受控模型内容。

可选 GoTrip 案例仅生成带来源和查询时间的旅行建议。天气、POI、路线等工具由应用显式注册，输出通过 schema 和事实约束后才能由用户主动保存；不注册交易工具。

## 9. Eval、性能与作品证据

Eval 数据集至少分为 `development`、`regression`、`adversarial` 和 `production-like`，固定输入、期望、评分器和版本。课程完成时累计不少于 100 个案例。

核心指标：

- Task Completion Rate；
- Tool Selection Accuracy 与 Argument Validity Rate；
- Recovery Rate、平均步骤数和人工介入率；
- RAG Recall、重排增益和 Citation Validity；
- Handoff 成功率、循环/死锁/超时率；
- P50/P95 延迟、Token 和每个成功任务的成本；
- Queue wait、Worker 利用率、恢复时间和错误预算。

指标必须能回到 Trace 和具体失败案例。模型输出波动不能被当作随机噪声忽略；开发集用于迭代，回归集用于门禁，对抗集验证权限和注入，production-like 集用于容量与成本判断。

每个作品级 Lab 需要 README、架构图或文字边界、运行命令、固定测试/Eval、至少一个故障实验、指标结果和已知限制。课程仓库、公开技术文章、架构决策和真实开源贡献可以构成岗位加分证据，但不得把本地测试描述成生产经历。

## 10. 验证、发布与完成定义

| 层级 | 自动验证 | 人工验证 |
| --- | --- | --- |
| Chapters manifest | schema、ID、顺序、路径、依赖 DAG | 标题、描述、学习顺序 |
| Book | 链接、代码摘录、命令、导航 | 叙事、深度、认知负荷、自然表达 |
| Lab | build、unit、race、失败路径 | 是否揭示目标机制 |
| Complex Lab | unit、integration、E2E、migration | 架构边界、隔离性、可运行性 |
| Eval/Security | regression、adversarial、budget、policy | 评分有效性、威胁模型 |
| Site | schema、build、assets、routes、base path | 导航、移动端、可访问性 |
| Reader | 自动化不能替代 | 从空环境完成并记录阻塞点 |

章节完成要求：正文自然进入问题，机制、代码、观察和边界相互承接；正常、边界和失败路径有证据；清单与 Lab 链接真实；没有秘密、绝对路径或迁移记录；作者审校和读者验证真实区分。

作品级 Lab 完成要求：从干净环境独立复现；默认离线测试通过；真实 E2E 有预算和开关；包含 Trace/Eval、故障或安全实验、指标与已知限制；不依赖上一周状态。

六个月路线完成要求：24 周都能解析到真实章节、Lab 和命令；四类作品级 Lab 可复现；Eval、恢复、安全、容量和成本证据完整；至少两个阶段获得真实读者反馈；能清楚说明仍缺少的生产规模和组织经验。

公开发布还必须通过 `docs/release-readiness.json`：许可和第三方归属、私密漏洞入口、社区联系方式、远端 CI/保护规则以及真人试学均有真实证据。自动脚本不能替代授权、远端设置或真人验收。

## 11. 实施优先级

当前基础：Part 1 代码和正文主体已建立；Part 2 已有数据库机制、Hertz、Kitex 和服务运行实验；Part 3 已有 Eino 与基础向量检索；Astro 已完成最小内容生成、页面和 Pages 工作流。所有读者验证仍待真实试学。

下一阶段按以下顺序推进，不再使用已经完成的 Phase 0/Phase 1 迁移计划：

1. **P0 · Agent 核心**：补完 Chapter 22–25，覆盖 LLM 原理、Tool Calling、ReAct/Plan/Reflection、自研 Runtime 和失败恢复。
2. **P1 · Memory 与 RAG**：完成 Chapter 27–29，将关系型、Redis、向量记忆与高级检索落到独立作品 Lab。
3. **P2 · 协议与多 Agent**：完成 Chapter 30–32，覆盖 MCP、Sandbox、DAG、Handoff、死锁、调度、checkpoint 和服务化。
4. **P3 · Eval 与治理**：完成 Chapter 33 和 100+ cases，再补 Model Router、Trace/Replay、成本、安全与多租户。
5. **P4 · 后端缺口**：并行补齐 Chapter 14、15、18–21 的 Redis、MQ、可靠性、高并发和分布式观测，为 Runtime 提供真实基础设施证据。
6. **P5 · 发布与验证**：逐章开放站点页面，完成真人试学、第三方核对、远端 GitHub 安全入口和 CI 保护验证。

每一优先级只在对应章节、Lab 和验证真实完成后更新状态。目录存在、代码能编译或页面能构建，都不能单独把内容或读者状态标为完成。

## 12. 方案维护

以下变化必须同步本文档：产品范围、正式章节、课程路线、目录/module 边界、单一真源、发布架构、Agent 权限、完成定义或长期能力目标。专项写作、Lab 元数据、安全变量和日常命令分别留在对应 rules、templates、源码和 AGENTS，不在本文复制第二份实现细节。

进度、临时排查、一次性迁移步骤和验收输出写入 Issue/PR 或私有 `.specs`，不追加到长期方案。新设计内容应合并进合适章节并删除失效表述，不通过继续拆分设计文件来记录历史。
