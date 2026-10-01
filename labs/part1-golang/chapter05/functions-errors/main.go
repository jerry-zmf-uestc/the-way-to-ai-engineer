package main

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	errNoInput      = errors.New("need at least one integer")
	errEmptyNumbers = errors.New("empty numbers")
)

func parseInts(args []string) ([]int, error) {
	if len(args) == 0 {
		return nil, errNoInput
	}

	nums := make([]int, 0, len(args))
	for _, arg := range args {
		n, err := strconv.Atoi(arg)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", arg, err)
		}
		nums = append(nums, n)
	}
	return nums, nil
}

func sumAvg(nums []int) (int, float64, error) {
	if len(nums) == 0 {
		return 0, 0, errEmptyNumbers
	}

	sum := 0
	for _, n := range nums {
		sum += n
	}
	return sum, float64(sum) / float64(len(nums)), nil
}

func main() {
	nums, err := parseInts([]string{})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	sum, avg, err := sumAvg(nums)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("sum=%d avg=%.2f\n", sum, avg)
}
