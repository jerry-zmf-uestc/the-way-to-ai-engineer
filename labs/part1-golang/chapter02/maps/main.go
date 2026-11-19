package main

import "fmt"

func countWords(words []string) map[string]int {
	counts := make(map[string]int)
	for _, word := range words {
		if word == "" {
			continue
		}
		counts[word]++
	}
	return counts
}

func lookup(counts map[string]int, word string) (int, bool) {
	count, ok := counts[word]
	return count, ok
}

func uniqueWords(words []string) []string {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(words))
	for _, word := range words {
		if word == "" || seen[word] {
			continue
		}
		seen[word] = true
		unique = append(unique, word)
	}
	return unique
}

func main() {
	words := []string{"go", "learn", "go", ""}
	counts := countWords(words)

	fmt.Println(counts)
	fmt.Println(lookup(counts, "go"))
	fmt.Println(uniqueWords(words))
}
