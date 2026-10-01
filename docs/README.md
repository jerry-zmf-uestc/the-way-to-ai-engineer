# Documentation

本目录保存长期有效的项目知识；公开变更说明和验证结果写入 Issue/PR，维护者可另留私有规格。

## 信息架构

| 分类 | 内容 | 入口 |
| --- | --- | --- |
| Designs | 项目愿景、课程、路线、章节状态、仓库与 Agent 架构 | [项目总体方案](designs/project-plan.md) |
| Rules | 章节、Lab、安全三份专项规范 | [强制读取规则](../AGENTS.md) |
| 学习准备 | 工作目录、运行和练习副本；正文归入 book | [如何使用本书](../book/introduction.md#running-examples) |
| Templates | 章节与内容审查模板 | [templates/book-chapter.md](templates/book-chapter.md) |
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
