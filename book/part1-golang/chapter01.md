# 第 1 章：Go 环境与第一个程序

从 Python 或 JavaScript 转来的读者，第一个 Go 程序只有 8 行，最初的困惑却往往不在语法：包名为什么必须叫 `main`？删了一行打印，编译反而报错？明明改对了代码，`go run` 正常，`go test` 却红了？这些问题的答案都指向本章的两条入口规则：**可执行程序从 `main` 包的 `func main()` 开始；把"生成内容"放进有返回值的函数，逻辑才能脱离终端输出、被直接测试。** 本章用一行欢迎语，把这两条规则连同 `go run` / `go test` / `go build` 三条命令一次跑通。

## main 包与 main 函数

**可执行程序的入口包必须叫 `main`，且其中必须有无参数、无返回值的 `func main()`。** 程序启动时先完成相关包（包括被导入的包）的初始化，再调用 main——"main 是入口"不能理解成进程启动后第一件事就是执行 main（初始化细节留到后续章节，本章没有显式初始化函数）。[Go 规范：程序执行](https://go.dev/ref/spec#Program_execution)

熟悉 Python 的读者可以对照：`if __name__ == "__main__":` 只是社区约定，解释器并不检查；Go 的入口是编译器强制的契约——入口包不叫 `main`，`go run` 会直接拒绝执行（[踩坑清单](#踩坑清单)第一条就是这个错误）。契约换来的是确定性：拿到任何一个陌生的 Go 可执行程序，不看内容也知道从哪里开始读。

| 写法 | 在本章中的作用 |
| --- | --- |
| `package main` | 声明该文件属于可执行程序的入口包 |
| `import "fmt"` | 让当前文件可以使用标准库 fmt 的导出功能 |
| `func main()` | 包初始化完成后的程序入口 |
| `func message() string` | 返回一段文字，方便调用与测试 |

包名与目录名不必相同：本章目录叫 [hello-world](../../labs/part1-golang/chapter01/hello-world/)，文件仍声明 `package main`。编译器只认文件里的包声明，不认文件夹名字。

## 第一个 Go 程序

**把"生成内容"和"输出内容"分成两个函数，返回值才能被直接测试。** 下面是 [main.go](../../labs/part1-golang/chapter01/hello-world/main.go) 的完整程序，与 Lab 保持一致。

<!-- verify:hello -->
```go
package main

import "fmt"

func message() string {
	return "hello, go"
}

func main() {
	fmt.Println(message())
}
```

逐行看这个程序：

1. `package main`：声明入口包；同一目录下的所有 `.go` 文件必须共享同一个包名。
2. `import "fmt"`：导入标准库的格式化 I/O 包；Go 以包为单位导入，导入而未使用是编译错误（见[踩坑清单](#踩坑清单)）。
3. `func message() string`：声明一个无参数、返回 `string` 的函数；函数定义本身不执行任何代码。
4. `return "hello, go"`：把字符串交还给调用方——返回不等于打印，这一行不会在终端产生任何输出。
5. `func main()`：包初始化完成后的程序入口。
6. `fmt.Println(message())`：先调用 `message` 取回欢迎语，再把结果写到标准输出并追加换行。

函数定义不执行：`message` 写在 `main` 上面也不会先执行，要等 `main` 中的调用表达式执行才取回欢迎语。括号是参数列表（`message()` 为空），括号后的 `string` 是返回值类型。

入口只打印一行时，直接写 `fmt.Println("hello, go")` 也完全合法。什么时候值得抽出一个函数：

| 场景 | 推荐写法 | 理由 |
| --- | --- | --- |
| 一次性演示、临时脚本 | `main` 里直接 `fmt.Println("hello, go")` | 少一层间接，一眼看到输出 |
| 需要测试这段文字 | 抽成 `message() string` | 测试直接比较返回值，不必捕获终端输出 |
| 多处使用同一段文字 | 抽成函数 | 生成逻辑只写一份，修改只有一处 |

本章选择第二种写法，正是为了下一节的 `go test`。

## 运行、测试和构建

三条命令的成功信号各不相同：

| 命令 | 用途 | 成功时观察什么 |
| --- | --- | --- |
| `go run` | 编译并立即运行 | 程序的标准输出 |
| `go test` | 执行测试 | 退出码 0、测试通过 |
| `go build` | 生成程序 | 指定位置出现可执行文件 |

行为差异值得记住：`go run` 把程序编译到临时位置、运行结束后清理临时二进制，适合边改边看结果；`go test` 收集包内所有以 `_test.go` 结尾的文件执行；`go build` 把编译产物留在你指定的位置，可以反复执行，也可以交付给没有安装 Go 的机器。

以下命令从仓库根目录（同时放着 `package.json`、`book/` 和 `labs/` 的项目目录）执行；编辑器只打开了 hello-world 子目录时，先回到根目录：

```bash
go version
go -C labs run ./part1-golang/chapter01/hello-world
```

`go -C labs` 让 go 命令先切换到 `labs` 目录再执行（`-C` 必须放在参数列表最前面，Go 1.20 起可用；本仓库以 [labs/go.mod](../../labs/go.mod) 声明的 Go 1.26.6 为基准），这样所有 Lab 共用同一份依赖清单。`./part1-golang/chapter01/hello-world` 指定包所在目录——`./` 前缀表示相对当前目录（即 labs）。第二条命令输出：

```text
hello, go
```

排错先分清哪一步出错：第一条命令就提示找不到 go，先检查安装和终端环境（这时还没运行到示例代码）；版本能显示而第二条找不到目录，再检查终端位置。

### 检查返回值

**测试不捕获终端输出，直接比较返回值。** 下面是 [main_test.go](../../labs/part1-golang/chapter01/hello-world/main_test.go) 的完整测试文件：

<!-- verify:hello-test -->
```go
package main

import "testing"

func TestMessage(t *testing.T) {
	if got := message(); got != "hello, go" {
		t.Fatalf("message() = %q, want %q", got, "hello, go")
	}
}
```

逐行看这个测试：

1. `package main`：测试与被测代码同包，因此可以直接调用未导出的 `message`。
2. `import "testing"`：测试框架就在标准库里，无需安装第三方依赖。
3. `func TestMessage(t *testing.T)`：`go test` 收集所有以 `Test` 开头、签名为 `func(*testing.T)` 的函数逐个执行。
4. `if got := message(); got != "hello, go"`：分号把"取结果存入 `got`"和"再比较"隔在同一个 `if` 中，`got` 只在分支内有效。
5. `t.Fatalf(...)`：报告失败并结束当前测试；`%q` 给字符串加引号，末尾空格、不可见字符的差异一眼可见。

测试里的预期文字手写，不随实现自动更新——欢迎语被误改时测试会指出差异。但测试只检查 `message` 的返回值，没检查 `main` 是否真的打印了它，前面的运行检查仍要保留。

```bash
go -C labs test ./part1-golang/chapter01/hello-world -count=1
```

测试通过时退出码为 0，输出含 `ok` 和包路径；`-count=1` 关闭测试结果缓存，让测试每次实际重跑（不带它时，代码未变的重复运行会直接复用上一次的 `ok`），时间字段可以不同。当实现与预期不一致时，失败输出的关键行长这样（时间、路径随环境变化）：

```text
--- FAIL: TestMessage (0.00s)
    main_test.go:7: message() = "hello, reader", want "hello, go"
```

先读这一行，再决定是改实现还是改测试。[Go 官方测试教程](https://go.dev/doc/tutorial/add-a-test)

### 构建独立程序

macOS/Linux 下：

```bash
build_dir=$(mktemp -d)
go build -o "$build_dir/hello-world" ./labs/part1-golang/chapter01/hello-world
"$build_dir/hello-world"
```

仍输出 `hello, go`。`mktemp -d` 建一个临时目录，`-o` 指定产物路径；`go build` 成功时通常没有文本输出。纯标准库程序编译出的是自包含二进制，可直接执行，也可拷贝到没有安装 Go 的同平台机器上运行——不必每次 `go run`。改过源码要重新构建，否则运行旧文件看到的仍是上次构建的结果。

## 给函数增加参数

**参数把"这次使用哪个名字"留给调用方决定，函数体不必为每个名字改写一遍。** 下面是完整教学程序（不是现有 Lab 的原始实现），保存为练习副本根目录的 `inspect.go`，运行 `go run inspect.go`：

<!-- verify:greet -->
```go
package main

import "fmt"

func greet(name string) string {
	return "hello, " + name
}

func main() {
	fmt.Println(greet("gopher"))
}
```

输出为 `hello, gopher`。`name string` 声明参数名和类型；`+` 拼接字符串。函数只返回内容，用哪个名字、是否打印由调用方决定，测试也能用不同输入多次调用。

两条边界先约定清楚：空字符串传入时返回 `"hello, "`（保留末尾空格），本章先不做输入校验，练习 1.2 会把这个边界写进测试；给已有函数加参数是一次签名变化——定义、所有调用方、所有测试调用必须一起改，漏掉任何一处都会编译失败，编译器会逐个点名。

## 踩坑清单

前两个坑各自从一份新的练习副本开始验证（创建方法见[示例与练习说明](../introduction.md#running-examples)）。

### 包名不对，程序无法运行

```go
package hello // 反例：入口包不叫 main
```

**症状**：`go run ./hello-world` 失败——先是同一目录发现 `hello` 和 `main` 两种包（测试文件仍是 `package main`）；把 main_test.go 也改为 `package hello` 后，Go 报告该包不是 main 包。

**原因**：`go run` 要求目标目录是一个可执行包——包名是 `main` 且含 `func main()`。函数叫 main 并不够，入口包也必须叫 main。

**修复**：两个文件都恢复为 `package main`，`go run ./hello-world` 和 `go test ./hello-world -count=1` 应恢复正常。

### 输出变了，测试却没有同步

```go
func message() string {
	return "hello, reader" // 反例：测试仍按 "hello, go" 断言
}
```

**症状**：程序照常运行、输出 `hello, reader`，但 `go test ./hello-world -count=1` 失败并指出实际值与 `hello, go` 不同。

**原因**：测试断言里的预期文字是手写常量，不随实现自动更新——代码能编译，只是新文字与测试要求的旧文字不一致。

**修复**：有意改输出，就同步测试断言里和失败信息里的两处预期值；改错了就恢复实现。不要只删断言让测试通过。

### 把所有逻辑放进 main

**症状**：照样能编译、运行，但生成内容、判断输入、打印结果全写在一起，测试只能从终端输出检查结果。

**原因**：终端输出是程序最不稳定的一层接口（换行、格式、缓冲都可能变化），直接比较返回值才是稳定契约。

**修复**：生成内容的代码放进有返回值的函数，main 只调用和打印——本章 `message`、`greet` 就是；不必为小例子预建多层目录。

### 导入了却没有使用

```go
import (
	"fmt"
	"strings" // 反例：导入了却没有使用
)
```

**症状**：编译失败，报 `imported and not used: "strings"`。最常见的触发是删掉了最后一个用到该包的语句，却忘了同步删掉 import。

**原因**：Go 把未使用的导入视为编译错误而非警告——它意味着代码与意图不一致。同理，声明了局部变量却不使用，会报 `declared and not used`。

**修复**：删掉未使用的 import 即可；让编辑器挂上 goimports，保存时自动增删导入。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 1.1 修改欢迎语 | 先把测试两处预期文字改为 hello, reader 并观察失败（失败信息应指出 want "hello, reader"），再把 message 返回值改为 hello, reader；函数签名不变 | 程序输出 hello, reader，测试通过 | `hello-world/main.go`、`main_test.go` |
| 1.2 欢迎语接受名字 | 给 message 增加 `name string` 参数并改为拼接 `"hello, "` 和 name；同步 main 调用与旧测试（漏改调用会报参数不足），再为 "gopher" 和空名字各加一条测试 | 程序输出 hello, gopher；三种输入的测试全部通过，空名字返回 `"hello, "`（保留末尾空格，暂不增加输入校验） | `hello-world/main.go`、`main_test.go` |

练习从一份新的 `hello-world` 练习副本开始（创建方法见[示例与练习说明](../introduction.md#running-examples)）；两题沿用同一份副本，1.2 在 1.1 的完成态上继续。三道小题带 TODO 注释的起点代码见 [tests/fixtures/chapter01-exercises.md](../../tests/fixtures/chapter01-exercises.md)，依次对应 1.1 与 1.2 的两个阶段。

每题都在练习副本根目录运行：

```bash
go run ./hello-world
go test ./hello-world -v -count=1
```

`-v` 会逐个列出测试函数名，方便确认 1.2 新增的两条测试确实被执行。更多练习见各 lab 的 README。

## 链接

- [Go 程序的编写方式](https://go.dev/doc/code)：包、module、构建与安装的官方说明，读完能解释本章 `go -C labs` 背后的 module 布局。
- [Go 规范：程序执行](https://go.dev/ref/spec#Program_execution)：入口与初始化顺序的准确定义，学到 init 函数时回来核对。
- [Go 官方测试教程](https://go.dev/doc/tutorial/add-a-test)：在真实模块里边写边测，延续本章的 TestMessage。
- [The Way to Go：程序基本结构](https://learnku.com/docs/the-way-to-go/the-basic-structure-and-elements-of-the-go-program/3583)、[文件名与标识符](https://learnku.com/docs/the-way-to-go/file-name-keyword-and-identifier/3582)、[编译与运行](https://learnku.com/docs/the-way-to-go/build-and-run-go-programs/3576)：补充程序组织、命名和编译运行的说明；工具链细节优先以当前官方文档为准。

[返回全书目录](../README.md) · [下一篇：数据类型](chapter02.md)
