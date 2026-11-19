package main

import (
	"fmt"
	"strconv"
)

// deferOrder 依次 defer 三次 append 操作并返回实际执行顺序，
// 证明 defer 语句按后进先出（LIFO）的顺序执行：后注册的先运行。
// 注意必须使用命名返回值，deferred 函数对 order 的修改才能被调用方看到。
func deferOrder() (order []int) {
	for i := 1; i <= 3; i++ {
		defer func(v int) {
			order = append(order, v)
		}(i)
	}
	return
}

// deferEval 对比 defer 的两种参数求值时机：
// immediate 使用「defer f(x)」形式，参数在 defer 语句执行时立即求值，捕获的是注册时刻的 "start"；
// late 使用「defer func() { ... }()」闭包形式，变量在 deferred 函数真正运行时才读取，读到的是最终的 "end"。
func deferEval() (immediate, late string) {
	name := "start"
	defer func(s string) { immediate = s }(name)
	defer func() { late = name }()
	name = "end"
	return
}

// trace 演示「defer trace("op")()」模式：defer 语句执行时 trace 本身立即被调用
// （打印 enter 并捕获 op），只有它返回的函数才被推迟到外围函数返回时执行，
// 常用于「进入时记录、退出时完成」的配对日志。
func trace(op string) func() {
	fmt.Printf("enter %s\n", op)
	return func() {
		fmt.Printf("exit %s\n", op)
	}
}

// parseError 是解析失败的专用类型，用于标识「预期的」panic，
// 借鉴 gopl soleTitle 的「有选择的 recover」：只有这种类型的 panic 会被转为 error，
// 未预期的 panic（如运行时除零）会原样继续抛出，不被 recover 吞掉。
type parseError struct {
	reason string
}

func (e *parseError) Error() string { return e.reason }

// convertPanic 是公用的选择性恢复逻辑，作为 defer 调用：
// 把预期的 *parseError 写入命名返回值 err，其余 panic 用 panic(p) 原样重新抛出。
func convertPanic(err *error) {
	switch p := recover().(type) {
	case nil:
		// 没有 panic，正常返回。
	case *parseError:
		*err = p
	default:
		panic(p)
	}
}

// parsePositive 解析正整数。内部解析逻辑用 panic 报告「非数字 / 非正数」两类内部错误，
// 在最外层通过命名返回值与 defer/recover 把预期的 panic 转为 error 返回给调用方。
func parsePositive(input string) (n int, err error) {
	defer convertPanic(&err)
	return mustParsePositive(input), nil
}

// mustParsePositive 是内部解析逻辑：失败时直接 panic，错误处理交给外层 recover。
func mustParsePositive(input string) int {
	n, err := strconv.Atoi(input)
	if err != nil {
		panic(&parseError{reason: "not a number: " + input})
	}
	if n <= 0 {
		panic(&parseError{reason: "not positive: " + input})
	}
	return n
}

// ratio 与 parsePositive 使用相同的选择性 recover 模式：
// count 为负数是预期错误，转为 error 返回；
// count 为 0 时触发运行时除零 panic，属于未预期错误，会穿过 recover 继续向上传播。
func ratio(total, count int) (result int, err error) {
	defer convertPanic(&err)
	if count < 0 {
		panic(&parseError{reason: "negative count"})
	}
	return total / count, nil
}

// closureValues 演示 Go 1.22 起的循环变量语义：for 循环每轮迭代都会新建循环变量，
// 三个闭包各自捕获当轮的 i，因此依次调用返回 [0 1 2]；
// 在 Go 1.21 及更早版本中所有闭包共享同一个变量，循环结束后调用会得到 [3 3 3]。
func closureValues() []int {
	fns := make([]func() int, 0, 3)
	for i := 0; i < 3; i++ {
		fns = append(fns, func() int { return i })
	}
	values := make([]int, 0, len(fns))
	for _, fn := range fns {
		values = append(values, fn())
	}
	return values
}

func main() {
	defer trace("demo")() // trace 立即执行打印 enter，返回的函数被 defer 到 main 结束时执行

	fmt.Println("defer LIFO 顺序:", deferOrder())

	immediate, late := deferEval()
	fmt.Printf("defer 求值时机: immediate=%q late=%q\n", immediate, late)

	if n, err := parsePositive("42"); err == nil {
		fmt.Println("parsePositive(\"42\") =", n)
	}
	if _, err := parsePositive("-3"); err != nil {
		fmt.Println("parsePositive(\"-3\") err:", err)
	}
	if _, err := parsePositive("abc"); err != nil {
		fmt.Println("parsePositive(\"abc\") err:", err)
	}

	if result, err := ratio(10, 2); err == nil {
		fmt.Println("ratio(10, 2) =", result)
	}

	fmt.Println("闭包捕获循环变量:", closureValues())
}
