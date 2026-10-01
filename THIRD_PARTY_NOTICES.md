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

逐个 Lab 的具体 URL 保存在 meta.yml 的 references，正文参考资料保留对应阅读入口。引用 URL 不是完整的权属审计。

## 生成代码与普通依赖

Kitex 实验包含 kitex_gen 生成目录，分别位于 chapter16 的两个工程与 chapter17 的两个 RPC 实验内；IDL 位于对应 Lab。保留生成器标记与现有文件，不宣称生成器模板为本项目原创。发布前核对生成器版本、模板适用许可及必要声明。

`labs/go.mod` 与 `labs/go.sum` 列出 Go Lab 的直接与间接依赖，本仓库没有 vendor 目录。依赖仍使用其各自许可证，不因根目录 LICENSE 改变。后续分发二进制、容器或 vendor 内容时，应针对实际分发集合补充所需声明，不把当前源码清单当成二进制分发审计。

## 本轮核对状态

已确认自有内容的维护者授权和公开署名；已列出参考来源，区分教材文字、代码和生成物。当前没有随教材纳入的图片或 PDF 文件。

仍需完成的人工核对：Go by Example 代码是否存在需标注的改编片段；Go 入门指南译文是否存在复制/改编；Uber 中文译本与示例的许可边界；Kitex 生成模板的声明义务。此处不把“有参考链接”直接判为复制，也不把“没有单独 LICENSE 文件”判为侵权。

上述项目未完成前，[发布清单](docs/release-readiness.json)中的 third-party-review 保持 pending。后续确认应记录具体路径、比对版本及结论，而不是仅将状态改成 confirmed。
