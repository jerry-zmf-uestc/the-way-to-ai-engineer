# Documentation

本目录保存长期有效的项目知识；公开变更说明和验证结果写入 Issue/PR，维护者可另留私有规格。

## 信息架构

| 分类 | 内容 | 入口 |
| --- | --- | --- |
| Designs | 项目愿景、课程、路线、章节状态、仓库与 Agent 架构 | [项目总体方案](designs/project-plan.md) |
| Rules | 章节、Lab、安全、面试内容与图示规范 | [强制读取规则](../AGENTS.md) |
| 学习准备 | 工作目录、运行和练习副本；正文归入 book | [如何使用本书](../content/book/introduction.md#running-examples) |
| Templates | 章节与内容审查模板 | [templates/book-chapter.md](templates/book-chapter.md) |
| 面试合集 | 框架中立的 50 个主问题、分章正文与统一内容生产 | [interview](../content/interview/README.md) / [内容规范](rules/interview-content-contract.md) |
| Sources | 第三方归属与许可边界 | [第三方说明](../THIRD_PARTY_NOTICES.md) |

## 真源规则

全仓目录、命名、变更纪律和规格工作流统一维护在 [AGENTS.md](../AGENTS.md)。专项文档只保留按任务读取的详细要求：

- [章节写作规范](rules/book-chapter-contract.md)：内容、示例、练习与阅读导航。
- [Lab 契约](rules/topic-contract.md)：产物、README、元数据与来源。
- [测试与验收要求](../AGENTS.md#testing-requirements)：通用测试规则；Agent 案例和正文验证边界分别归入安全与章节规范。
- [安全规范](rules/security.md)：秘密、外部输入、工具权限、变量规划与真实服务接入清单。

写章前使用 [章节模板](templates/book-chapter.md)，审查与验收使用 [章节审查表](templates/book-chapter-review.md)。两份模板都必须结合章节写作规范执行。

- 项目愿景、章节编号、四维状态、六个月路线、独立 Lab、站点、架构和完成定义：[项目总体方案](designs/project-plan.md)。
- 章节写作规则：[book-chapter-contract](rules/book-chapter-contract.md)。
- 作者审校范围与状态：[总体方案](designs/project-plan.md#author-review)。测试代码与夹具存放在 tests，不在本目录。
- 第三方材料：[来源与许可](../THIRD_PARTY_NOTICES.md)。
- 复杂变更意图与验收：公开 Issue/PR；本地规格不作为贡献前提。
- Agent 操作约束：根目录 `AGENTS.md`。
- 发布前人工与远端检查清单：`release-readiness.json`。

- [面试专项方案](designs/interview-handbook.md)：50 个主问题边界、课程关联、编辑数据与迁移验收。
- 公众号生产：[合集与草稿规则](rules/interview-wechat.md)、[渠道稿模板](templates/interview-wechat-article.md)、[本期命令与格式](rules/interview-content-contract.md#本期制作命令与资料格式)。同案例技术递进、设计者视角、无底色青绿记忆点及编号短图注统一维护在规则和模板，不在各篇另建风格副本。issue.mjs按列表选题，私有写作→审核/保存回读→归档→经另行授权的网站同稿准入；仅本地检查，不自动操作公众号、git或部署。wechat-check核对版本及缺口，候选预期snapshot不能当实际保存证据。
- [图示规范](rules/diagram-contract.md)：课程与Archify各自的登记、验证及面试配图风格。
- 面试编辑模板：[文章](templates/interview-article.md)、[研究](templates/interview-research.md)、[审核](templates/interview-review.md)。逐篇研究与审核在仓库外，模板不是审核结果。

- 面试独立站点、website.json可见门槛及共享Pages部署：[面试专项方案](designs/interview-handbook.md#独立面试站点与共享pages托管)。两个站点独立构建，发布包统一组装，不修改课程地址或公众号状态。
