# 每日一题审核说明

每题归档审核位于私有工作目录 `reviews/AE-NNN.json`；公众号本期审核先写 `wechat/AE-NNN/working/review.json`，除article_sha256外必须绑定media_sha256，随审核版本冻结，归档后回填。旧scaffold保留兼容。不得将此模板内的说明当成审核结果。

- reviewer：实际审阅者标识，明确是作者、真人同行或 Codex。
- reviewed_at：实际审核日期 YYYY-MM-DD。
- article_sha256：运行 hash 子命令得到，修改文章后需重新审核。
- evidence_level：实际达到的 design / runnable / measured，不得低于题库 minimum_evidence；不能把替身测试当真实模型效果。
- evidence_notes：实际输入、环境、命令、结果与限制；未完成不得写通过。
- notes：逐项写来源定位、验证命令及结果、未验证边界、许可判断。证据不足不能写通过。
- checks.facts：关键主张与权威来源一致、版本与前提明确。
- checks.engineering：机制和失败路径有证据，示意与已运行代码区分。
- checks.clarity：核心问题、机制、取舍、反例与回案例解释清楚；新稿公众号与网站同稿，历史补充只按其既有版本另审。
- checks.sources：引用可定位、许可和归属正确。
- checks.safety：权限、敏感数据、预算和副作用边界正确。

全部 checks 为 true 才能进入 ready。程序只验证字段和摘要，不判断技术事实是否正确，也不会自动给出通过结论。
