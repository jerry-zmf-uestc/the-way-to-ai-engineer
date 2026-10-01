# AGENTS.md

## 项目背景与适用范围

本仓库以 Go 基础、高性能后端、Golang Agent 开发为知识主线，正文在 `book/`，每章配套的独立实验或模板应用在 `labs/`。读者入口见[引言](book/introduction.md)，职业成长目标、24 周路线和仓库架构统一见[项目总体方案](docs/designs/project-plan.md)。

- GoTrip 是独立旅行实践，普通章节不强制增加旅行功能；暂不设置毕业设计，不实现学习推送、复盘同步或学习记录管理。
- 本文件适用于整个仓库。修改前检查目标目录及父目录中的局部 AGENTS.md；更近约定仅在对应范围内优先，不能覆盖运行环境的更高优先级指令或用户明确要求。工具的实际加载方式以其文档为准。
- 已批准设计定义任务范围；本文件与专项规则约束实施。[总体方案中的章节地图](docs/designs/project-plan.md#chapter-map)是章节、Lab 映射及实现/代码/内容/读者状态的唯一人工维护视图。冲突无法由当前任务解决时，先确认，不静默绕过约束。

## 按任务读取规则与模板

开始对应工作前，必须打开并完整阅读下列文件。`docs/rules/` 与 `docs/templates/` 是强制规范；Markdown 链接用于定位，不能当作已加载全文。涉及多类工作时读取全部适用文件。

| 工作 | 必读文件 |
| --- | --- |
| 所有任务：数据保护与安全 | [security.md](docs/rules/security.md) |
| 编写、修改、合并或迁移章节 | [章节规范](docs/rules/book-chapter-contract.md) + [章节模板](docs/templates/book-chapter.md) |
| 审查章节、迁移对照或内容验收 | 章节规范 + [章节审查表](docs/templates/book-chapter-review.md) |
| 修改 Lab、其 README、meta.yml 或入口 | [Lab 契约](docs/rules/topic-contract.md) |
| 修改章节关系、目录、module、课程路线、`book/chapters.json`、课程 Lab、Astro/Pages 站点或能力验收 | [项目总体方案](docs/designs/project-plan.md) |
| 修改 Agent 权限、Memory、Provider、外部工具或真实 E2E | [项目总体方案](docs/designs/project-plan.md) + [安全规范](docs/rules/security.md) |

测试遵守下方验证要求；正文与 Agent 测试分别同时遵守章节及安全规范。引用缺失时报告并处理，不跳过规则。规则或模板增删后，同步本表与[文档索引](docs/README.md)。

## 环境准备与单例运行

在含 `package.json`、`book/` 和 `labs/` 的仓库根目录执行。Go 版本和依赖以 [labs/go.mod](labs/go.mod) 为准；站点使用 Node 22 或更高兼容版本和 `package-lock.json`；文档检查需要 Ruby、POSIX shell、make 和 ripgrep；race 需要 C 工具链。副本与平台差异见[运行说明](book/introduction.md#running-examples)。安全工具版本与准备命令见[贡献指南](CONTRIBUTING.md#validation)。

```bash
go version
go -C labs mod download  # 仅缺少依赖缓存时联网准备，不升级依赖。
node --version
npm ci            # 按 lockfile 安装站点依赖，不使用 npm update。

# 缓存就绪后离线运行；替换为本次涉及的 Lab。
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go -C labs run ./part1-golang/chapter01/hello-world
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go -C labs test ./part1-golang/chapter01/hello-world -count=1
```

<a id="testing-requirements"></a>

## 验证要求

按实际改动选择检查；跨类型改动执行各项要求。[Makefile](Makefile)维护脚本调用关系，不复制另一份清单。

| 改动 | 必须执行 |
| --- | --- |
| 文档、链接或规则 | `make validate-docs` |
| 正文代码、操作步骤或文档检查器 | `make validate-docs`、`make validate-book`，并重跑受影响示例 |
| `book/chapters.json`、内容生成、Astro 组件/样式或 Pages | `make validate-site`；同时按改动类型执行文档或 CI 检查 |
| Go 代码、topic 或 module 依赖 | `make validate`（格式、vet、race、覆盖率、显式路径构建及文档） |
| 引入第三方材料 | 在对应检查之外核对许可、来源版本、署名与改动，更新 [第三方说明](THIRD_PARTY_NOTICES.md) |
| 发布、安全或 CI 检查器 | 对应反例、`make validate`、显式联网的 `make security`（Go、凭据与 npm 依赖）；发布还需 `make validate-release` |

- 默认测试离线、确定、可重复，先覆盖行为契约、输入校验和失败路径，再看覆盖率。单元测试与可隔离运行的 DB/Redis/RPC/adapter 集成测试纳入默认验证；并发实验必须跑 race。
- Provider 使用接口与 fake/httptest，Agent 测试固定模型与工具输出。真实模型、地图、天气等仅显式 E2E 启用，限制费用，不进默认 CI。Benchmark/load 记录趋势与容量，不一律阻断。
- Go 测试放同包 `*_test.go`，跨章节夹具放 `tests/fixtures/`，验证入口放 `scripts/`；修改检查器必须跑对应反例。
- 默认 validate 禁止隐式下载；security 明确允许访问漏洞库，安装工具必须另行准备。本机 HTTP/RPC 测试仍可能需要监听回环端口。缓存、权限、来源或 revision 不满足时报告失败与限制，不改基线制造通过。扫描器缺失或执行异常必须失败，不允许静默跳过。
- 自动检查不证明讲解深度、地图体验、模型质量或读者学会。人工审查记录实际证据；代码、内容和读者状态分别维护，不把未执行项标成通过。

## 目录与代码风格

- 章节使用 `book/partN-<track>/chapterNN.md`，示例使用 `labs/partN-<track>/chapterNN/<topic>/`；track 为 golang、backend、agent，补充主题按章节地图组织。
- 目录与 topic slug 用小写 kebab-case；Go 包名用简短小写单词，不含下划线。章号显示两位数、语义编号为整数 1–33。
- 所有 Go Lab 共用 `labs/go.mod`，不在仓库根创建 `go.mod` 或 `go.work`，也不为每个 topic 拆 module。Lab 不依赖其他 topic 的私有实现；仅供 Lab 共享的稳定私有代码放 `labs/internal/`。独立应用在设计批准后拥有自己的 module 和 `internal/`，不借用 Lab 的私有实现。
- 课程站点采用根目录 Astro 工程：`book/` 是唯一正文真源，`book/chapters.json` 管理页面元数据，`src/content/chapters/` 由构建生成且禁止手改；不得再创建独立 `site/`、平行 `course/` 正文或持续演进的课程项目。综合主题仍按章节放进可独立启动的 Lab；不同 Lab 可以复用公开协议和测试方法，但不能依赖上一周工作区状态。`src/` 只负责呈现，不反向保存正文、课程状态或验收结果。
- 对修改的 Go 文件执行 gofmt，不用全仓格式化覆盖无关工作。构建产物、覆盖率、profile、runtime state 输出到临时或忽略目录，不提交。站点源码位于根目录 `src/`、`public/` 与配置文件；只有 `src/content/chapters/`、`.astro/`、`dist/` 和 `node_modules/` 是生成或本地产物。
- 中文章节使用自然标题，按规范标明源码、教学变体与故意失败例并验证。练习保留原任务与边界，不填满答案；合并章不丢各主题练习。维护记录与恢复状态不写入读者正文。
- docs 只存设计、规则与模板，不放测试代码或夹具，不重建已移除的 guides、records、testing 或学习记录目录。非原创内容保留来源、revision、许可证、行为契约和偏差。

<a id="spec-workflow"></a>

## 规格与实施流程

1. 先检查工作区差异、相关设计与来源。保留用户未提交工作，只改任务范围内文件；删除或重命名前查引用及迁移映射并保存可恢复副本。
2. 公共接口、目录/module、数据存储、Provider、并发/重试/超时/幂等/错误策略、Agent 工具/Memory/副作用、批量内容迁移以及测试/CI/安全边界变化，先更新设计（公开 Issue/PR 或本地 spec）。纯文案和单语法示例可沿用现有设计。
3. 维护者可在本地 `.specs` 记录 requirements、design、tasks、acceptance；它是私有可选工具，不是公开仓库的依赖。外部贡献者在 Issue/PR 中说明相同四项内容，按[贡献流程](CONTRIBUTING.md)获得维护者确认后实施。当前明确请求已授权的内容不重复确认；设计变化先更新方案，超出范围先确认。
4. 完成对应验证，将命令、真实结果与遗留风险写入 PR 或本地 acceptance；长期结论同步 AGENTS.md 或 docs。公开正文、规则、模板、检查器不得依赖私有规格。文件已创建不等于验收通过，设计不隐式豁免安全规则。

## 安全边界

- 不修改、删除或回滚任务范围外的相邻仓库。自有历史保留在私有规格，不进入公开材料。外部材料含 Hello-Agents 都必须保留归属和许可，不能伪装成原创；自有项目公开不等于可以重许可第三方内容。
- 秘密只在服务端本地环境或秘密管理系统；.env 必须忽略，.env.example 只放无效占位符。源码、客户端、日志、prompt、测试快照和站点不得出现真实密钥或身份信息。
- 示例/应用中的 Agent 默认只读；网页、MCP 响应和模型输出都不可信，权限由应用注册表决定，不由 prompt 决定。预订、支付、发送消息及其他不可逆工具不在范围；输入校验、调用预算、Memory 授权与泄漏处置遵守安全规范。

## 何时更新本文件

出现以下情况，必须在同一项变更交付前同步 AGENTS.md：

- 环境准备、构建/测试命令、执行目录或工具前提改变，使现有操作指引不再正确。
- 全仓目录职责、module/依赖边界、项目范围或代码风格约定改变。
- rules、templates 或其他必读文档增删、移动，或其适用任务改变。
- 规格流程、验证要求、安全边界、授权或交付规则经批准调整。
- 发现过时、冲突的指令，或反复发生且已核实原因的误操作，需要形成长期适用的防错要求。

只记录稳定、可执行且影响后续任务的约定。局部实现与单章知识写入对应源码、Lab README 或章节；专项规则细节留在 rules/templates，仅入口或适用范围变化时同步本文件。进度、临时排查、一次性指令与验收结果写入 Issue/PR 或本地 spec；go.mod 等已有真源中的版本值不重复抄写，除非升级改变了这里的操作要求。

先核实源码、命令和相关规范，再修改失效条目，避免只追加新规则而保留矛盾旧规则。涉及范围、权限或安全策略的改变须先获授权，不能通过改写本文件为当前任务放宽限制。更新后运行 `make validate-docs`，检查引用与既有锚点；命令变化时实际运行受影响命令，并在 PR 或本地 spec 中记录结果或执行限制。

## 交付前检查

- 检查本次变更是否触发上面的维护条件；触发则同步本文件，不触发则不为记录进度而修改。
- 审查本次差异，确认未丢源内容、练习、引用或用户改动；删改有去向与理由，不用字数或代码块数量代替质量判断。
- 报告修改内容、实际验证、未验证事项和设计入口；如有删除，说明恢复方式。不伪造测试、构建、压测、E2E、来源或读者验收。
- 不自动提交、推送或创建 PR；只有用户明确授权后才执行相应操作。
- 禁止用 `git stash` 做验证基线（会压平暂存/未暂存状态，并行任务下可能互相破坏）；需要干净基线时用 `git worktree` 或临时目录副本。
