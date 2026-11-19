# 第 3 章：控制流与边界分支

配套实验：[control-flow](../../../labs/part1-golang/chapter03/control-flow/README.md)。

第 2 章解决了"数据长什么样"，这一章解决"数据进来之后往哪走"。想象一个给 AI 应用打分的回调接口：请求体里只有一个 `score` 字段，看似无害；但如果处理函数先判 `score >= 90`、把非法值检查放在最后，一条 `{"score": 101}` 的脏数据就会被评成 excellent 写进报表——分支写对了，次序错了，结果照样错。Go 的控制流只有 `if`、`for`、`switch` 三样，但判断顺序本身就是正确性：先拦非法值再走主路径，用 `continue` 把无关情况尽早踢出循环。后面的参数校验、重试逻辑、请求路由、worker pool，底层都在重复这套机制。

## 3.1 守卫式 if：先拦非法值，再走正常路径

**分支先判断合法范围，再判断等级**——次序颠倒，101 就会被归为优秀。摘录自 control-flow/main.go。

```go
func labelScore(score int) string {
	if score < 0 || score > 100 {
		return "invalid"
	}
	if score >= 90 {
		return "excellent"
	}
	if score >= 60 {
		return "pass"
	}
	return "retry"
}
```

这段代码的读法是从上到下"过闸机"：

- 第一道闸 `score < 0 || score > 100` 拦住一切非法输入。它放在函数开头，后面的每个分支都可以放心假设 score ∈ [0, 100]，不用各自再校验。
- 之后按从高到低判 90、60：先命中的分支直接 return，后续条件天然落在"剩余区间"里，所以不用写 `score >= 60 && score < 90` 这种双边条件。
- 末尾的 `return "retry"` 是兜底出口：所有区间都有归宿，函数不存在"漏判"的路径。

这种写法有名字：**守卫子句（guard clause），也叫早返回（early return）**。类比机场安检——违禁品在门口拦下，登机流程里不再检查行李。会 Python 的读者可以对照 `raise ValueError` 的习惯：Go 的风格是把所有校验堆在函数顶部，让主路径始终保持最左缩进；每多一层嵌套，读者就要多记一层上下文。

- 测试至少覆盖 -1、0、59、60、89、90、99、100、101；只测 80 和 95，看不出分支次序的问题。注意这些值几乎都取自判断边界的两侧（±1），边界两侧才是次序错误暴露的地方。
- 不带表达式的 `switch` 从上到下检查条件，只执行首个匹配分支，不需要写 `break`（Go 的 case 默认不贯穿，真想贯穿要显式写 `fallthrough`，实际很少需要）。把 if 链改成 switch 时必须保留非法输入和及格分支——形式变短不能证明行为等价，让两个实现同时接受同一组边界数据比较。

if 链和无表达式 switch 表达力相同，选择看场景：

| 维度 | if-else if 链 | 无表达式 switch |
| --- | --- | --- |
| 条件写法 | 每个分支任意布尔表达式 | `case` 后跟布尔表达式，语义与 if 链等价 |
| 匹配行为 | 首个为真的分支执行后结束 | 从上到下只执行首个匹配分支，默认不贯穿 |
| 可读性 | 分支少时紧凑；分支多了向右堆、条件参差不齐 | 条件纵向对齐，对同一变量的区间/状态判断更醒目 |
| 何时选 | 分支少、条件异质（如同时判 error 和值） | 对同一变量做一组互斥区间或状态分类 |

## 3.2 for + continue：跳过当前轮次，比多层嵌套更清楚

Go 用同一个 `for` 表达不同循环形式，功能并未减少。摘录自 control-flow/main.go 的累加函数，用 `continue` 把奇数尽早踢出循环：

```go
func sumEven(limit int) int {
	total := 0
	for n := 0; n <= limit; n++ {
		if n%2 != 0 {
			continue
		}
		total += n
	}
	return total
}
```

`sumEven(10)` 实际执行 11 轮：n 取 0 到 10，其中 5 个奇数在 `continue` 处被踢掉，6 个偶数 0+2+4+6+8+10 累加得 30。循环体里剩下的代码就是"主路径"，不被无关情况裹进嵌套——这和守卫子句是同一个思想，只是战场从函数入口挪到了循环体入口。

- `continue` 跳过本轮循环剩余语句，`break` 退出当前循环；两者都不会自动退出包住循环的函数。
- limit<0 不进入循环；limit=0 会执行 n=0 这一轮，但总和仍为 0；大数累加可能超出 int 范围，本例未实现溢出检测。循环起点与终止条件共同决定是否执行循环体——总和相同，不代表执行次数相同。

