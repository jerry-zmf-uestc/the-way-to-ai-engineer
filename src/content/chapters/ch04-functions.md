---
pageId: "ch04-functions"
title: "第 4 章：函数"
part: "Part 1 · Go 基础"
unitNumber: 4
order: 8
description: "签名与值传递、多返回值、递归、函数值、闭包、可变参数与 defer。"
published: true
source: "book/part1-golang/chapter04.md"
labs: ["labs/part1-golang/chapter04/defer-panic","labs/part1-golang/chapter04/function-values"]
capabilities: ["值传递","闭包","defer"]
---
Go 的函数设计极度克制：没有默认参数、没有重载、只有值传递，限制换来的是每行代码的行为都能本地推断。

## 4.1 函数声明

**函数签名 = 参数类型 + 返回值类型**，参数名不参与。没有默认参数，不能按名传参。

以下四种等价签名的声明方式。

```go
func add(x int, y int) int   { return x + y }

// 命名返回值（named return） + 裸 return（naked return）
func sub(x, y int) (z int)   { 
  z = x - y
  return  // 裸 return，相当于 return z
} 

// _ 强调未使用的参数
func first(x int, _ int) int { return x }    

func zero(int, int) int      { return 0 }
```

- **Go 只有值传递**：实参永远拷贝。第 2 章已经看到切片共享底层数组——传参后能"改到原数据"，是因为拷贝的头部结构里含指针，不是引用传递。
- 命名返回值让 `return` 可以裸写，但会掩盖实际返回的值——超过一屏的函数别用。

## 4.2 多返回值

Go 的标志性特性。最常见的用途是"结果 + 是否成功"：

```go
// 多返回值：结果 + error（第 5 章展开）
func div(a, b float64) (float64, error) { 
  if b == 0 {
    return 0, fmt.Errorf("division by zero")
  }
  return a / b, nil
}

// 多返回值：结果 + ok（第 2 章的逗号 ok）
func lookup(m map[string]int, key string) (int, bool) { 
  v, ok := m[key]
  return v, ok
}
```

调用处必须显式接收全部返回值，不用的赋给 `_`。一次调用可以整体作为另一个调用的参数（`log.Println(div(4, 2))`），调试时很方便。

## 4.3 递归

函数可以直接或间接调用自身。**多数语言的固定大小栈怕深递归；Go 的栈按需增长**，一般无需担心递归深度：

```go
// 摘录自 function-values/main.go。
func factorial(n int) int {
	if n < 0 {
		return -1 // 边界约定：非法输入返回 -1
	}
	if n == 0 {
		return 1 // 递归出口
	}
	return n * factorial(n-1)
}
```

递归的两要素：**出口条件**和**向出口收敛的子问题**。缺了任何一个就是栈耗尽。

## 4.4 函数值

**函数是第一类值**：有类型（签名），能赋值、传参、作返回值。

```go
// 摘录自 function-values/main.go。
func applyAll(f func(int) int, nums []int) []int {
	out := make([]int, len(nums))
	for i, n := range nums {
		out[i] = f(n) // "做什么"由调用方传入，遍历逻辑复用
	}
	return out
}
```

- 函数值的零值是 nil，**调用 nil 函数值会 panic**；函数值之间不可比较，也不能作 map 的键。
- 函数值参数化行为是 Go 的标准手法：`strings.Map`、HTTP 中间件（第 10 章）、`sort.Slice` 都是。

## 4.5 匿名函数与闭包

函数字面量（`func` 后无名字）是表达式，可在任意位置定义函数值，并且**捕获外层变量**——这就是闭包：

```go
// 摘录自 function-values/main.go。
func squares() func() int {
	var x int
	return func() int { x++; return x * x } // 捕获并更新外层 x
}

sq := squares()
fmt.Println(sq(), sq(), sq()) // "1 4 9"：x 的生命周期由闭包延续
```

**闭包捕获的是变量本身，不是某一时刻的值。** 循环里生成闭包曾因此集体踩坑（所有闭包共享同一个循环变量的最终值）；Go 1.22 起循环变量每轮新建，语言层面已修复，读旧代码时仍要知道这段历史。

