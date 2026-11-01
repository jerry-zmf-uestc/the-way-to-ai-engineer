# 示例： string builder

参考 [Uber Go 风格指南：字符串与切片性能](https://github.com/xxjwxc/uber_go_guide_cn)。

## 规范要点

- 循环中拼接字符串必须用 `strings.Builder`
- 已知大致容量时调用 `Grow` 预分配
- `+=` 在循环里每次都重新分配整个底层数组

## 反例

```go
out := ""
for _, t := range tags {
    out += t + ","
}
```

## 正例

见 `main.go` 中的 `JoinTags`：

- 用 `strings.Builder`
- 调用 `Grow` 预估容量
- 不在循环中拼字符串

## Run

```bash
go -C labs run ./part1-golang/chapter11/uber-string-builder
go -C labs test ./part1-golang/chapter11/uber-string-builder -race -count=1
```

## Code Map

- [main.go](main.go): shows `strings.Builder` plus `Grow` in the smallest useful
  loop.
- [main_test.go](main_test.go): keeps the joined output stable while the
  implementation demonstrates capacity planning.

## Related Exercises

- [Chapter 11 · Engineering Exercises](../../../../content/book/part1-golang/chapter11.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Pair this topic with [slice-preallocate](../uber-slice-preallocate) to compare
string capacity planning and slice capacity planning side by side.
