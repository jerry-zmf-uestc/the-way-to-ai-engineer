# Topic Contract

## Required artifacts

可运行的正式 topic 默认包含：

```text
README.md
main.go 或 cmd/
main_test.go 或对应包测试
meta.yml
```

纯阅读选修必须在 `meta.yml` 标记 `kind: reading`，不得显示为可运行或已验证。

## README contract

- 解决的问题和非目标。
- 前置章节。
- 关键机制与观察点。
- 运行、测试和预期输出。
- 至少一个失败或变体练习。
- 可选场景或项目链接，不要求逐章增加 GoTrip 功能。
- 外部来源与许可证。

代表性迁移练习应给出实际起点文件/函数、需要学习者完成的改动、可观察的反例和验证边界。原有测试可作为回归基线，不能替代新任务验收；新测试名属于学习者待创建内容时要说明。Go练习使用`-run`筛选前先用`-list`确认测试存在，`no tests to run`或全跳过不能算完成。不要为练习提供已经填满的实现，未做真人试学仍保留读者状态。

正文内容要求见 [章节写作规范](book-chapter-contract.md)。topic 的技术状态不等于章节内容或读者验证状态。

## Metadata contract

```yaml
course:
  part: 1
  chapter: 1
topic_slug: hello-world
kind: original
status: planned
source: null
license: MIT
validation:
  offline: true
  e2e_gate: null
```

book-port 还必须记录 repository、path、revision、license、file map、behavior contract 和 deviations。

cmd/internal 示例在 `entrypoints` 列表中记录相对于 topic 的入口目录；单文件示例缺省为 `["."]`。需要参数的演示使用 `run_args`，例如 review-cli 的两组 -item。测试至少覆盖一个对应包，入口不能指向绝对路径或父目录。公开夹具 tests/fixtures/book-regions.json 仅描述当前阅读区域；自有开发历史不进入公开仓库。第三方来源信息不能以私有历史为由省略。

`kind` 允许值为 `original`、`book-port`、`warmup`、`elective`、`reading`。original 表示维护者有权授权的自有代码，source 为 null，license 明确；外部复制或改编使用 book-port 等适当分类并保留 source。参考链接放在 references，不把普通阅读链接当成代码复制证据。

## Validation contract

验证必须从仓库根目录执行并记录真实输出：格式、静态检查、race test、覆盖率和显式输出路径的构建。需要外部服务的实验必须提供离线 fake。
