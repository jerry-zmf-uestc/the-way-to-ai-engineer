package main

import (
	"fmt"
	"sync"
	"time"
)

type result struct {
	source string
	value  string
}

func startWorker(source, value string, delay time.Duration) <-chan result {
	ch := make(chan result, 1)
	go func() {
		time.Sleep(delay)
		ch <- result{source: source, value: value}
		close(ch)
	}()
	return ch
}

func firstResult(primary, fallback <-chan result, timeout time.Duration) (result, bool) {
	select {
	case got := <-primary:
		return got, true
	case got := <-fallback:
		return got, true
	case <-time.After(timeout):
		return result{}, false
	}
}

func fanIn(inputs ...<-chan result) <-chan result {
	out := make(chan result)
	var wg sync.WaitGroup
	wg.Add(len(inputs))

	for _, input := range inputs {
		go func(ch <-chan result) {
			defer wg.Done()
			for got := range ch {
				out <- got
			}
		}(input)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func collectAll(inputs ...<-chan result) []result {
	results := make([]result, 0, len(inputs))
	for got := range fanIn(inputs...) {
		results = append(results, got)
	}
	return results
}

func main() {
	cache := startWorker("cache", "cached profile", 10*time.Millisecond)
	database := startWorker("database", "fresh profile", 30*time.Millisecond)

	got, ok := firstResult(cache, database, 50*time.Millisecond)
	if !ok {
		fmt.Println("timeout waiting for profile")
		return
	}

	fmt.Printf("%s: %s\n", got.source, got.value)
}
