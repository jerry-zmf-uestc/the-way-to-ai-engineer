package main

import (
	"reflect"
	"testing"
)

func TestFilterEvenBasic(t *testing.T) {
	got := FilterEven([]int{1, 2, 3, 4, 5, 6})
	want := []int{2, 4, 6}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterEven() = %v, want %v", got, want)
	}
}

func TestFilterEvenEmpty(t *testing.T) {
	if got := FilterEven(nil); len(got) != 0 {
		t.Fatalf("FilterEven(nil) = %v, want empty slice", got)
	}
}

func TestFilterEvenPreallocates(t *testing.T) {
	src := []int{1, 3, 5}
	got := FilterEven(src)
	// 即使全部被过滤掉，底层容量也至少预留到 len(src)。
	if cap(got) < len(src) {
		t.Fatalf("FilterEven() cap = %d, want >= %d", cap(got), len(src))
	}
}
