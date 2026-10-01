# 示例：Hello World

## Goal

运行最小 Go 程序，并理解如何把输出逻辑放进可测试函数。

## Concepts

- `package main`
- `func main()`
- `fmt.Println`
- 最小单元测试

## Run and test

```bash
go -C labs run ./part1-golang/chapter01/hello-world
go -C labs test ./part1-golang/chapter01/hello-world -race -count=1
go build -o /tmp/hello-world ./labs/part1-golang/chapter01/hello-world
```

预期运行输出：

```text
hello, go
```

## Code map

- `main.go`：定义可测试的 `message()` 和程序入口。
- `main_test.go`：固定最小行为契约。
- `meta.yml`：记录课程坐标和迁移来源。

## Exercise

在临时副本中完成欢迎语修改、参数化和测试扩展，步骤与验收见 [第 1 章练习](../../../../book/part1-golang/chapter01.md#练习)。先改预期观察失败，再改实现观察通过。

## Reading

返回 [Chapter 01](../../../../book/part1-golang/chapter01.md)。
