# the-way-to-ai-engineer

用 Go 学习语言基础、高性能后端与 Agent 应用开发。教材和示例放在同一个仓库，读完一节就能打开代码、运行测试，再动手修改。

适合希望系统学习 Go，并继续实践后端和 Agent 开发的读者。起步只需要会使用终端和编辑器；后端、Agent 部分以前面的 Go 知识为基础。

## 从这里开始

先安装 [Go](https://go.dev/doc/install)，版本要求见 [labs/go.mod](labs/go.mod)。在本仓库根目录运行：

```bash
go version
go -C labs run ./part1-golang/chapter01/hello-world
go -C labs test ./part1-golang/chapter01/hello-world -count=1
```

程序输出 `hello, go`，测试应通过。接着阅读[第 1 章](book/part1-golang/chapter01.md)，学习如何构建程序、修改欢迎语和验证不同输入。

第一次运行后端或 Agent 示例前，执行 `go -C labs mod download` 准备锁定依赖。单个 Lab 不要求安装文档检查工具；完整环境和练习副本说明见[如何使用本书](book/introduction.md#running-examples)。

## 学习路线

| 部分 | 章节范围 | 学习内容 |
| --- | --- | --- |
| Part 1 · Go 基础 | 1–11 | 语法、数据结构、接口、并发、HTTP 与测试 |
| Part 2 · 高性能后端 | 12–21 | 数据库、Redis、微服务、分布式、高并发与可观测性 |
| Part 3 · Go Agent | 22–33 | 模型接入、工具调用、Eino、RAG、Memory、MCP 与评估 |

[打开全书目录](book/README.md) · [查看项目总体方案](docs/designs/project-plan.md)

## 当前范围

项目仍在建设。当前有 45 个 Lab、21 篇教学正文和 44 道练习；这些数量不代表规划的 33 章已经全部完成。

- Go 基础已有连续阅读路线；部分后端与 Agent 主题已有独立示例。
- 现有数据库、模型和向量检索示例以本地机制演示、fake 或内存实现为主，不代表生产数据库或真实模型已接通。运行条件以各 Lab README 为准。
- Redis、MQ、完整 Agent Runtime、GoTrip 和文档站点尚未完成。
- 代码检查、作者审校与真实读者验证分别记录，状态以[总体方案中的章节地图](docs/designs/project-plan.md#chapter-map)为准；没有读者反馈的章节不标记为试学通过。

GoTrip 是后续独立旅行实践，不要求每章增加旅行功能，也不设置毕业设计。项目不提供预订、支付或消息发送能力，不包含学习推送、学习记录管理和定时复盘。

## 仓库导航

| 入口 | 用途 |
| --- | --- |
| [book](book/README.md) | 教材与阅读目录 |
| labs | 各章独立示例，从正文进入 |
| [docs](docs/README.md) | 长期设计、规则与模板 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 当前实现与目标架构边界 |
| [tests](tests/README.md) | 文档与检查器回归说明 |

## 验证与贡献

维护者需要 Go、Node 22、npm、Ruby、make、POSIX shell、ripgrep，以及支持 race 的 C 工具链。首次准备依赖后执行：

```bash
go -C labs mod download
npm ci
make validate
```

默认验证在依赖就绪后离线运行，但 HTTP/RPC 测试需要本机回环端口。安全工具准备、快照检查和发布验收见[贡献指南](CONTRIBUTING.md)。

欢迎纠正文字、补充边界案例、改进示例或提出新主题。小改动可直接提交 PR；目录、接口和课程范围调整先讨论。写作继续使用现有章节模板，练习用带任务注释的代码块表达。

## 许可证与安全

原创代码和原创教学代码块使用 [MIT](LICENSE)；原创教材文字与图示使用 [CC BY 4.0](LICENSE-DOCS)。第三方材料保留自己的许可，具体范围及待核对项见[第三方说明](THIRD_PARTY_NOTICES.md)。版权署名：2026 jerry-zmf-uestc。

漏洞请按[安全政策](SECURITY.md)私密报告，不要在公开 Issue 中贴密钥、真实用户数据或可直接滥用的细节。开源发布前仍须完成第三方核对和远端安全入口验证，不能用本机测试通过代替这些检查。
