package main

import (
	"fmt"
	"strings"
)

// JoinTags concatenates tags with a comma separator.
//
// 反例（不要在循环中用 += 拼接 string，每次都会分配新字符串）：
//
//	out := ""
//	for _, t := range tags {
//	    out += t + ","
//	}
//
// 正例使用 strings.Builder，复用底层缓冲区。
func JoinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	var b strings.Builder
	// 预估容量，避免 Builder 多次扩容。
	b.Grow(len(tags) * 8)

	for i, t := range tags {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(t)
	}
	return b.String()
}

func main() {
	fmt.Println(JoinTags([]string{"go", "json", "uber"}))
}
