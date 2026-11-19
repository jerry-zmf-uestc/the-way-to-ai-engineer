package main

import (
	"reflect"
	"testing"
)

func TestStringStatsCountsBytesAndRunes(t *testing.T) {
	stats := Analyze("Go语言")

	if stats.Bytes != 8 {
		t.Fatalf("Bytes = %d, want 8", stats.Bytes)
	}
	if stats.Runes != 4 {
		t.Fatalf("Runes = %d, want 4", stats.Runes)
	}
}

func TestRuneSliceKeepsCharacters(t *testing.T) {
	got := FirstRunes("你好 Go", 2)
	want := []rune{'你', '好'}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FirstRunes() = %q, want %q", string(got), string(want))
	}
}

func TestNormalizeWordsUsesStringsHelpers(t *testing.T) {
	got := NormalizeWords(" go\tlearn  go ")
	want := "go,learn,go"
	if got != want {
		t.Fatalf("NormalizeWords() = %q, want %q", got, want)
	}
}

func TestReplaceTopic(t *testing.T) {
	got := ReplaceTopic("learn java, write java", "java", "go")
	want := "learn go, write go"
	if got != want {
		t.Fatalf("ReplaceTopic() = %q, want %q", got, want)
	}
}
