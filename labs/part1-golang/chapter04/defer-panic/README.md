# 示例： defer panic

## Goal

理解 defer 的执行语义、defer 参数的求值时机，以及 panic 的传播与 recover 的「选择性恢复」模式。

## Concepts

- defer 按后进先出（LIFO）顺序执行，命名返回值让 deferred 函数的修改对调用方可见。
- `defer f(x)` 的参数在 defer 语句执行时立即求值；`defer func() { ... }()` 闭包形式在执行时才读取变量。`defer trace("op")()` 模式利用这一点：trace 立即执行，返回的函数才被推迟。
- 内部逻辑可以用 panic 报告失败，最外层用命名返回值 + defer/recover 把 panic 转为 error。
- 「有选择的 recover」：用专用类型（`*parseError`）标识预期 panic，未预期的 panic 用 `panic(p)` 原样抛出，不被吞掉。
- Go 1.22 起 for 循环每轮迭代新建循环变量，闭包捕获的是当轮副本（旧版本共享同一变量，结果是 [3 3 3]）。

## Run

```bash
gofmt -l labs/part1-golang/chapter04/defer-panic
go -C labs vet ./part1-golang/chapter04/defer-panic
go -C labs test ./part1-golang/chapter04/defer-panic -race -count=1
go -C labs run ./part1-golang/chapter04/defer-panic
```

Expected output:

```text
enter demo
defer LIFO 顺序: [3 2 1]
defer 求值时机: immediate="start" late="end"
parsePositive("42") = 42
parsePositive("-3") err: not positive: -3
parsePositive("abc") err: not a number: abc
ratio(10, 2) = 5
闭包捕获循环变量: [0 1 2]
exit demo
```

## Code Map

- [main.go](main.go): 四个可测试函数分别演示 defer LIFO、defer 求值时机、recover 转 error 与闭包捕获循环变量；`trace` 演示 `defer trace("op")()` 模式。
- [main_test.go](main_test.go): 断言四个函数的预期输出，并验证未预期的运行时 panic（除零）会穿过选择性 recover 继续传播。

## Exercise

在临时副本中把 `ratio` 里除零的运行时 panic 也改成 recover 后返回 error，运行 `TestRatioUnexpectedPanicPropagates` 观察它失败，体会「吞掉未预期 panic」为什么危险：调用方再也区分不出程序 bug 与业务错误。练习与验收见 [第 4 章](../../../../content/book/part1-golang/chapter04.md)。

## Reading

返回 [Chapter 04](../../../../content/book/part1-golang/chapter04.md)。参考 [Go 语言圣经 · 第 5 章 函数](https://golang-china.github.io/gopl-zh/ch5/ch5.html)（defer、panic、recover 与 soleTitle 示例）。License: MIT。
