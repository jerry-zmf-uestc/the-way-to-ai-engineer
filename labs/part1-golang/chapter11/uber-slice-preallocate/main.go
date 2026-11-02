package main

import "fmt"

// FilterEven returns even numbers from src using a pre-allocated slice.
//
// 反例（未预分配容量，append 会多次扩容）：
//
//	var out []int
//	for _, v := range src {
//	    if v%2 == 0 {
//	        out = append(out, v)
//	    }
//	}
//
// 正例：根据 src 的长度上限预分配容量。
func FilterEven(src []int) []int {
	out := make([]int, 0, len(src))
	for _, v := range src {
		if v%2 == 0 {
			out = append(out, v)
		}
	}
	return out
}

func main() {
	fmt.Println(FilterEven([]int{1, 2, 3, 4, 5, 6}))
}
