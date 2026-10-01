# 如何使用本书

这本书把正文和示例代码放在同一个仓库中，学习记录由你保存在自己的笔记里。正文在 `book/`，示例在 `labs/partN-主题/chapterNN/示例名/`。每章的示例索引可以直接打开源码，不需要另外下载一份练习工程。

## 一次只完成一个可验证的改动

1. 读正文，先弄清输入、输出和要解决的问题。
2. 打开本章示例的 README，确认入口与运行方式。
3. 运行程序和测试，记下原来的结果。
4. 在副本中改一处代码，运行前先预测结果，再用实际输出检查判断。
5. 记录改了哪里、为什么改、哪个边界用例改变了结果，以及还解释不清的问题。

记录可以写在自己的笔记中。不要只保存一张测试通过的截图：如果未来忘了为什么加这个测试，具体输入和预期结果比“已学完”更有用。

## 阅读路线

| 部分 | 先学什么 | 再用什么检验 |
| --- | --- | --- |
| Go 基础 | 程序结构、声明、控制流、集合、函数与错误 | 接口、字符串、JSON、泛型、反射、并发与 HTTP |
| 后端实践 | 数据访问、事务和一致性边界 | 配置、日志、探针、关闭流程、Hertz 与 Kitex |
| Agent 开发 | 消息、模型接口、embedding 与检索 | 上下文组装、回答生成、稳定输出和 RAG 测试 |

工程实践穿插在语言学习之后：先比较单包示例和 cmd/internal 工程，再讨论职责是否值得拆分。完整路线见[阅读目录](README.md)。

<a id="running-examples"></a>

## 运行与练习

### 环境与工作目录

示例要求 Go 1.26.6 或更新的兼容版本，最低版本以 `labs/go.mod` 为准。命令默认从 `the-way-to-ai-engineer` 仓库根目录运行，先执行 `go version`，再确认当前位置有 `package.json`、`book/` 和 `labs/`。

后端和 Agent 示例还依赖 `labs/go.mod` 锁定的第三方包，首次运行前联网执行 `go -C labs mod download`；依赖缓存准备好后，默认测试无需访问远程模型或数据库。

