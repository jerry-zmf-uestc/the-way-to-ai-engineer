# 示例： Project Layout

本示例把 `golang-standards/project-layout` 的目录骨架缩小到当前学习仓库真正需要维护的职责清单。

## 学习目标

- 区分当前正在使用的目录和暂缓引入的目录。
- 区分章节目录、设计说明、cmd 入口和内部包的职责。示例中的规划目录并不要求在工作区实际创建。
- 避免为了追求完整骨架而提前引入 `pkg/`、`api/`、`deployments/` 等目录职责。

## 运行

```bash
go -C labs test ./part1-golang/chapter11/project-layout -race -count=1
go -C labs run ./part1-golang/chapter11/project-layout
```

## Code Map

- [main.go](main.go)：把当前仓库真正采用的目录职责输出成一份可执行清单。
- [main_test.go](main_test.go)：保证目录边界和教学输出保持一致。

## 当前取舍

- 学习 topic 继续放在 `chapterN/<topic>/`。
- 公开变更在 Issue/PR 说明需求、设计、任务和验收；本地规格是维护者可选工具，不是运行示例的前提。
- 真实练手项目的入口可以放在 `cmd/<name>/main.go`，业务逻辑再进入 `internal/`。
- `pkg/` 只有在对外复用 API 已经稳定后才引入。

## 相关练习

- [Chapter 11 · Engineering Exercises](../../../../content/book/part1-golang/chapter11.md#练习)：回到正文里的练习区，按伪代码和验收标准继续
  扩展这个 topic。

## Continue

- <https://github.com/golang-standards/project-layout>
- <https://go.dev/doc/modules/layout>
- Module 布局：<https://go.dev/doc/modules/layout>
