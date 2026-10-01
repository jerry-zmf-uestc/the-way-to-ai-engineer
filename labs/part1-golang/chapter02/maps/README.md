# 示例：Map

## Goal

使用 map 完成计数、查找和保持输入顺序的去重。

## Concepts

- `make(map[K]V)`
- `value, ok`
- 计数
- 去重与输入顺序

## Run and test

```bash
go -C labs run ./part1-golang/chapter02/maps
go -C labs test ./part1-golang/chapter02/maps -race -count=1
```

## Exercise

在临时副本中为计数增加大小写归一化，然后合并两个计数结果，保持输入不变。步骤、空输入约定及验收见 [第 2 章练习](../../../../book/part1-golang/chapter02.md#collections-exercises)。

## Reading

返回 [Chapter 02](../../../../book/part1-golang/chapter02.md)。
