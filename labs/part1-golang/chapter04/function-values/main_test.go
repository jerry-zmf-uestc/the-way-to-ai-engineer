package main

import "testing"

func TestApplyAllDouble(t *testing.T) {
	double := func(n int) int { return n * 2 }
	got := applyAll(double, []int{1, 2, 3, 4})
	want := []int{2, 4, 6, 8}
	if len(got) != len(want) {
		t.Fatalf("applyAll(加倍, [1 2 3 4]) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("applyAll(加倍, [1 2 3 4]) = %v, want %v", got, want)
		}
	}
}

func TestApplyAllSquare(t *testing.T) {
	square := func(n int) int { return n * n }
	got := applyAll(square, []int{1, 2, 3, 4})
	want := []int{1, 4, 9, 16}
	if len(got) != len(want) {
		t.Fatalf("applyAll(平方, [1 2 3 4]) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("applyAll(平方, [1 2 3 4]) = %v, want %v", got, want)
		}
	}
}

func TestSquaresClosure(t *testing.T) {
	// 闭包捕获并更新外层变量，连续四次调用应依次返回 1, 4, 9, 16。
	next := squares()
	want := []int{1, 4, 9, 16}
	for i, w := range want {
		if got := next(); got != w {
			t.Fatalf("第 %d 次调用 squares 闭包 = %d, want %d", i+1, got, w)
		}
	}
}

func TestSumEmpty(t *testing.T) {
	if got := sum(); got != 0 {
		t.Fatalf("sum() = %d, want 0", got)
	}
}

func TestSumSpread(t *testing.T) {
	// sum(nums...) 切片展开与逐个传参完全等价。
	nums := []int{1, 2, 3}
	if got := sum(nums...); got != 6 {
		t.Fatalf("sum(nums...) = %d, want 6", got)
	}
	if sum(nums...) != sum(1, 2, 3) {
		t.Fatalf("sum(nums...) != sum(1, 2, 3)，切片展开应与逐个传参等价")
	}
}

func TestFactorial(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int
	}{
		{name: "zero", n: 0, want: 1},
		{name: "five", n: 5, want: 120},
		{name: "negative", n: -1, want: -1}, // 边界约定：n < 0 返回 -1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := factorial(tt.n); got != tt.want {
				t.Fatalf("factorial(%d) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
}
