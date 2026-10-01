package main

import (
	"slices"
	"testing"
	"time"
)

func readyResult(source, value string) <-chan result {
	ch := make(chan result, 1)
	ch <- result{source: source, value: value}
	close(ch)
	return ch
}

func TestFirstResult(t *testing.T) {
	t.Run("uses ready channel", func(t *testing.T) {
		got, ok := firstResult(readyResult("cache", "hit"), make(chan result), time.Second)
		if !ok {
			t.Fatal("firstResult() ok = false, want true")
		}
		if got.source != "cache" || got.value != "hit" {
			t.Fatalf("firstResult() = %+v, want cache hit", got)
		}
	})

	t.Run("times out", func(t *testing.T) {
		got, ok := firstResult(make(chan result), make(chan result), time.Millisecond)
		if ok {
			t.Fatalf("firstResult() = (%+v, true), want timeout", got)
		}
	})
}

func TestCollectAll(t *testing.T) {
	got := collectAll(
		readyResult("cache", "hit"),
		readyResult("database", "fresh"),
	)

	sources := make([]string, 0, len(got))
	for _, item := range got {
		sources = append(sources, item.source)
	}
	slices.Sort(sources)

	if !slices.Equal(sources, []string{"cache", "database"}) {
		t.Fatalf("collectAll() sources = %v, want [cache database]", sources)
	}
}
