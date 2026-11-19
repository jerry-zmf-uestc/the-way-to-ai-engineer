# 示例： Module Layout

本示例记录官方 `Organizing a Go module` 对不同项目形态的目录建议，并映射到当前学习仓库的实践阶段。

## 学习目标

- 区分单一 package、单一 command、混合仓库和 server 项目的布局取舍。
- 理解什么时候需要 `cmd/` 和 `internal/`，什么时候保持根目录简单即可。
- 避免把学习示例过早搬进 `cmd/`、`internal/` 或 `pkg/`。

## 运行

```bash
go -C labs test ./part1-golang/chapter11/module-layout -race -count=1
go -C labs run ./part1-golang/chapter11/module-layout
```

## Code Map

- [main.go](main.go)：把仓库目录边界压缩成可执行的三级模型输出。
- [main_test.go](main_test.go)：锁住目录职责说明，避免文档和示例输出漂移。

## 项目取舍

- 当前仓库的基础和规范学习示例继续放在 `chapterN/<topic>/`。
- 真正的可执行练手项目放在所属章节下的 `chapterN/<name>/cmd`，例如已有的 `json-api` 和 `review-cli`。
- 服务端实践开始后，入口放 `chapterN/<service>/cmd/main.go`，私有业务逻辑放 `chapterN/<service>/internal/<domain>/`。
- `/pkg` 只在确实需要对外复用的包出现后再引入，不为了布局完整而提前创建代码。

## 目录演进三级模型

- L1：学习示例。默认放在 `chapterN/<topic>/`，重点是讲清一个 topic。
- L2：重型示例 / capstone。即使出现多包、私有 `internal/`、生成代码，只要主要用途还是教学归档，仍然留在所属章节目录。
- L3：仓库级练手项目。只有当示例开始承担稳定入口、持续迭代、仓库级维护责任时，才升级为 `chapterN/<name>/{cmd,internal}`。

可以用一个简单问题判断：这个目录的首要目标还是“帮助学习一个 topic”，还是“作为仓库级 app 长期演进”？前者留在 `chapterN/<topic>/`，后者才升级到 `chapterN/<name>/{cmd,internal}`。

## 相关练习

- [Chapter 11 · Engineering Exercises](../../../../content/book/part1-golang/chapter11.md#练习)：回到正文里的练习区，按伪代码和验收标准继续
  扩展这个 topic。

## Continue

See the official module layout guide at <https://go.dev/doc/modules/layout>, then
compare it with [project-layout](../project-layout) and the `json-api` /
`review-cli` examples in this chapter.
