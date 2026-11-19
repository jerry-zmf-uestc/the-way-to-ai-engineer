# 示例： errors wrap

参考 [Uber Go 风格指南：错误处理](https://github.com/xxjwxc/uber_go_guide_cn)。

## 规范要点

- 错误信息小写开头、不带句号
- 用 `fmt.Errorf("...: %w", err)` 包装，不要字符串拼接
- 判定错误统一用 `errors.Is` / `errors.As`
- Sentinel error 命名以 `Err` 开头并放包级常量

## 反例

```go
// 反例 1：吞掉错误
data, _ := os.ReadFile(path)

// 反例 2：字符串比较
if err.Error() == "file not found" { ... }

// 反例 3：包装失去原因
return fmt.Errorf("load failed: " + err.Error())
```

## 正例

见 `main.go`：

- `LoadConfig` 用 `%w` 包装底层错误
- 调用方用 `errors.Is(err, ErrConfigNotFound)` 判别
- 同时仍能识别底层 `os.ErrNotExist`，调用方诊断能力不丢失

## Run

```bash
go -C labs run ./part1-golang/chapter11/uber-errors-wrap
go -C labs test ./part1-golang/chapter11/uber-errors-wrap -race -count=1
```

## Code Map

- [main.go](main.go): shows the wrapped error path and the sentinel-error check
  side by side.
- [main_test.go](main_test.go): verifies that callers can still detect both the
  project-level and underlying filesystem errors.

## Related Exercises

- [Chapter 11 · Engineering Exercises](../../../../content/book/part1-golang/chapter11.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Read [chapter5/functions-errors](../../chapter05/functions-errors) first for
the language-level error model, then come back here to see the stricter
engineering conventions.
