# 示例： function values

## Goal

理解函数是一等值：函数可以作为参数与返回值传递，闭包可以捕获并更新外层变量，可变参数与切片展开等价，递归需要明确的边界条件。

## Concepts

- 函数值：函数的类型（如 `func(int) int`）与普通类型一样可以作为参数传递，调用方决定行为，被调用方决定流程。
- 闭包：函数字面量捕获外层局部变量；外层函数返回后变量仍然存活（逃逸到堆上），每次调用看到的是同一份状态。
- 可变参数：`func sum(vals ...int)` 内部 `vals` 是 `[]int`；`sum(1, 2, 3)` 与 `sum(nums...)` 完全等价。
- 递归：函数调用自身，必须有终止条件；对没有数学定义的输入（如负数阶乘）要明确约定边界返回值。

## Run

```bash
gofmt -l labs/part1-golang/chapter04/function-values
go -C labs vet ./part1-golang/chapter04/function-values
go -C labs test ./part1-golang/chapter04/function-values -race -count=1
go -C labs run ./part1-golang/chapter04/function-values
```

Expected output:

```text
applyAll(加倍, nums) = [2 4 6 8]
applyAll(平方, nums) = [1 4 9 16]
闭包连续调用: 1 4 9 16
sum(1, 2, 3) = 6
sum(nums...) = 10
factorial(5) = 120
factorial(-1) = -1
```

## Code Map

- [main.go](main.go): `applyAll` 演示函数值作为参数；`squares` 返回捕获并更新外层变量的闭包；`sum` 演示可变参数；`factorial` 演示递归与负数的边界约定。
- [main_test.go](main_test.go): 断言 applyAll 传入加倍/平方两种函数的结果、闭包连续四次调用返回 1/4/9/16、sum 的空参与切片展开、factorial 的 0/5/-1。

## Exercise

在临时副本中把 `factorial` 的递归终止条件 `n == 0` 删掉，运行 `TestFactorial` 观察栈溢出（runtime stack overflow），体会递归为什么必须有明确的终止条件。练习与验收见 [第 4 章](../../../../book/part1-golang/chapter04.md)。

## Reading

返回 [Chapter 04](../../../../book/part1-golang/chapter04.md)。参考 [Go 语言圣经 · 第 5 章 函数](https://golang-china.github.io/gopl-zh/ch5/ch5.html)（函数值、匿名函数、可变参数与递归示例）。License: MIT。
