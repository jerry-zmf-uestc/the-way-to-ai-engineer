# 检查器与夹具

Go 测试与被测代码同包，位于 labs。本目录保存正文、目录和发布检查器的回归测试，测试只修改临时副本；不保存学习记录、私人历史或扫描报告。

`fixtures/book-regions.json` 描述当前教学区域，值可以是文件或文件加主题锚点。第 2、7 章按主题检查练习数量、连续题号和定位链接，不把整章重复计数。调整章节组织时，夹具与正文一起审查，不能为了通过测试随意删除检查项。

| 测试 | 检查边界 |
| --- | --- |
| docs-layout-test.rb | 规则、模板、引言与 AGENTS 入口 |
| book-template-test.rb | 正文结构和必要教学内容 |
| chapter-merge-test.rb | 单文件章节、主题锚点及练习连续性 |
| privacy-check-test.rb | 扫描器缺失、执行异常、命中脱敏 |
| public-links-test.rb | 私有依赖、失效公开链接 |
| release-snapshot-test.rb | working、index、revision 字节来源，私有文件和缺失文件 |
| security-command-test.rb | 安全工具缺失或版本不符 |
| release-readiness-test.rb | 发布清单结构及未完成项阻塞 |
| ci-policy-test.rb | Action 提交固定、最小权限与 PR 事件边界 |
| chapter-evidence-test.rb | 章节证据类型、锚点与人工验证边界 |
| topic-metadata-test.rb | 原创许可与第三方来源必需字段 |

执行单个测试：`ruby tests/对应文件.rb`。全部入口由 [Makefile](../Makefile) 的 validate-book 维护。快照测试创建的是临时 Git 仓库；不会暂存或提交真实工作区。

这些测试确认检查器能够拒绝故障，不证明全部教材深度、第三方授权或真实读者学习效果。
