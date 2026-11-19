package main

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestDeferOrder(t *testing.T) {
	got := deferOrder()
	want := []int{3, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("deferOrder() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("deferOrder() = %v, want LIFO order %v", got, want)
		}
	}
}

func TestDeferEval(t *testing.T) {
	immediate, late := deferEval()
	if immediate != "start" {
		t.Fatalf("deferEval() immediate = %q, want %q（参数在 defer 注册时立即求值）", immediate, "start")
	}
	if late != "end" {
		t.Fatalf("deferEval() late = %q, want %q（闭包在执行时才读取变量）", late, "end")
	}
}

func TestParsePositive(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr string
	}{
		{name: "valid positive", input: "42", want: 42, wantErr: ""},
		{name: "zero", input: "0", want: 0, wantErr: "not positive"},
		{name: "negative", input: "-3", want: 0, wantErr: "not positive"},
		{name: "not a number", input: "abc", want: 0, wantErr: "not a number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := parsePositive(tt.input)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("parsePositive(%q) error = %v, want nil", tt.input, err)
				}
				if n != tt.want {
					t.Fatalf("parsePositive(%q) = %d, want %d", tt.input, n, tt.want)
				}
				return
			}
			var pe *parseError
			if !errors.As(err, &pe) {
				t.Fatalf("parsePositive(%q) error = %v, want *parseError", tt.input, err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parsePositive(%q) error = %q, want containing %q", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestRatioNegativeCountReturnsError(t *testing.T) {
	_, err := ratio(10, -1)
	var pe *parseError
	if !errors.As(err, &pe) {
		t.Fatalf("ratio(10, -1) error = %v, want *parseError", err)
	}
}

func TestRatioUnexpectedPanicPropagates(t *testing.T) {
	// count 为 0 触发运行时除零 panic，不属于预期的 *parseError，
	// 选择性 recover 必须原样重新抛出，不能吞掉。
	defer func() {
		p := recover()
		if p == nil {
			t.Fatal("ratio(1, 0) did not panic, want runtime divide-by-zero panic to propagate")
		}
		if _, ok := p.(runtime.Error); !ok {
			t.Fatalf("ratio(1, 0) panic = %T(%v), want runtime.Error", p, p)
		}
	}()
	_, _ = ratio(1, 0)
}

func TestClosureValues(t *testing.T) {
	got := closureValues()
	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("closureValues() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("closureValues() = %v, want %v（Go 1.22 起每轮迭代新建循环变量）", got, want)
		}
	}
}
