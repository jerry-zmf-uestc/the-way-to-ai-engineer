package main

import "fmt"

// applyAll 演示函数值作为参数：把函数 f 应用到 nums 的每个元素，
// 返回结果切片。f 的类型 func(int) int 与普通类型一样可以作为参数传递。
func applyAll(f func(int) int, nums []int) []int {
	result := make([]int, len(nums))
	for i, n := range nums {
		result[i] = f(n)
	}
	return result
}

// squares 返回一个闭包，捕获并更新外层局部变量 x。
// 函数返回后 x 仍然存活（逃逸到堆上），每次调用先自增再返回 x 的平方，
// 连续调用依次得到 1, 4, 9, 16。
func squares() func() int {
	x := 0
	return func() int {
		x++
		return x * x
	}
}

// sum 演示可变参数：调用方可以逐个传参（sum(1, 2, 3)），
// 也可以用 sum(nums...) 把切片展开传入，函数内部 vals 都是 []int 类型，两者完全等价。
func sum(vals ...int) int {
	total := 0
	for _, v := range vals {
		total += v
	}
	return total
}

// factorial 用递归计算 n 的阶乘。
// 边界约定：n == 0 时返回 1；n < 0 没有数学定义，约定返回 -1 表示非法输入。
func factorial(n int) int {
	if n < 0 {
		return -1
	}
	if n == 0 {
		return 1
	}
	return n * factorial(n-1)
}

func main() {
	nums := []int{1, 2, 3, 4}

	// 函数值作为参数：同一个 applyAll 传入不同的函数得到不同结果。
	double := func(n int) int { return n * 2 }
	square := func(n int) int { return n * n }
	fmt.Println("applyAll(加倍, nums) =", applyAll(double, nums))
	fmt.Println("applyAll(平方, nums) =", applyAll(square, nums))

	// 闭包捕获并更新外层变量。
	next := squares()
	fmt.Println("闭包连续调用:", next(), next(), next(), next())

	// 可变参数：逐个传参与切片展开等价。
	fmt.Println("sum(1, 2, 3) =", sum(1, 2, 3))
	fmt.Println("sum(nums...) =", sum(nums...))

	fmt.Println("factorial(5) =", factorial(5))
	fmt.Println("factorial(-1) =", factorial(-1))
}
