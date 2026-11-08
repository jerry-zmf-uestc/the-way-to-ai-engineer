# 第三方来源与许可

本项目的 MIT 与 CC BY 4.0 仅覆盖维护者有权授权的原创内容，不覆盖第三方作品。普通参考链接不等于复制；复制或改编的材料必须另行注明适用路径、版本、作者、许可及改动。

## 参考材料

| 来源 | 本项目中的位置 | 归属与处理 |
| --- | --- | --- |
| [Go by Example](https://gobyexample.com/) | Part 1 多个 Lab 的 references | Mark McGranaghan、Eli Bendersky；上游 README 声明 [CC BY 3.0](https://github.com/mmcgrana/gobyexample#license)。参考链接保留；若逐段核对发现改编内容，应按上游许可标注，不改标 MIT |
| [Go 官方文档](https://go.dev/doc/)与[标准库](https://pkg.go.dev/std) | 正文与 Lab 的 API、语言机制说明 | Go Authors；网页引用与代码复制分开处理，代码复制时保留对应版本声明 |
| [Go 入门指南](https://learnku.com/docs/the-way-to-go) | 引言、Part 1 和部分后端章节的参考资料 | 保留原书与译者归属；当前参考链接不授予复制整段译文的权利，文字改编边界须在发布前核对 |
| [Uber Go Style Guide](https://github.com/uber-go/guide)与[中文译本](https://github.com/xxjwxc/uber_go_guide_cn) | chapter11 的错误包装、预分配、字符串构建示例 | 英文上游采用 [Apache-2.0](https://github.com/uber-go/guide/blob/master/LICENSE)；译本署名与许可单独核对，不能直接套用英文项目声明 |
| [Project Layout](https://github.com/golang-standards/project-layout) | chapter11/project-layout | 社区布局讨论，不是 Go 官方强制标准；与官方 module 布局文档对照阅读 |
| [CloudWeGo](https://www.cloudwego.io/docs/)与 [Eino](https://github.com/cloudwego/eino) | Part 2 HTTP/RPC、Part 3 Eino/RAG | 文档为参考；框架依赖按 `labs/go.mod` 固定版本，生成代码另见下节 |
| [GORM](https://gorm.io/docs/)和 [Apache Thrift IDL](https://thrift.apache.org/docs/idl) | Part 2 数据访问及 IDL 示例 | 保留参考链接；文档示例复制与依赖使用分别核对 |
| [Exponential Backoff and Jitter](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/) | HTTP 客户端实验参考资料 | AWS 作者的退避与抖动说明；仅作为参考入口，不授权转载原文或图片 |
| [Hello-Agents](https://datawhalechina.github.io/hello-agents/) | 课程设计与独立旅行助手方向 | Datawhale；作为路线和场景参考，未引入其完整课程或旅行应用源码。将来复制/改编时先核对对应版本许可 |
| [AI Agent Handbook](https://aliyun.github.io/ai-agent-handbook/contents.html)与[治理篇](https://aliyun.github.io/ai-agent-handbook/governance/) | Part 3 三大篇职责组织、构建篇人工介入、治理篇导读及资产/行为验证分工 | 阿里云与社区贡献者；2026-10-29查阅目录和构建篇第6章，2026-11-08对照治理导读及第13–16章。上游站点声明Apache-2.0；本项目仅参考职责组织与概念分工，采用原创只读资料案例，未复制其正文段落、代码或图片，不将普通参考混同为上游文本的重新许可 |

逐个 Lab 的具体 URL 保存在 meta.yml 的 references，正文参考资料保留对应阅读入口。引用 URL 不是完整的权属审计。

## 生成代码与普通依赖

Kitex 实验包含 kitex_gen 生成目录，分别位于 chapter16 的两个工程与 chapter17 的两个 RPC 实验内；IDL 位于对应 Lab。保留生成器标记与现有文件，不宣称生成器模板为本项目原创。发布前核对生成器版本、模板适用许可及必要声明。

`labs/go.mod` 与 `labs/go.sum` 列出 Go Lab 的直接与间接依赖，本仓库没有 vendor 目录。依赖仍使用其各自许可证，不因根目录 LICENSE 改变。后续分发二进制、容器或 vendor 内容时，应针对实际分发集合补充所需声明，不把当前源码清单当成二进制分发审计。

## 本轮核对状态

已确认自有内容的维护者授权和公开署名；已列出参考来源，区分教材文字、代码和生成物。原创图示的生成器与产物在public/diagrams/sources.json登记；外部图片或PDF如被纳入教材，须另列来源与适用许可。

仍需完成的人工核对：Go by Example 代码是否存在需标注的改编片段；Go 入门指南译文是否存在复制/改编；Uber 中文译本与示例的许可边界；Kitex 生成模板的声明义务。此处不把“有参考链接”直接判为复制，也不把“没有单独 LICENSE 文件”判为侵权。

上述项目未完成前，[发布清单](docs/release-readiness.json)中的 third-party-review 保持 pending。后续确认应记录具体路径、比对版本及结论，而不是仅将状态改成 confirmed。

Memory的PostgreSQL驱动采用 [pgx](https://github.com/jackc/pgx)（MIT），版本以labs/go.mod为准；本仓库仅引用依赖，不复制驱动源码。隔离测试所用PostgreSQL运行时遵守 [PostgreSQL许可](https://www.postgresql.org/about/licence/)，源码和临时数据不进入仓库。

Retrieval的显式向量数据库实验使用 [pgvector v0.8.7](https://github.com/pgvector/pgvector/tree/v0.8.7)，固定提交f37c13f68b57d2c3472b2214fbcff699d6d34876，采用[PostgreSQL License](https://github.com/pgvector/pgvector/blob/v0.8.7/LICENSE)。本次仅将官方源码编译到临时PostgreSQL运行时，不将源码、扩展二进制或测试数据纳入本仓库；后续二进制分发须按实际集合保留其版权与许可声明。

本地Embedding伴随实验使用 [sentence-transformers/all-MiniLM-L6-v2](https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2) 固定revision1110a243fdf4706b3f48f1d95db1a4f5529b4d41（模型卡Apache-2.0）。本机资产校验清单与Python固定版本见neural-retrieval Lab；权重和虚拟环境不纳入仓库。ONNX Runtime（MIT）、tokenizers（Apache-2.0）、NumPy（BSD-3-Clause）及传递依赖保留各自许可；此处不是模型权重/二进制分发许可审计。Go/Python封装、语料与问题为原创，未复制上游实现。

本地假想文档查询与RAG草稿生成对照使用[HuggingFaceTB/SmolLM2-135M-Instruct](https://huggingface.co/HuggingFaceTB/SmolLM2-135M-Instruct)，固定revision12fd25f77366fa6b3b4b768ec3050bf629380bac，模型卡Apache-2.0。资产清单与Python环境锁定文件在neural-retrieval及evidence-answer Lab；Transformers与safetensors按Apache-2.0，PyTorch及传递依赖保留各自上游许可。权重和运行时不入仓库；原创调用封装未复制其实现，不将此记录视为二进制再分发许可审计。

Eino工作流Python对照使用[LangGraph 1.0.3](https://pypi.org/project/langgraph/1.0.3/)（MIT），完整实际环境版本见eino-workflow/framework-requirements.lock；LangChain及其他传递依赖保留各自许可。固定任务、Go/Python封装为原创，没有复制上游示例实现；不分发虚拟环境或依赖二进制，不启用远端Tracing。

RAG本地配置对照另用[Qwen/Qwen3-0.6B](https://huggingface.co/Qwen/Qwen3-0.6B)，固定revision c1899de289a04d12100db370d81485cdf75e47ca（模型卡Apache-2.0），资产元数据见evidence-answer/qwen-generator-lock.json；动作/初始计划对照复用同一官方资产身份，清单及固定runtime元数据见strategy-comparison/model-lock.json与generation-requirements.lock。权重仅在本机临时缓存，复用固定生成runtime，不分发权重或依赖二进制；原始输入与封装为原创合成教学材料。

Skills全量/按需输入对照复用上述固定Qwen3资产与独立生成运行时，清单见skills-loader/model-lock.json及generation-requirements.lock。Go输入投影、Python调用与检查、任务及教学包均为原创；不分发模型权重或运行时，不将数字/字段覆盖检查当作通用语义Judge。

Self-Ask可选模型配置对照使用[Qwen/Qwen3-1.7B](https://huggingface.co/Qwen/Qwen3-1.7B)，固定revision70d244cc86ccca08cf5af4e1e306ecf908b1ad5e，官方LICENSE为Apache-2.0。两份safetensors权重、索引、分词资产及许可证的校验身份见strategy-comparison/model-lock-1.7b.json；沿用既有固定生成运行时。资产仅在本机独立临时缓存，不分发权重或依赖二进制。生成封装、任务与记录为原创；模型配置对照不等于参数规模的独立因果实验。

Context摘要对照复用同一固定Qwen3资产与生成运行时，清单见context-builder/model-lock.json及generation-requirements.lock。原始历史、Python生成封装与Go保留检查为原创；仅记录公开合成数据，不分发权重或依赖二进制。

MCP双Server模型路由对照复用同一Qwen3资产与生成运行时，清单见mcp-stdio/model-lock.json及generation-requirements.lock。Python生成、Go进程边界及公开教学请求为原创；不分发模型权重或依赖二进制。

Trace SDK对照使用[OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go/tree/v1.37.0) API/SDK/trace v1.37.0及其传递依赖（Apache-2.0与各自上游许可），版本和校验由labs/go.mod/go.sum维护。局部Provider、内存Exporter及carrier适配代码为原创调用封装，未复制SDK实现；不部署或分发Collector二进制。

RAG联合草稿配置对照复用上述Qwen3-1.7B固定官方资产，独立校验清单见evidence-answer/qwen-1.7b-generator-lock.json；沿用generation-requirements.lock，不依赖strategy-comparison私有实现或分发资产。提示、Go准入与合成条件案例为原创；同预算配置对照不证明参数规模的独立因果效应。

Memory缓存显式对照使用[Redis 7.2.11](https://github.com/redis/redis/releases/tag/7.2.11)官方源码，SHA256与来源见user-memory/redis-runtime-lock.json，并与[官方哈希清单](https://github.com/redis/redis-hashes/blob/master/README)核对。Redis核心采用BSD-3-Clause，Lua/hiredis等内含依赖保留各自许可；仅在本机临时目录编译，不分发源码或二进制。课程Go进程封装及合成资料为原创，不将本记录当成二进制分发许可审计。

Retrieval重排对照使用[cross-encoder/ms-marco-MiniLM-L6-v2](https://huggingface.co/cross-encoder/ms-marco-MiniLM-L6-v2)固定revision233902d25c440f23af6f7d6e94d2946bac0bee0a，模型卡Apache-2.0；官方Git/LFS身份和各资产SHA256核对后，仅保存在本机临时目录。清单见hybrid-retrieval/reranker-lock.json，运行时复用已锁定torch/Transformers依赖，封装及3题合成资料原创。未复制上游示例、分发权重或依赖二进制；此记录不替代再分发许可审计。
