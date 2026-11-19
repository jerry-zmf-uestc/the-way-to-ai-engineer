# 示例： slice preallocate

参考 [Uber Go 风格指南：字符串与切片性能](https://github.com/xxjwxc/uber_go_guide_cn)。

## 规范要点

- 已知容量上限时使用 `make([]T, 0, n)` 预分配
- 避免 `append` 反复扩容带来的内存拷贝
- 同样适用于 `make(map[K]V, n)` 与 `bytes.Buffer`

## 反例

```go
var out []int
for _, v := range src {
    if v%2 == 0 {
        out = append(out, v)
    }
}
```

## 正例

见 `main.go` 中的 `FilterEven`：

- 使用 `make([]int, 0, len(src))` 预分配
- 即使全部被过滤掉，底层容量也保留到上限

## Run

```bash
go -C labs run ./part1-golang/chapter11/uber-slice-preallocate
go -C labs test ./part1-golang/chapter11/uber-slice-preallocate -race -count=1
```

## Code Map

- [main.go](main.go): contrasts the preallocated slice path with the naive
  append loop.
- [main_test.go](main_test.go): checks the filtered result shape while keeping
  the example bounded.

## Related Exercises

- [Chapter 11 · Engineering Exercises](../../../../content/book/part1-golang/chapter11.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Pair this topic with [string-builder](../uber-string-builder) to see the same
"known capacity first" idea applied to strings and slices.
