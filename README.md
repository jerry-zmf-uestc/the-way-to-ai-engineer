# the-way-to-ai-engineer

用 Go 学习语言基础、高性能后端与 Agent 应用开发。教材和示例放在同一个仓库，读完一节就能打开代码、运行测试，再动手修改。

主要写给已有 TypeScript/React 经验、希望补齐 Go 后端与 Agent 工程能力的前端工程师，也适合其他已有软件工程经验、准备系统进入 Go AI 全栈方向的读者。起步只需要会使用终端和编辑器；后端、Agent 部分以前面的 Go 知识为基础。

## 从这里开始

先安装 [Go](https://go.dev/doc/install)，版本要求见 [labs/go.mod](labs/go.mod)。在本仓库根目录运行：

```bash
go version
go -C labs run ./part1-golang/chapter01/hello-world
go -C labs test ./part1-golang/chapter01/hello-world -count=1
```

程序输出 `hello, go`，测试应通过。接着阅读[第 1 章](content/book/part1-golang/chapter01.md)，学习如何构建程序、修改欢迎语和验证不同输入。

第一次运行后端或 Agent 示例前，执行 `go -C labs mod download` 准备锁定依赖。单个 Lab 不要求安装文档检查工具；完整环境和练习副本说明见[如何使用本书](content/book/introduction.md#running-examples)。

## 学习路线

| 部分 | 章节范围 | 学习内容 |
| --- | --- | --- |
| Part 1 · Go 基础 | 1–11 | 语法、数据结构、接口、并发、HTTP 与测试 |
| Part 2 · 高性能后端 | 12–21 | 数据库、Redis、微服务、分布式、高并发与可观测性 |
| Part 3 · Go Agent | 22–33 | Agent Harness、RAG、多 Agent 协作、Streaming/Serving 与运行治理 |

[打开全书目录](content/book/README.md) · [查看项目总体方案](docs/designs/project-plan.md)

## 当前范围

项目仍在建设。当前仓库已有 72 个可验证 topic、49 个 manifest 页面和 44 个已发布教学页；这些数量不等于课程已经完成最终验收。

- Go 基础已有连续阅读路线；Part 3 已形成 三大篇、16 个 Agent 教学单元和对应实验入口。
- 现有数据库、模型和向量检索示例以本地机制演示、fake、内存实现或受控本地模型对照为主，不代表生产数据库、真实外部权限或完整线上服务已经接通。运行条件以各 Lab README 为准。
- Part 2 仍缺 Redis、MQ、服务发现/配置、可靠性与压测等单元的正式正文与实验；Part 3 仍需继续补齐模型效果、语义评分、真人试学与作品级验收。
- 代码检查、作者审校与真实读者验证分别记录，状态以[总体方案中的章节地图](docs/designs/project-plan.md#chapter-map)为准；没有读者反馈的页面不标记为试学通过。
- GoTrip 仍是独立实践题材，不承担课程主线或毕业设计角色；课程站点已有构建链路，发布前仍须完成第三方核对、远端入口与真人试学验证。

GoTrip 是后续独立旅行实践，不要求每章增加旅行功能，也不设置毕业设计。项目不提供预订、支付或消息发送能力，不包含学习推送、学习记录管理和定时复盘。

## 仓库导航

| 入口 | 用途 |
| --- | --- |
| [interview](content/interview/README.md) | 按十大章节组织的 50 个主问题面试手册（独立于课程章号） |
| [book](content/book/README.md) | 教材与阅读目录 |
| labs | 各章独立示例，从正文进入 |
| [docs](docs/README.md) | 长期设计、规则与模板 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 当前实现与目标架构边界 |
| [tests](tests/README.md) | 文档与检查器回归说明 |

面试合集可独立运行 `make validate-interview`，只需 Node；课程与合集共享已有 Lab，不强制面试读者先学习 Go。课程与面试使用独立页面、导航和构建；共享Pages部署，面试入口为 `/the-way-to-ai-engineer/interview/`。

## 验证与贡献

维护者需要 Go、Node 22、npm、Ruby、make、POSIX shell、ripgrep，以及支持 race 的 C 工具链。首次准备依赖后执行：

```bash
go -C labs mod download
npm ci
make validate
```

默认验证在依赖就绪后离线运行，但 HTTP/RPC 测试需要本机回环端口。安全工具准备、快照检查和发布验收见[贡献指南](CONTRIBUTING.md)。

欢迎纠正文字、补充边界案例、改进示例或提出新主题。小改动可直接提交 PR；目录、接口和课程范围调整先讨论。写作继续使用现有章节模板，练习与细粒度操作说明以配套 Lab README 和任务表为准。

## 许可证与安全

原创代码和原创教学代码块使用 [MIT](LICENSE)；原创教材文字与图示使用 [CC BY 4.0](LICENSE-DOCS)。第三方材料保留自己的许可，具体范围及待核对项见[第三方说明](THIRD_PARTY_NOTICES.md)。版权署名：2026 jerry-zmf-uestc。

漏洞请按[安全政策](SECURITY.md)私密报告，不要在公开 Issue 中贴密钥、真实用户数据或可直接滥用的细节。开源发布前仍须完成第三方核对和远端安全入口验证，不能用本机测试通过代替这些检查。

面试站点独立运行：`npm run dev:interview`（4322），生产预览用 `npm run build:interview`、`npm run preview:interview`（4323）。访问路径为 `/the-way-to-ai-engineer/interview/`。`npm run check:sites` 检查课程与面试两个站点并生成共享Pages部署包；课程原地址保持。网站只展示website.json显式列出的正文，其他题仅作规划目录，不等同于公众号公开发布。
