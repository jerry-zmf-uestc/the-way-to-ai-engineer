# 示例：数组与切片

## Goal

理解切片扩容、复制、窗口和边界检查，并避免无意修改输入切片。

## Concepts

- 切片长度与容量
- `append`
- `copy`
- 共享底层数组
- 非法窗口

## Run and test

```bash
go -C labs run ./part1-golang/chapter02/arrays-slices
go -C labs test ./part1-golang/chapter02/arrays-slices -race -count=1
```

## Exercise

在临时副本中观察 len/cap、共享窗口和独立副本，并检查非法边界，步骤见 [第 2 章练习](../../../../book/part1-golang/chapter02.md#collections-exercises)。本 Lab 不依赖旅行应用。

## Reading

返回 [Chapter 02](../../../../book/part1-golang/chapter02.md)。
