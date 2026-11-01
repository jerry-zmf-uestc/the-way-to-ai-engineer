# 示例：值、变量与常量

## Goal

理解常量、局部变量、类型和布尔分支如何共同生成稳定结果。

## Concepts

- `const`、`var` 与 `:=`
- 静态类型和零值
- `if` 分支
- `fmt.Sprintf`

## Run and test

```bash
go -C labs run ./part1-golang/chapter02/values-variables-constants
go -C labs test ./part1-golang/chapter02/values-variables-constants -race -count=1
```

预期运行输出：

```text
Go example v1 is stable
```

## Exercise

新增目的地、旅行天数和预算变量，用一个常量表示默认币种；为零预算和非零预算分别增加测试。

## Reading

返回 [Chapter 02](../../../../content/book/part1-golang/chapter02.md)。

