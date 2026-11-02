# 安全政策

## 报告漏洞

本项目选择 GitHub 私密漏洞报告作为安全入口。仓库公开且功能启用后，请进入仓库的 Security → Advisories → Report a vulnerability，私密提供复现步骤、受影响路径和版本、影响范围及建议修复。

不要在公开 Issue、PR、讨论或日志中贴真实密钥、用户数据或可直接滥用的漏洞细节。若没有看到私密报告按钮，不要改为公开提交细节；该情况表示入口尚不可用，应先等待维护者完成启用。

发布前必须验证此入口实际可用；文档中的说明不代表 GitHub 设置已完成。启用与确认方法见 [GitHub 私密报告说明](https://docs.github.com/en/code-security/security-advisories/working-with-repository-security-advisories/configuring-private-vulnerability-reporting-for-a-repository)。

## 适用范围

当前项目是学习教材与本地示例，不提供生产服务，也没有已承诺维护的历史发行分支。维护者在默认分支评估问题，不承诺未经确认的响应时限。教材错误、运行环境问题和内容建议请使用普通 Issue；社区行为投诉不用漏洞渠道。

默认测试不调用真实模型、地图、天气或生产数据库。使用者接入外部服务时应自行限制权限、预算和数据范围。内部实现约束见[安全规范](docs/rules/security.md)。

## 泄漏处置

若发现凭据泄漏，先撤销或轮换凭据，再排查提交、日志和生成物中的副本。删除文件不等于凭据失效。不要在报告中附上仍然有效的秘密；历史清理和对外披露需由维护者协调。
