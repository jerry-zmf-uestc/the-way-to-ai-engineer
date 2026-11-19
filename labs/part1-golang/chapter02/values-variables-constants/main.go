package main

import "fmt"

const language = "Go"

func summary(version int, stable bool) string {
	status := "experimental"
	if stable {
		status = "stable"
	}
	return fmt.Sprintf("%s example v%d is %s", language, version, status)
}

func main() {
	version := 1
	fmt.Println(summary(version, true))
}
