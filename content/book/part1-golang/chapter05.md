# 第 5 章：错误处理与异常

程序只要开始解析输入、读文件、调网络，就一定会失败。Go 的答案很极端：没有异常体系，错误就是一个普通返回值。读懂这套设计，是写出"凌晨被 oncall 翻出来也扛得住"的代码的第一步。

## 5.1 错误是值

Go 的惯例：最后一个返回值承载 `error`（或 `bool` 表示成功）。`if err != nil` 的重复是必要的——不要试图抽象掉它。

`labs/part1-golang/chapter05/functions-errors` 的数据流很典型：`parseInts` 管输入解析，`sumAvg` 管业务计算，`main` 管入口编排。摘录自 [functions-errors/main.go](../../../labs/part1-golang/chapter05/functions-errors/main.go)：

```go
var (
	errNoInput      = errors.New("need at least one integer")
	errEmptyNumbers = errors.New("empty numbers")
)

func parseInts(args []string) ([]int, error) {
	if len(args) == 0 {
		return nil, errNoInput // 哨兵错误：稳定的失败类型，调用方用 errors.Is 判定
	}

	nums := make([]int, 0, len(args))
	for _, arg := range args {
		n, err := strconv.Atoi(arg)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", arg, err) // %w 保留错误链
		}
		nums = append(nums, n)
	}
	return nums, nil
}
```

运行它（当前示例故意走错误路径）：

```bash
go -C labs run ./part1-golang/chapter05/functions-errors
# 输出：error: need at least one integer
```

五种处理策略，按使用频率排：

1. **传播**：直接返回，或用 `fmt.Errorf` 添加上下文。错误信息小写开头、不带换行，便于逐层包成因果链。
2. **重试**：只用于偶发错误，必须限次数、加退避、设总 deadline。
3. **终止程序**：`log.Fatalf` / `os.Exit(1)` 只能出现在 main；库函数永远传播错误。
4. **只记日志**：不中断，但同一错误别在每层重复打印。
5. **明确忽略**：少数场景可以（如清理临时文件），但要写注释说明意图。

两个细则：返回 non-nil error 时，其余返回值应被忽略（例外如 `io.Read` 返回已读字节）；`io.EOF` 是最著名的哨兵错误，用 `errors.Is(err, io.EOF)` 判定。

## 5.2 panic：不可恢复的路径

数组越界、nil 解引用这类运行时错误会引发 panic：**中断执行 → 沿栈展开所有 deferred 函数 → 崩溃并打印堆栈**。主动 `panic(v)` 只用于"逻辑上不应到达"的路径，比如穷举 switch 的 default。

判断标准一句话：**调用方能合理预料的失败用 error；程序内部矛盾才 panic。** `regexp.MustCompile`、`template.Must` 的 `Must` 命名惯例是例外：输入确定合法时，用 panic 免去错误检查。

defer 在 panic 路径上同样执行——这既是资源清理的保障（第 4 章），也是 recover 能工作的前提。

## 5.3 recover：只在边界恢复

`recover` **只在 deferred 函数里有效**：外围函数正在 panic 时，它中止传播并返回 panic 值；否则返回 nil。典型用法是把包内部的 panic 转成 error：

```go
// 教学变体：有选择的 recover（gopl soleTitle 模式）。
func parse(input string) (result AST, err error) {
	type bailout struct{} // 专用类型标记"预期内"的 panic
	defer func() {
		switch p := recover(); p {
		case nil:       // 无 panic
		case bailout{}: // 预期内：转为 error
			err = fmt.Errorf("internal parse error: %v", p)
		default:
			panic(p) // 未预期的继续抛，不吞
		}
	}()
	// ...解析逻辑，无法继续时 panic(bailout{})
}
```

[defer-panic](../../../labs/part1-golang/chapter04/defer-panic) 示例的 `parsePositive` 完整演示了这个模式：预期内的解析失败转为 error，未预期的 runtime panic 原样传出。

边界规则：只恢复本包引起的预期 panic；公有 API 返回 error 而非 panic；panic 之后包级状态可能不一致，恢复后不要假装什么都没发生。net/http 为不杀进程 recover 每个 handler 的 panic，是工程折中，不是鼓励滥用。

## 踩坑清单

### 包装错误用了 %v，错误链断了

```go
return nil, fmt.Errorf("parse failed: %v", err) // 反例：链断了
```

现象：调用方 `errors.As(err, &numErr)` 再也取不到 `*strconv.NumError`。修复：改 `%w`。验证：副本中改回 `%w` 后 `errors.As` 恢复命中。

### 吞掉错误，返回"看起来合法"的零值

```go
if len(nums) == 0 {
	return 0, nil // 反例：0 也是合法平均值，调用方分不清
}
```

现象：空输入被当成成功结果参与后续计算。修复：返回 `errEmptyNumbers`。验证：`errors.Is(emptyErr, errEmptyNumbers)` 通过，`[1,2]` 仍返回 sum=3、avg=1.5。

### recover 吞掉未预期的 panic

```go
defer func() {
	if p := recover(); p != nil {
		log.Println(p) // 反例：什么 panic 都记一笔就完事，bug 被静默
	}
}()
```

现象：数组越界这类真 bug 被记成一条日志，程序带着不一致的状态继续跑。修复：用专用类型区分预期 panic，其余 `panic(p)` 继续抛——见 5.3 的模式。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 5.1 检查包装错误 | 用 `parseInts([]string{"3", "bad"})` 触发失败；再把 `%w` 改成 `%v` 对比 | 错误文字包含 bad；`errors.As` 能取到 `*strconv.NumError`；改 `%v` 后判定失败 | `functions-errors/main.go`、`main_test.go` |
| 5.2 有选择地 recover | 分别测试 `parsePositive("42")`、`("abc")`、`("-3")`；用 defer/recover 断言未预期 panic 仍传出 | "abc"/"-3" 转为 error；"42" 正常；runtime panic 不被吞掉 | `labs/part1-golang/chapter04/defer-panic` 的 `main_test.go` |

完成后运行：

```bash
go -C labs test ./part1-golang/chapter05/... -count=1
go -C labs test ./part1-golang/chapter04/defer-panic/... -count=1
```

更多练习（哨兵错误、空输入边界、recover 后状态检查）见各 lab 的 README。

记住这几条：

- **错误是值：传播为主，终止只发生在 main。**
- **`%w` 保链、`%v` 断链；判定用 errors.Is/As，不依赖文案。**
- **panic 是内部矛盾的信号；recover 只放边界，只恢复预期内的 panic。**

## 链接

- [Go 程序设计语言（gopl）第 5 章：错误、Panic、Recover](https://golang-china.github.io/gopl-zh/ch5/ch5.html)：本章主要参考，含 WaitForServer、soleTitle 完整示例。
- [Go 官方错误处理说明](https://go.dev/blog/go1.13-errors)：`%w`、Is、As 的权威约定。
- [错误处理](https://learnku.com/docs/the-way-to-go/131-error-handling/3674)：补充阅读。

[返回全书目录](../README.md) · [上一篇：函数](chapter04.md) · [下一篇：结构体与方法](chapter06.md)
