package main

import "testing"

func TestJoinTagsEmpty(t *testing.T) {
	if got := JoinTags(nil); got != "" {
		t.Fatalf("JoinTags(nil) = %q, want empty", got)
	}
}

func TestJoinTagsBasic(t *testing.T) {
	got := JoinTags([]string{"go", "json", "uber"})
	want := "go,json,uber"
	if got != want {
		t.Fatalf("JoinTags() = %q, want %q", got, want)
	}
}

func BenchmarkJoinTags(b *testing.B) {
	tags := []string{"go", "json", "uber", "style", "guide"}
	for i := 0; i < b.N; i++ {
		_ = JoinTags(tags)
	}
}
