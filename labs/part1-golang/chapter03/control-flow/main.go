package main

import "fmt"

func labelScore(score int) string {
	if score < 0 || score > 100 {
		return "invalid"
	}
	if score >= 90 {
		return "excellent"
	}
	if score >= 60 {
		return "pass"
	}
	return "retry"
}

func sumEven(limit int) int {
	total := 0
	for n := 0; n <= limit; n++ {
		if n%2 != 0 {
			continue
		}
		total += n
	}
	return total
}

func main() {
	fmt.Println(labelScore(91))
	fmt.Println(sumEven(10))
}
