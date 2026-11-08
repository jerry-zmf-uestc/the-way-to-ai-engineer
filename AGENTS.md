# AGENTS.md

## 项目背景与适用范围

本仓库以 Go 基础、高性能后端、Golang Agent 开发为知识主线，课程正文在 `content/book/`，每章配套的独立实验或模板应用在 `labs/`。读者入口见[引言](content/book/introduction.md)，职业成长目标、24 周路线和仓库架构统一见[项目总体方案](docs/designs/project-plan.md)。

- GoTrip 是独立旅行实践，普通章节不强制增加旅行功能；暂不设置毕业设计，不实现学习推送、复盘同步或学习记录管理。
- 本文件适用于整个仓库。修改前检查目标目录及父目录中的局部 AGENTS.md；更近约定仅在对应范围内优先，不能覆盖运行环境的更高优先级指令或用户明确要求。工具的实际加载方式以其文档为准。
- 已批准设计定义任务范围；本文件与专项规则约束实施。[总体方案中的章节地图](docs/designs/project-plan.md#chapter-map)是章节、Lab 映射及实现/代码/内容/读者状态的唯一人工维护视图。冲突无法由当前任务解决时，先确认，不静默绕过约束。

面试合集位于 `content/interview/`，按题库列表制作公众号科普文章，保持框架中立；公众号优先保存草稿并回读，按项目本期流程经官方读者预览和作者确认后归档唯一正文与正式素材，不默认另建在线协作文稿；网站按授权呈现同稿，不进入课程章号。`node scripts/interview/issue.mjs next` 是新稿入口，私有本期与归档、网站准入规则见 [内容规范](docs/rules/interview-content-contract.md) 和 [合集方案](docs/designs/interview-handbook.md)。课程与归档文章各自只有一个正文真源；网页上线公开正文，保存草稿不等于发表。

## 按任务读取规则与模板

开始对应工作前，必须打开并完整阅读下列文件。`docs/rules/` 与 `docs/templates/` 是强制规范；Markdown 链接用于定位，不能当作已加载全文。涉及多类工作时读取全部适用文件。

| 工作 | 必读文件 |
| --- | --- |
| 面试题、内容生产、题库与公众号草稿 | [面试内容规范](docs/rules/interview-content-contract.md) + [合集方案](docs/designs/interview-handbook.md) + [文章模板](docs/templates/interview-article.md) + [研究模板](docs/templates/interview-research.md) + [审核模板](docs/templates/interview-review.md)，同时遵守全仓安全规范 |
| 公众号渠道稿、草稿续接与验收 | [公众号规则](docs/rules/interview-wechat.md) + [渠道稿模板](docs/templates/interview-wechat-article.md)；以设计者问题和同案例递进组织技术讲解，核心判断同字号青绿加粗无底色，图注序号加短标题；最新公众号稿保存回读后再归档，不套用旧候选 |
| 绘制或迁移图示 | [图示规范](docs/rules/diagram-contract.md)，课程图保留现有重建契约 |
| 所有任务：数据保护与安全 | [security.md](docs/rules/security.md) |
| 编写、修改、合并或迁移课程章节 | [章节规范](docs/rules/book-chapter-contract.md) + [章节模板](docs/templates/book-chapter.md) |
| 审查课程章节、迁移对照或内容验收 | 章节规范 + [章节审查表](docs/templates/book-chapter-review.md) |
| 修改 Lab、其 README、meta.yml 或入口 | [Lab 契约](docs/rules/topic-contract.md) |
| 修改章节关系、目录、module、课程路线、`content/book/chapters.json`、课程 Lab、Astro/Pages 站点或能力验收 | [项目总体方案](docs/designs/project-plan.md) |
| 修改 Agent 权限、Memory、Provider、外部工具或真实 E2E | [项目总体方案](docs/designs/project-plan.md) + [安全规范](docs/rules/security.md) |

测试遵守下方验证要求；正文与 Agent 测试分别同时遵守章节及安全规范。引用缺失时报告并处理，不跳过规则。规则或模板增删后，同步本表与[文档索引](docs/README.md)。

## 环境准备与单例运行

在含 `package.json`、`content/book/` 和 `labs/` 的仓库根目录执行。Go 版本和依赖以 [labs/go.mod](labs/go.mod) 为准；站点使用 Node 22 或更高兼容版本和 `package-lock.json`，其内容验证也运行 Console 页面行为测试；文档检查需要 Ruby、Python 3（Agent摘录校验与反例）、POSIX shell、make 和 ripgrep；race 需要 C 工具链。副本与平台差异见[运行说明](content/book/introduction.md#running-examples)。npm run dev通过scripts/dev-book.mjs监督书稿扫描与Astro子进程；正文/清单/元数据变化触发去抖生成，失败保留上一版但终端报错。npm run preview通过scripts/preview-book.mjs复用同一子进程监督，仅服务dist、不生成或监听正文。两个入口关闭时清理自己的扫描与子进程，不使用background/ignore-lock，不改系统监听限制；端口占用停止而非开第二个服务。安全工具版本与准备命令见[贡献指南](CONTRIBUTING.md#validation)。

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
| 面试题库、状态、工具与模板 | `make validate-interview`；公共检查不读取私有审核；新本期另运行 `node scripts/interview/issue.mjs check AE-ID`，本地交付另运行 `node scripts/interview.mjs delivery-check AE-ID`，公众号本地登记另运行 `node scripts/interview.mjs wechat-check`。`node scripts/interview/export.mjs AE-ID --wechat` 仅生成摘要绑定的本地渠道预览，不操作公众号。公开文档/引用变化另跑 `make validate-docs`，检查器变化按类型跑反例 |
| 文档、链接或规则 | `make validate-docs` |
| 正文代码、操作步骤或文档检查器 | `make validate-docs`、`make validate-book`，并重跑受影响示例 |
| `content/book/chapters.json`、内容生成、Astro 组件/样式或 Pages | `make validate-site`；同时按改动类型执行文档或 CI 检查 |
| Go 代码、topic 或 module 依赖 | `make validate`（格式、vet、race、覆盖率、显式路径构建及文档） |
| 引入第三方材料 | 在对应检查之外核对许可、来源版本、署名与改动，更新 [第三方说明](THIRD_PARTY_NOTICES.md) |
| 发布、安全或 CI 检查器 | 对应反例、`make validate`、显式联网的 `make security`（Go、凭据与 npm 依赖）；发布还需 `make validate-release` |

- 默认测试离线、确定、可重复，先覆盖行为契约、输入校验和失败路径，再看覆盖率。单元测试与可隔离运行的 DB/Redis/RPC/adapter 集成测试纳入默认验证；并发实验必须跑 race。
- Provider 使用接口与 fake/httptest，Agent 测试固定模型与工具输出。Memory PostgreSQL集成仅用显式MEMORY_E2E=1、MEMORY_TEST_DATABASE=1及隔离memory_lab数据库；默认Fake不联网。pgvector显式对照仅用VECTOR_E2E=1、VECTOR_TEST_DATABASE=1和隔离vector_lab，默认Go固定向量不连数据库。本地神经Embedding仅在EMBEDDING_E2E=1加载已校验资产与独立Python runtime，默认不加载；模型数据/虚拟环境不入仓库。本地假想文档生成仅在HYPOTHETICAL_E2E=1与EMBEDDING_E2E=1同时启用，固定资产哈希及独立Python runtime，不加载远程代码；生成文本只作查询，不作为证据。RAG本地自由生成仅在RAG_GENERATION_E2E=1加载固定资产与独立runtime，截断草稿不得交付；批处理只用合成公开资料。Skills输入对照仅在SKILL_COMPARISON_E2E=1使用锁定Python与已校验本地Qwen资产，Go实际加载后只投影执行输入；评分字段不入模型，原始输出与截断保留，字段覆盖不等于任务质量。Python框架对照仅在FRAMEWORK_E2E=1使用锁定独立runtime，关闭外部Tracing；默认Go验证共同任务，运行不调用模型或网络。本地动作/初始计划生成、有界算术搜索及自然语言Self-Ask仅在LOCAL_STRATEGY_E2E=1使用STRATEGY_PYTHON/STRATEGY_ASSETS加载已校验Qwen资产，模型输入只含任务与已观察资料，不含评分Labels；过程失败、原始提议、Token与读取预算全部保留。受限候选似然模式记录每次前向与候选Token，OutputTokens为0，不将候选JSON当成生成usage或Provider账单。真实生成模型、地图、天气等仅显式 E2E 启用，限制费用，不进默认 CI。Benchmark/load 记录趋势与容量，不一律阻断。
- RAG联合草稿使用同一`RAG_GENERATION_E2E=1`门禁、锁定runtime及输入1024/输出192预算；只登记`rag-joint-v1/v2/v3`，v3将引用键投影为evidence_id并保存实际messages，不改原文或输出，显式`qwen3-1.7b`配置仅用于联合协议，默认单引用模式不变。生成位于RunJoint的输入授权检查之后；原始输出、截断、usage、错误与拒答保持原样，解析失败不补写引用。固定联合Judge、真实生成、通用语义及读者效果分别验收。 Eval的运行时验收对照复用同一门禁、GENERATION_PYTHON/GENERATION_ASSETS、锁定1.7B与rag-joint-v3公开JSON入口；应用冻结Harbor合成证据，失败差距作为普通question数据，最多2候选/总180秒，保留实际messages、原输出、截断和用量。 输入模式仅登记feedback/no-feedback，后者不附差距但仍保留验收，未登记值调用前拒绝；各次实际生成分开保存，旧记录无Mode时仅按原feedback读取。固定规则仅判断该任务，不加载其他topic私有代码或将回归记录计为新独立案例。
- 运行诊断真实生成仅在`DIAGNOSIS_GENERATION_E2E=1`使用GENERATION_PYTHON/GENERATION_ASSETS、完整generation lock及既有固定Qwen3-1.7B资产；公开JSON生成入口显式登记runtime-diagnosis-v1，不改变RAG默认提示。任务视图只有问题/事件/允许建议，评分字段拒绝；两批各4题、全流程360秒、每题输入1024/输出192、CPU单线程，保存实际messages/原文/截断/用量。建议不执行、原因无独立判断保持unknown，开发诊断不冒称独立验收或合格案例。默认仅纯投影与记录重放。
- 资源核算真实生成仅在`ACCOUNTING_GENERATION_E2E=1`复用GENERATION_PYTHON/ASSETS、完整generation lock与固定1.7B资产，显式resource-accounting-v1只读JSON入口；12题三批各4、360秒总时限、1024/192每题及CPU1线程。问题/事实不含Expected或规则代码；记录原文/messages/截断/本地Token，数值合同通过不冒称真实账单/语义校准，默认只投影反例与记录重放。
- Eval/可观测性集成真实生成仅在`OBSERVABILITY_GENERATION_E2E=1`使用GENERATION_PYTHON/ASSETS、完整generation lock及固定1.7B资产；只通过既有rag-joint-v3公开CLI，四条原合成任务的最终阶段各一次、420秒总时限、1024/192每次、CPU1线程。应用选择和固定只读工具分别标记来源；只提交实际观察，不含Expected/Critical/Split。原文/messages/截断/用量与适配结果分开保存，新记录路径排他创建；默认只验证投影及原记录重放，不调用模型。实际Tokenizer计数不作为Provider费用或通用语义证据。
- 同一观察生成门禁可显式设置OBSERVABILITY_GENERATION_MODE=protocol-repair：Schema5冻结每Case最多2次最终生成/全批最多8次，420秒与每次1024/192预算不变；只重试answer_protocol，原证据、错误、每次用量及原文不改，第二次仅追加稳定协议反馈和上一原文的引用数据，不提供隐藏期望。旧单次默认与记录保持；取消、超时、截断、来源异常及语义评分失败不自动修复，不把增加尝试的收益描述为同调用预算结果。
- Memory真实Redis缓存仅在`MEMORY_REDIS_E2E=1`使用应用指定的已校验绝对server/cli路径；实验启动自有私有Unix socket进程，禁TCP与持久化、限定资源、结束停止并等待清理。Redis中的Scope/字段键由应用绑定，缓存候选回查既有事实源；Redis单门禁对照使用Fake事实源；真实SQL联合还须`MEMORY_E2E=1`与`MEMORY_TEST_DATABASE=1`，核对隔离memory_lab，沿用SQLStore契约。缓存键隔离不替代身份认证，通知与性能分别验收。默认不启动外部进程。
- Delegation SQL租约仅在DELEGATION_LEASE_E2E=1、DELEGATION_TEST_DATABASE=1和隔离delegation_lab启用，DELEGATION_DSN由应用配置；默认只跑参数反例，不连接DB。新表仅保存课程合成任务的Owner/Epoch/到期/受控结果，数据库时钟与条件更新核对提交资格。父/子真实进程使用已登记辅助入口，到期仅测试管理连接受控注入，不宣称自然超时/分区；不打印DSN，不读取业务资料。原文件Handoff、预算与角色恢复保持，SQL机制不自动继承完整跨实例交接或真实身份。
- 同一Delegation SQL门禁另验证次数预算：自有Scope/Task预算表保存原Limit/Reserved，接管不清零，双行锁及资格检查控制预留；真实父子进程与并发请求使用固定辅助入口。Reserved不冒称实际发送或费用，未知确认不自动退款/重试；稳定尝试ID幂等与文件Handoff/费用整合另验收。默认不连接DB或运行辅助进程，不修改业务表。
- Memory实际输入生成仅在MEMORY_INPUT_GENERATION_E2E=1且MEMORY_E2E/MEMORY_TEST_DATABASE/MEMORY_REDIS_E2E/RAG_GENERATION_E2E同时开启；可信GENERATION_PYTHON/ASSETS、完整Python3.12.14 generation lock、既有Qwen1.7B及公开rag-joint-v3 CLI。相同SQL/Redis任务在更新状态、原记录仍存在的其他用户、删除后分别完成生成，再改变状态；180秒总时限、每项1024/192、CPU1。输入仅所选键/值/修订投影，保存messages、原文、截断与Tokenizer用量；记录路径排他创建，默认只检查投影与冻结记录。逐字偏好问答不作为实际表达风格采用、通用语义、真实授权或完整知识作品验收。
- 同一Memory输入生成门禁可显式设置MEMORY_INPUT_VIEW=key-value：仅把已回查投影表示为Key=Value证据正文，ID/修订仍单独绑定，问题与模型/单次预算不变；Schema3登记InputView，旧默认与Schema1/2原记录保持。格式化不授予权限或改变值；原文、输入、协议/值失败与用量保留，不补写成合格答案，不计为新独立案例。
- 交叉编码器重排仅在`RERANK_E2E=1`的独立Python入口使用已校验本地固定资产与runtime，不自动下载或加载远程代码；CPU单线程、每对512Token拒绝超限，模型仅接问题和候选id/text。默认Go重放原始评分；同池身份、评分类型、排序指标与答案/权限分别验证。
- 向量/重排联合入口须同时`EMBEDDING_E2E=1`与`RERANK_E2E=1`，固定调用已公开的本地JSON CLI，不import其他topic私有实现或接收模型路径；两独立锁定runtime/资产的revision、Space、截断与数量分别核对，同查询候选/评分绑定保留。精确小池排序、ANN/数据库、标签校准与任务支持分别验收。
- Context本地压缩仅在`CONTEXT_COMPRESSION_E2E=1`加载已校验Qwen资产和锁定生成runtime；模型仅接收历史与任务视图，Go有效事实快照不进入生成输入。原始候选、前缀、Token和截断保留，不修复后冒充成功；默认只运行离线投影、截断与解析/保留契约检查。
- 语言反思/后续Actor沿用`LOCAL_STRATEGY_E2E=1`与strategy-comparison锁定资产/runtime；Reflector只接公开任务、实际观察与原始失败，不接评分值。建议按未验证建议输入，不写事实或授予权限；原始反思、实际Actor输入、失败及全部用量保留，默认只验证投影和记录重放。
- 原生语言反馈只把准入的提示作为普通数据消息；任务指纹与数据/工具/评估/模型版本、来源尝试和有效范围由应用核对，不从建议扩展发现ID或权限。反思文本不完整不替写；模型返回动作JSON也按原文记录，不能冒称已完成语言诊断。提示进入输入、行为变化与独立收益分别验收，进程内选择不继承持久Memory/时间TTL或Go缓冲的完整保证。
- Self-Ask CLI可用`--model`显式选择已登记的Qwen3-0.6B或1.7B锁定资产，默认仍为0.6B；不接受任意锁文件或自动下载，沿用同一显式E2E门禁与运行预算。模型名称、revision和实际dtype分别记录，配置对照不能直接归因于参数规模；其他策略与语言反思入口仍使用默认资产，不继承此对照结果。
- 原生Self-Ask同样使用既有门禁/模型锁/runtime与预算；只读lookup提议和最终JSON分别适配，当前拒绝多调用，不从普通文本推导执行。Provider原文、转换结果、实际messages/tools分开记录；单次回归覆盖不证明问题分解、等预算收益或通用Provider支持，默认仅重放与验证控制契约。
- 原生Self-Ask显式v3只提交question，应用在已发现未读范围进行英文词项匹配并记录application_query_match；无匹配计查询尝试后失败，不猜ID、不扩大范围。v1/v2默认与原始记录保持，v3不复用v2固定流程对照；实际检索绑定、模型问题分解与策略收益分别验证。
- 原生对照的任务集只按登记名称选择；`--compare-workflow`仅在v2显式开启。固定遍历只沿已返回链接，程序调度与真实模型调用分别记录来源/计数；两者都受原有控制周期、读取和修复上限约束，不用程序步骤虚构模型usage。需观察来源属于评分侧，不进入生成输入；新任务首次使用与后续回归用途分别核对。
- MCP本地模型路由仅在`MCP_MODEL_E2E=1`及应用配置`MCP_MODEL_PYTHON`、`MCP_MODEL_SCRIPT`、`MCP_MODEL_ASSETS`时执行；复用固定本地Server，模型不能选择进程或地址。`MCP_MODEL_CONTRACT=v2`仅由应用绑定本地调用ID，不改工具/参数与原始生成；默认v1仍重放旧协议。校验资产及生成runtime，四轮/两工具、输入1024/输出192、总Run180秒；默认只测试门禁、解析和原始记录重放，不加载模型。
- Safety容器探针仅在`SAFETY_SANDBOX_E2E=1`显式运行；只绑定本机Unix Docker endpoint，将仅含课程自有静态Go探针和合成文件的根文件系统导入本机Engine，不使用基础镜像或独立构建后端。模型不控制Docker、命令、挂载或环境。仅挂载自建公开合成资料，非root、只读根/资料、无外部网络及资源上限；退出清理本次标记的容器/镜像，不prune共享缓存。默认CI不调用Docker，显式环境缺失失败；实际文件/网络/内存/PID/CPU行为与生产隔离分别验收。
- Go 测试放同包 `*_test.go`，跨章节夹具放 `tests/fixtures/`，验证入口放 `scripts/`；修改检查器必须跑对应反例。
- 默认 validate 禁止隐式下载；security 明确允许访问漏洞库，安装工具必须另行准备。本机 HTTP/RPC 测试仍可能需要监听回环端口。缓存、权限、来源或 revision 不满足时报告失败与限制，不改基线制造通过。扫描器缺失或执行异常必须失败，不允许静默跳过。
- 自动检查不证明讲解深度、地图体验、模型质量或读者学会。人工审查记录实际证据；代码、内容和读者状态分别维护，不把未执行项标成通过。

## 目录与代码风格

- 章节默认使用 `content/book/partN-<track>/chapterNN.md`，示例使用 `labs/partN-<track>/chapterNN/<topic>/`；track 为 golang、backend、agent，补充主题按章节地图组织。Part 3 为了让源码路径与教学单元一致，正文源码使用 `content/book/part3-agent/{building,runtime,governance}/chapterNN-<topic>.md`；其中 `chapterNN` 表示当前教学单元号22–37，`<topic>` 表示该单元主题，具体映射以 `content/book/chapters.json` 为准。合并退出的页面ID通过 `content/book/redirects.json` 跳转至对应原主题锚点，不进入课程导航；修改跳转时核对旧片段与目标可达性。
- 目录与 topic slug 用小写 kebab-case；Go 包名用简短小写单词，不含下划线。旧章节状态地图继续记录1–33的实现证据；站点单元编号由chapters.json的unitNumber维护1–37，引言不编号，Harness 作为 Part 3 的普通编号单元维护。课程规划为37个教学单元，其中Part 3为16个；公开导航按三大篇组织：构建篇、运行时、治理篇。九类 Agent 知识体系可作为内部蓝图和内容审查维度，不作为 `navigationPath` 真源。主题用语义ID追踪；稳定页面ID、公开分组、教学单元号和旧证据地图分别维护，不用文件名推断已验收状态。变更映射时同步方案、元数据与检查器。
- 所有 Go Lab 共用 `labs/go.mod`，不在仓库根创建 `go.mod` 或 `go.work`，也不为每个 topic 拆 module。Lab 不依赖其他 topic 的私有实现；仅供 Lab 共享的稳定私有代码放 `labs/internal/`。独立应用在设计批准后拥有自己的 module 和 `internal/`，不借用 Lab 的私有实现。
- Async Tasks与Serving复用 `labs/internal/tasks` 的同进程任务契约，两个Lab独立启动；核心状态测试随共享包运行，正文摘录链接实现真源，HTTP协议留在Serving Lab。共享包不是持久服务或新的课程平台。
- 课程站点采用根目录 Astro 工程：`content/book/` 是课程正文唯一真源，`content/book/chapters.json` 管理页面元数据，`.generated/chapters/` 由构建生成且禁止手改；不得再创建独立 `site/`、根目录 `book/`/`interview/` 或平行 `course/` 正文或持续演进的课程项目。综合主题仍按章节放进可独立启动的 Lab；不同 Lab 可以复用公开协议和测试方法，但不能依赖上一周工作区状态。`src/` 只负责呈现，不反向保存正文、课程状态或验收结果。
- Part 3页面的 `navigationPath` 在章节清单中维护三大公开分组；所有已发布 Part 3 正文（包括 Harness）都归入构建篇、运行时或治理篇。课程导航只显示已发布页面，当前页面与所属分组展开，链接沿用稳定页面ID；公开分组与线性阅读顺序一致，并分别维护，不从标题或文件章号推断。prerequisites只记录真实知识依赖，推荐顺序由order/前后导航表达；按正文与独立Lab需要核对传递依赖，不把上一页自动当硬前置，也不因精简依赖删除主题/练习/效果要求。改变分组时同步清单校验与生成页面验证。未发布规划项可缺正文，但发布项必须有真实源码；未发布项不生成路由、卡片或导航链接，禁止创建空正文补数量。
- 对修改的 Go 文件执行 gofmt，不用全仓格式化覆盖无关工作。修改正文摘录关联的源码后，先运行`python3 scripts/verify-agent-snippets.py`再执行完整验证；新增函数插入连续摘录区域时，同步正文或保留原区域连续性。构建产物、覆盖率、profile、runtime state 输出到临时或忽略目录，不提交。站点源码位于根目录 `src/`、`public/` 与配置文件；只有 `.generated/chapters/`、`.astro/`、`dist/` 和 `node_modules/` 是生成或本地产物。
- Part 3当前Python/SVG图示在`public/diagrams/sources.json`登记源与产物。`validate-book`在临时副本逐源重建，核对字节一致、注册集合、正文替代文字和受限SVG结构；图名/源码/产物变化同时更新登记与反例。检查不证明布局、图与实现的语义或任意SVG安全；未来其他源格式先扩展明确契约，不绕过登记。执行的是仓库受信任生成器，临时目录不是未知代码沙箱。
- 中文章节按主题知识点组织，用代码和图示解释机制，不写成Lab导读；实验输出、测试命令与操作引导放Lab README及代码注释。复杂主题可增加最佳实践和常见问题排查，主体不套统一模块标题。目标规范固定顶部“配套实验”、文末“小结”和“参考文档”，练习移至Lab；Harness、Model、Loop、Tool、Runtime、Session、Context、Retrieval、RAG、Eval、ReAct、Planning、Checkpoint、Trace、MCP、Skills、Workspace、Memory、Delegation、DAG、Async Tasks、Streaming、Serving、Safety、Cost与HITL正文已同步规则、模板和反例，采用新结构；Part 3整合为16章并覆盖原26主题，HITL归入构建篇的受控行动；其余正文暂时使用旧栏目，按总体方案逐章迁移并同步检查器。按规范标明源码、教学变体与故意失败例并验证。练习保留原任务与边界，不填满答案；合并章不丢各主题练习。维护记录与恢复状态不写入读者正文。
- Agent章节以最小决策闭环定位机制，解释其输入、状态、动作或控制权变化；按[总体方案](docs/designs/project-plan.md)与[章节规范](docs/rules/book-chapter-contract.md)区分机制正确、任务有效和复杂度收益。四个审查问题不固定为标题，Provider字段或具体调度/通信接口不作为通用架构定义。
- `content/interview/<大章节>/ae-ID-主题.md` 是面试正文唯一真源；同一篇文章不复制到 book。题库与状态在 `content/interview/`，生产工具与测试统一在根目录 scripts/tests。interview 保存总导航、questions.json、state.json与章节正文；十大章节预建，空目录用零字节 .gitkeep 保留，不放章节 README、局部规则或过程材料。研究、逐篇审核及 interview 的本地预览、图工具运行回执、平台标识与备份保存在仓库外私有工作目录，公开目录不依赖这些材料。
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

## 独立面试站点

面试网页以content/interview为正文真源，website.json显式管理可见题目，不改变编辑/公众号发布状态。独立配置astro.interview.config.mjs读取src/interview-site，缓存.astro-interview、中间副本.generated/interview、输出dist-interview；与课程src/pages、.generated/chapters、dist隔离。复用依赖和公共静态资源，不另建人工正文或Node工程。npm run check:interview验证面试站点，dev:interview/preview:interview分别使用4322/4323，沿用受监督前台进程与失败保留上次有效内容。make validate-site执行npm run check:sites，检查两个站点并组装.pages-dist；Pages只部署合并包，既有课程地址不变，面试位于/the-way-to-ai-engineer/interview/。公开页面不得包含私有研究、审核、回执或凭据；自动检查不等于远端发布验收。
