package main

import "fmt"

func appendCopy(base []int, values ...int) []int {
	out := make([]int, len(base), len(base)+len(values))
	copy(out, base)
	return append(out, values...)
}

func window(values []int, start, end int) []int {
	if start < 0 || end > len(values) || start > end {
		return nil
	}
	return values[start:end]
}

func main() {
	values := []int{1, 2, 3}
	extended := appendCopy(values, 4, 5)
	middle := window(values, 1, 3)

	fmt.Println("base:", values)
	fmt.Println("extended:", extended)
	fmt.Println("middle:", middle)
}