除了上面这种"初始化；条件；步进"的计数循环，其他语言的 while、do-while 在 Go 里也都归并到 `for`：

```go
// 教学变体：for 的三种形态。
for i := 0; i < 3; i++ { /* 初始化；条件；步进——最常见的计数循环 */ }
for err != nil {        /* 只有条件——相当于其他语言的 while */ }
for {                   /* 无条件——无限循环，靠 break 或 return 退出（重试、轮询常用） */ }
```

[control-flow](../../../labs/part1-golang/chapter03/control-flow) 示例把这两个函数做成了可运行程序：

```bash
go -C labs run ./part1-golang/chapter03/control-flow
```

```text
excellent
30
```

两行输出分别对应 `labelScore(91)` 和 `sumEven(10)`。

## 3.3 for-range：遍历集合的统一写法

`for-range` 遍历切片、map、字符串和通道，**每次迭代返回"下标/键 + 元素副本"**：

```go
// 教学变体：range 遍历三种集合。
scores := []int{90, 60, 75}
for i, s := range scores { // i 是下标，s 是元素的副本
	_ = i
	_ = s
}

counts := map[string]int{"go": 2}
for word, count := range counts { // map 遍历顺序随机，不要依赖
	_ = word
	_ = count
}

for i, r := range "你好" { // 字符串按 rune 解码，i 是字节下标
	_, _ = i, r
}
```

四种遍历对象的行为速查：

| 遍历对象 | 第一个值 | 第二个值 | 容易踩的点 |
| --- | --- | --- | --- |
| 切片/数组 | 下标 `int` | 元素副本 | 改第二个值不动原切片，要改就用下标 |
| map | 键 | 值副本 | 遍历顺序随机，两次运行次序可以不同 |
| string | 字节下标 `int` | `rune` | 按 rune 解码，多字节字符下标按字节跨度跳 |
| 通道 | 元素 | 无第二个值 | 通道关闭且数据取完后循环自动退出 |

- 迭代得到的是元素副本，`for _, s := range scores { s = 100 }` 不会改原切片；要修改就用下标 `scores[i]`。只要一个返回值时，切片得到的是下标、map 得到的是键；不要的值用 `_` 丢弃。
- Go 1.22 起，循环变量每轮迭代都是新变量。在此之前，`go func() { fmt.Println(v) }()` 这类闭包捕获会看到同一个变量的最终值——这是旧代码里最经典的并发 bug 之一（第 4、8 章还会遇到）。

副本语义对照着看更直观：

```go
// ❌ 反例：改的是副本，循环结束后原切片原封不动
for _, s := range scores {
	s = 100
}

// ✅ 正确：通过下标改到原切片
for i := range scores {
	scores[i] = 100
}
```

## 3.4 逗号 ok：一次取值，一次判断"有没有"

Go 里三类操作都用 `v, ok := ...` 的双值形式表达"结果 + 是否成功"：

```go
// 教学变体：逗号 ok 的三种场景。
count, ok := counts["go"]  // map 查询：ok 表示键是否存在
v, ok := <-ch              // 通道接收：ok 为 false 表示通道已关闭且数据取完
s, ok := x.(string)        // 类型断言：ok 表示断言是否成立（第 7 章详述）
_, _, _ = count, v, s
_ = ok
```

统一的心智模型：**单值形式在失败时给零值或 panic，双值形式把失败变成可检查的布尔**。三种场景的单值行为并不一样，值得分开记：

| 场景 | 单值形式失败时 | 双值形式 `ok` 的含义 | 什么时候可以用单值 |
| --- | --- | --- | --- |
| map 查询 | 返回值类型的零值 | 键是否存在 | "缺失"和"零值"在业务里等价时 |
| 通道接收 | 通道关闭且取完后立即得到零值，无法区分 | false 表示通道已关闭且数据取完 | 用 for-range 接收时（退出即关闭） |
| 类型断言 | panic | 断言是否成立 | 几乎不用，除非类型已由上下文确定 |

map 查询用单值还是双值，取决于"缺失"和"零值"在你的业务里是不是同一件事：

```go
// ❌ 反例：零值歧义——"go 出现 0 次"和"go 不存在"无法区分
if counts["go"] == 0 {
	// 到底是没统计过这个词，还是计数真的是 0？
}

// ✅ 正确：双值接收，count 和 ok 的作用域限在 if 内，用完即销毁
if count, ok := counts["go"]; ok {
	fmt.Println("go 出现了", count, "次")
}
```

`if v, ok := ...; ok { }` 把取值和判断压在同一行，变量还不泄漏到 if 之外——这是 Go 代码里出现频率最高的惯用法之一。

## 踩坑清单

### 边界判断放到最后，101 被归为 excellent