## 4.6 可变参数

最后一个参数类型前加 `...`，函数接收任意多个实参：

```go
// 摘录自 function-values/main.go。
func sum(vals ...int) int { // 函数体内 vals 是 []int
	total := 0
	for _, v := range vals {
		total += v
	}
	return total
}

sum(1, 2, 3)
sum([]int{1, 2, 3}...) // 已有切片用 ... 展开
```

`func(...int)` 与 `func([]int)` 是不同类型，不能互换。

## 4.7 defer：延迟执行

defer ≈ 其他语言的 try/finally，但写在哪一行都行。**释放资源的 defer，紧跟获取资源的语句。**

defer 核心三条语义：

1. 多条 defer **后进先出**（LIFO）。
2. 参数在 defer 语句处**立即求值**，推迟的只是调用。
3. deferred 函数在 return 给返回值赋值**之后**执行，配合命名返回值可以观察甚至修改返回值。

```go
// defer 释放锁 + trace 计时。
func lookup(key string) int {
	mu.Lock()
	defer mu.Unlock() // 紧随 Lock，任何返回路径都会解锁
	return m[key]
}

func trace(msg string) func() {
	start := time.Now()
	log.Printf("enter %s", msg)
	return func() { log.Printf("exit %s (%s)", msg, time.Since(start)) }
}

func bigSlowOperation() {
	defer trace("bigSlowOperation")() // 双括号：trace 立即执行，返回的函数被 defer
	// ...
}
```

[defer-panic](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/tree/main/labs/part1-golang/chapter04/defer-panic) 示例把 LIFO 顺序、参数求值时机做成了可断言的函数：

```bash
go -C labs run ./part1-golang/chapter04/defer-panic
```

defer 在错误路径和 panic 展开中同样执行——那是它真正的舞台，下一章展开。

## 踩坑清单

### 调用 nil 函数值

```go
var f func() int
f() // 反例：panic: call of nil function
```

现象：运行期直接 panic。修复：函数值作可选回调时，调用前判 `if f != nil`。

### 循环里 defer 资源释放

```go
for _, path := range files {
	f, _ := os.Open(path)
	defer f.Close() // 反例：全部堆到函数结束才执行，文件描述符先耗尽
	// ...
}
```

现象：处理大量文件时 "too many open files"。修复：把循环体抽成独立函数，在函数内 defer。

### 以为闭包捕获的是"当时的值"

```go
for _, v := range []int{1, 2, 3} {
	funcs = append(funcs, func() int { return v })
}
```

Go 1.22 之前三个闭包都返回 3；**Go 1.22 起每轮迭代是新的 v**，返回 1、2、3。维护旧代码时在循环体内加 `v := v`。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 4.1 观察闭包的独立状态 | `a := squares()` 调三次、`b := squares()` 调一次 | a 的第三次返回 9，b 的第一次返回 1；注释写清两个闭包各自捕获的变量 | `function-values/main_test.go` |
| 4.2 预测 defer 执行顺序 | 先写下 deferOrder 三条 defer 的预期输出顺序，再交换其中两条对比 | 断言交换前后返回切片的具体内容，不只断言长度 | `defer-panic/main.go`、`main_test.go` |

完成后运行：

```bash
go -C labs test ./part1-golang/chapter04/... -count=1
```

更多练习（applyAll 函数值传参、factorial 递归边界、deferEval 求值时机）见各 lab 的 README。

## 链接

- [Go 程序设计语言（gopl）第 5 章：函数](https://golang-china.github.io/gopl-zh/ch5/ch5.html)：本章结构的主要参考，含 findLinks、topoSort 完整示例。
- [Effective Go：Functions](https://go.dev/doc/effective_go#functions)：多返回值、命名返回值与 defer 的官方说明。
- [Go 规范：函数声明](https://go.dev/ref/spec#Function_declarations)：签名与调用规则的准确定义。
