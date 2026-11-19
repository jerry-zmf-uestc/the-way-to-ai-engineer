package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type StringStats struct {
	Bytes int
	Runes int
}

func Analyze(s string) StringStats {
	return StringStats{
		Bytes: len(s),
		Runes: utf8.RuneCountInString(s),
	}
}

func FirstRunes(s string, n int) []rune {
	if n <= 0 {
		return nil
	}

	result := make([]rune, 0, n)
	for _, r := range s {
		result = append(result, r)
		if len(result) == n {
			break
		}
	}
	return result
}

func NormalizeWords(s string) string {
	return strings.Join(strings.Fields(s), ",")
}

func ReplaceTopic(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

func main() {
	stats := Analyze("Go语言")
	fmt.Printf("bytes=%d runes=%d\n", stats.Bytes, stats.Runes)
	fmt.Println(string(FirstRunes("你好 Go", 2)))
	fmt.Println(NormalizeWords(" go\tlearn  go "))
	fmt.Println(ReplaceTopic("learn java, write java", "java", "go"))
}
