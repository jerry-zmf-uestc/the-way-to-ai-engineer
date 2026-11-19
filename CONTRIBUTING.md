# 参与贡献

欢迎纠正文档、改进示例、补充测试或提出课程建议。参与前请阅读[社区行为准则](CODE_OF_CONDUCT.md)：交流时围绕问题和证据，不作人身攻击，不公开他人的私人信息。社区行为投诉使用准则中的维护者邮箱；安全漏洞按 [SECURITY.md](SECURITY.md) 私密报告，两类渠道分别处理。

## 先确定改动范围

- 错别字、失效链接和小范围修复可以直接提交 PR，说明改了什么及如何验证。
- 新章节、目录调整、公共接口、依赖、安全或 CI 变化，先开 Issue 说明问题、方案、任务和验收条件，维护者确认后实施。
- 公开贡献不需要维护者的本地规格或其他私有仓库。需求、设计、任务、验收可写在同一个 Issue/PR；长期约定归入现有 docs 或 AGENTS。

不要只提交生成的长文而不核对示例。使用辅助工具不免除事实、测试和来源责任；不得提交私人聊天记录、真实密钥或授权不明的第三方内容。

## 写作与代码要求

先读 [AGENTS.md](AGENTS.md) 中按任务列出的规则。正文使用[章节模板](docs/templates/book-chapter.md)与[章节规范](docs/rules/book-chapter-contract.md)；Lab 遵守[示例契约](docs/rules/topic-contract.md)。不在本文件复制另一套规范。

保留原有知识点、边界和练习。练习以任务注释、关键代码和 TODO 呈现，不用大段操作说明代替代码，也不直接填满答案。实验改动在副本进行，Go 测试与被测代码同包，文档检查器测试放 tests，不放 docs。

<a id="validation"></a>

## 运行检查

以下命令从仓库根目录执行。普通 Lab 需要 Go；完整维护检查还需要 Node 22、npm、Ruby、make、POSIX shell、ripgrep 和 C 工具链。CI 配置 Linux 完整检查与 macOS 兼容检查；Windows 用户可使用 WSL2，当前不承诺原生 Windows shell 脚本兼容。远端实际通过范围以 CI 结果为准。

```bash
go version
node --version
ruby --version
make --version
rg --version
go -C labs mod download
npm ci
make validate
```

Go 版本以 `labs/go.mod` 为准，Node 最低版本以 `package.json` 为准。现有脚本在本机 Ruby 2.6 上运行；CI 使用 Ruby 3.3。`go -C labs mod download` 与 `npm ci` 是显式联网准备，`make validate` 不隐式升级或下载依赖。HTTP/RPC 测试仍需要回环端口。

如果系统 Go 比 `labs/go.mod` 指定的安全补丁旧，可以安装对应版本，或显式准备项目工具链并将其加入当前 shell 的 PATH：

```bash
GOTOOLCHAIN=go1.26.6 go version
project_go_root=$(GOTOOLCHAIN=go1.26.6 go env GOROOT)
export PATH="$project_go_root/bin:$PATH"
go version
```

这只改变当前 shell，不替换系统 Go。Makefile 使用 GOTOOLCHAIN=local，因此仅下载工具链但不更新 PATH，仍会被旧版本拒绝。

| 改动 | 检查 |
| --- | --- |
| 文档和规则 | make validate-docs |
| 正文代码、命令或检查器 | make validate-docs、make validate-book、受影响示例；检查器增加对应反例 |
| chapters 清单、内容生成器、Astro 页面或样式 | make validate-site；正文变化同时执行对应文档检查 |
| Go、依赖、CI 或发布流程 | make validate；安全相关还执行 make security |

安全工具由 [security-tools.json](scripts/security-tools.json) 固定版本，不隐式安装：

```bash
make tools-install  # 明确联网，将固定版本工具安装到本仓库忽略的 .tools/bin。
export PATH="$PWD/.tools/bin:$PATH"
make security      # 凭据扫描；明确联网查询 Go 与 npm 漏洞库。
make security-history  # 已有提交时扫描 Git 历史；无提交会报告不能执行。
```

扫描失败或缺工具不能当成通过；命中后先判断影响并修复，不用全目录忽略掩盖问题。需要暂时接受误报时，在 PR 记录位置、原因、负责人和复查条件，缩小例外范围。

## 发布快照

本机存在的文件不一定会进入提交。发布检查必须在独立副本中执行：

```bash
# 当前工作目录的可提交文件，只作为候选演练。
ruby scripts/validate-release.rb --mode working --checks validate

# 精确验证 Git index 中已选择的内容，不包含未暂存修改。
make validate-release RELEASE_MODE=index

# 正式发布使用已确认的提交；将实际提交 SHA 传给 --ref。
ruby scripts/validate-release.rb --mode revision --ref HEAD --checks release
```

脚本不替你暂存、提交或推送。working 不是发布提交；index 使用已暂存字节；revision 会输出解析后的完整 SHA。--checks docs 仅检查文档，--checks validate 执行完整离线验证，--checks release 还执行安全与就绪检查。准备好依赖和安全工具后再运行，临时副本不会复制私有规格、工具缓存或 Git 内部状态。

发布前还须确认[发布清单](docs/release-readiness.json)：第三方材料已核对、私密漏洞入口实际可用、远端 CI 与保护规则已确认、试读或明确的预览发布限制已评估。清单必须填写真实证据，脚本不能证明授权或真人验收。

## 提交 PR

说明动机、涉及章节、实际运行的命令与结果，以及尚未验证的部分。第三方代码和文字注明来源版本、许可和改动，更新[第三方说明](THIRD_PARTY_NOTICES.md)。不要把本地代码测试通过写成全部读者已经学会。

贡献者应确保有权提交内容。原创代码按 MIT、原创教材按 CC BY 4.0 提交；第三方内容保持其许可，不能因提交 PR 而自动改标为本项目许可。没有额外 CLA 或 DCO 签署要求。
