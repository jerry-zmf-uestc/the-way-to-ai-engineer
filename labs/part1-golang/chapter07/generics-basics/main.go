package main

import "fmt"

type number interface {
	~int | ~int64 | ~float64
}

type set[T comparable] struct {
	items map[T]struct{}
}

func sum[T number](values []T) T {
	var total T
	for _, value := range values {
		total += value
	}
	return total
}

func mapSlice[T any, R any](values []T, convert func(T) R) []R {
	results := make([]R, 0, len(values))
	for _, value := range values {
		results = append(results, convert(value))
	}
	return results
}

func first[T any](values []T) (T, bool) {
	if len(values) == 0 {
		var zero T
		return zero, false
	}
	return values[0], true
}

func newSet[T comparable](values ...T) set[T] {
	s := set[T]{items: make(map[T]struct{}, len(values))}
	for _, value := range values {
		s.add(value)
	}
	return s
}

func (s set[T]) add(value T) {
	s.items[value] = struct{}{}
}

func (s set[T]) contains(value T) bool {
	_, ok := s.items[value]
	return ok
}

func (s set[T]) len() int {
	return len(s.items)
}

func main() {
	scores := []int{10, 20, 15}
	prices := []float64{12.5, 7.5}
	names := []string{"go", "generics"}

	lengths := mapSlice(names, func(name string) int {
		return len(name)
	})
	services := newSet("cache", "database", "cache")

	firstScore, ok := first(scores)
	if !ok {
		fmt.Println("no scores")
		return
	}

	fmt.Printf("score total: %d\n", sum(scores))
	fmt.Printf("price total: %.1f\n", sum(prices))
	fmt.Printf("name lengths: %v\n", lengths)
	fmt.Printf("first score: %d\n", firstScore)
	fmt.Printf("contains cache: %t, unique services: %d\n", services.contains("cache"), services.len())
}