```go
func labelScore(score int) string {
	if score >= 90 {
		return "excellent"
	}
	if score >= 60 {
		return "pass"
	}
	if score < 0 || score > 100 {
		// 反例：非法值检查太晚了，阅读时要先扫完整个正常路径。
		return "invalid"
	}
	return "retry"
}
```

现象：错误路径和正常路径混在一起，在独立副本中用这版实现运行已有 TestLabelScore，会在 101 处失败。原因：if 链按从上到下顺序匹配，101 在第一个 `>= 90` 分支就被"正常"地收走了，根本轮不到非法值检查。修复：把非法范围判断移回函数开头，修复后 -1、59、60、90、101 的已有断言通过。

### 用嵌套代替 continue，累加动作被包进 if 里

```go
func sumEven(limit int) int {
	total := 0
	for n := 0; n <= limit; n++ {
		if n%2 == 0 {
			// 反例：逻辑能跑，但真正重要的累加动作被包进了嵌套里。
			total += n
		}
	}
	return total
}
```

现象：结果虽对，主路径却往右缩；若误用 `break`，limit=10 时只能加第一轮，得到 0 而不是 30。原因：嵌套把"过滤条件"和"主路径动作"揉在一起，读代码时要在脑子里维护缩进层级，`continue`/`break` 一字之差就改变语义。修复：`continue` 的价值就是把"无关情况"尽早踢走，让主路径往左靠——对 limit 为 -1、0、1、10 应分别得到 0、0、0、30。

### 把带操作数的 switch 写成区间判断

```go
switch score {
case >= 90: // 反例：编译错误，>= 90 不是可以与 score 比较相等的值
	return "excellent"
}
```

现象：编译期报 invalid operation 之类的类型错误。原因：带操作数的 `switch score` 做的是相等比较，每个 case 必须是和 score 同类型的值，不能放布尔表达式。解法：区间/条件判断改用无表达式 switch——`switch { case score >= 90: ... }`，或者直接保留 if 链。

### 嵌套循环里的 break 只退出内层

```go
for _, row := range table {
	for _, cell := range row {
		if cell == target {
			break // 反例：只退出内层循环，外层继续扫剩下的行
		}
	}
}
```

现象：明明已经找到目标，程序却还在处理后续的行，数据量大时能从日志里看到多余的扫描。原因：`break` 只作用于直接包住它的那一层循环，不会级联向外。解法：给外层循环加标签（`rows:` 写在 for 前，然后 `break rows`）；更符合早返回风格的做法是把内层抽成函数，找到就 `return`，一次退干净。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 3.1 增加满分分支 | 在 `labelScore` 的非法范围判断之后加满分分支，同步修改测试；想清楚分支该放 `>= 90` 之前还是之后——若把满分写成 `score >= 100`，101 会先撞上它 | 100 返回 perfect；-1 和 101 仍返回 invalid；99、101、-1 的断言保留 | `control-flow/main.go`、`main_test.go` |
| 3.2 用 switch 实现同样分类 | 新增 `labelScoreSwitch`，用无表达式 switch 依次处理 invalid、perfect、excellent、pass 和 retry | 与 if 版本在 -1、0、59、60、89、90、99、100、101 上逐一比较结果相同 | `control-flow/main.go`、`main_test.go` |

完成后运行：

```bash
go -C labs test ./part1-golang/chapter03/control-flow/... -count=1
```

已有测试通过只说明已覆盖的用例符合预期；更多练习（计算奇数和 sumOdd）见各 lab 的 README。

## 小结

- 守卫分支先排除非法输入，后续代码才能在更小、更可靠的状态空间里工作。
- `continue` 跳过当前轮次，`break` 退出当前循环；嵌套循环需要标签或函数边界才能一次退出多层。
- `for range` 统一遍历集合，但不同集合对顺序和迭代值的保证不同，测试不能依赖未承诺的顺序。
- map 的逗号 ok 与类型断言的双值形式都在表达“取值和存在性是两个问题”。

## 链接

- [Go 规范：语句](https://go.dev/ref/spec#Statements)：核对 if、switch、for、break 和 continue 的行为。
- The Way to Go 中译补充示例：[if-else 结构](https://learnku.com/docs/the-way-to-go/if-else-structure/3592)、[switch 结构](https://learnku.com/docs/the-way-to-go/switch-structure/3594)、[for 结构](https://learnku.com/docs/the-way-to-go/for-structure/3595)、[break 和 continue](https://learnku.com/docs/the-way-to-go/break-and-continue/3596)。

[返回全书目录](../README.md) · [上一篇：数据类型](chapter02.md) · [下一篇：函数](chapter04.md)