单个示例只需要 Go 和该示例注明的运行条件。若要运行全仓 `make validate`，还需要 Node 22、npm、Ruby、make、POSIX shell、ripgrep，以及 race 检查使用的 C 工具链；准备方式见[贡献指南](../CONTRIBUTING.md#validation)。不要把维护整本书和构建课程站点需要的工具误当成开始第一个 Go 程序的前提。

课程站点的正文仍来自 `book/`。维护者首次构建站点时运行 `npm ci`，随后执行 `npm run dev` 本地预览，或用 `make validate-site` 完成内容清单、生成器反例和正式构建检查。`src/content/chapters/` 是构建产物，不要直接修改。

### 运行与测试

正文中的 `go -C labs run ./...` 运行一个 Lab，`go -C labs test ./... -count=1` 执行其测试。测试输出中的运行时间和机器路径可以变化，重点看退出码和测试结论。还没学过测试语法时，可以先按章节说明运行现有测试。

正文的命令在仓库根目录执行，例如：

```bash
go -C labs run ./part1-golang/chapter01/hello-world
go -C labs test ./part1-golang/chapter01/hello-world -count=1
```

完整测试使用：

```bash
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go -C labs test ./... -count=1
```

“离线”指不访问远程依赖和服务；HTTP/RPC 集成测试仍可能在本机回环地址监听端口。若系统禁止监听，先读对应 README 区分纯函数测试与本地集成测试，不要把环境限制误判为业务错误。

### 第 1、2 章的练习副本

示例的故意失败操作与练习在副本中进行。以下为 macOS/Linux shell 命令；`mktemp` 返回的位置每次不同，只用于本次操作。

```bash
practice_dir=$(mktemp -d)
cp labs/go.mod "$practice_dir/go.mod"
cp -R labs/part1-golang/chapter01/hello-world "$practice_dir/hello-world"
cp -R labs/part1-golang/chapter02/arrays-slices "$practice_dir/arrays-slices"
cp -R labs/part1-golang/chapter02/maps "$practice_dir/maps"
cd "$practice_dir"
```

复制完成后不再使用仓库中的完整 Lab 路径。现在运行 `go run ./hello-world` 或 `go test ./maps -count=1`。练习和故意失败操作都在副本中完成，原仓库文件不受影响。

每组独立实验重新从仓库根目录复制一份。连续练习按章节顺序使用同一份副本。不要把故障实验的错误代码带到练习起点。完成后可保留副本复盘，`/tmp` 下数据不适合作为长期笔记。

Windows 用户可在临时工作目录手动复制同样的文件和三个目录，再在该目录执行同样的 Go 命令；上面的 shell 初始化步骤只验证了 macOS 环境。

### 后续章节与分层工程

上面的扁平副本只适用于第 1、2 章的标准库练习。其他章节统一保留仓库中的 labs 路径；尤其 cmd/internal 工程的 import 包含完整 module 路径，不能只复制 cmd 或随意改变层级。

从仓库根目录准备一份完整示例副本：

```bash
practice_dir=$(mktemp -d)
cp -R labs "$practice_dir/labs"
cd "$practice_dir"
```

此后可直接执行正文中的 `go -C labs run ./...` 与 `go -C labs test ./...`。首次使用第三方依赖先联网执行 `go -C labs mod download`，之后用 `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off` 验证离线运行。review-cli 必须提供 -item 参数，完整命令见它的 README；无参数报错是预期输入校验，不是运行环境故障。

修改 IDL 的练习还需要指定版本的生成工具，按对应 README 在副本中操作。现有示例已经提交生成代码，普通运行和测试不需要重新生成。真实模型和数据库不在默认练习命令中启用；本机回环 HTTP/RPC 测试仍需要监听权限。

### 区分完整程序、摘录与错误示例

“源码摘录”应放回指定上下文，不是独立程序。“完整程序”可以保存成单独文件后执行。“错误示例”预期失败，错误文本可能因工具链版本不同而略有差异。教材中的完整程序可以在练习根目录保存为 `inspect.go`，用 `go run inspect.go` 运行；每次替换该文件内容，不把多个 `main` 放进同一个包。

章节中的 `<!-- verify:... -->` 是维护者验证标记，普通阅读可以忽略。测试脚本据此提取正文代码，避免另存一份示例后与正文分叉。

### 完成练习后检查什么

练习代码块会标出文件、任务和预期结果。省略 package/import 或函数剩余部分的块是摘录，不能直接覆盖整个文件。每题从新副本开始；题目明确写“接着上一题”时再沿用修改。第 1、2 章另有连续题的具体约定。

改函数签名后，连同 main 和测试调用一起检查。故意失败的练习只在副本操作；恢复后再跑已有测试。遇到 Go 代码中的 `TODO`、`panic("TODO: ...")` 或 `t.Fatal("TODO: ...")`，它们用于标出未完成的位置，不是最终实现。

## 配套阅读

- [Go 入门指南](https://learnku.com/docs/the-way-to-go)：系统查阅传统 Go 语法与标准库主题，泛型等较新的内容要配合当前 Go 文档。
- [前言](https://learnku.com/docs/the-way-to-go/preface/3561)：了解这套参考书的组织方式。
- [起源和发展](https://learnku.com/docs/the-way-to-go/origin-and-development/3562)：补充语言背景。
- [Go 程序的基本结构和要素](https://learnku.com/docs/the-way-to-go/the-basic-structure-and-elements-of-the-go-program/3583)：与第一个程序对照阅读。

[开始第 1 章](part1-golang/chapter01.md) · [返回目录](README.md)
