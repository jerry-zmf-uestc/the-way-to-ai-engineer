package main

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
)

func snapshotChunk(version, space, text string, vector []float64) Chunk {
	return Chunk{ID: "guide@" + version, Source: "guide", Version: version, Tenant: "reader", Text: text, Space: space, Vector: vector}
}

func TestSnapshotPublicationKeepsQueryGenerationAndSpace(t *testing.T) {
	ctx := context.Background()
	var publisher Publisher
	if _, err := publisher.Capture(); !errors.Is(err, ErrInvalid) {
		t.Fatal("empty publication accepted")
	}
	oldChunk := snapshotChunk("v1", "space-a", "timeout 30", []float64{1, 0})
	if err := publisher.Publish(ctx, 0, "space-a", 2, []Chunk{oldChunk}); err != nil {
		t.Fatal(err)
	}
	old, _ := publisher.Capture()
	oldChunk.Vector[0] = 0
	newChunk := snapshotChunk("v2", "space-b", "timeout 45", []float64{0, 1})
	if err := publisher.Publish(ctx, 1, "space-b", 2, []Chunk{newChunk}); err != nil {
		t.Fatal(err)
	}
	current, _ := publisher.Capture()
	for _, view := range []*Snapshot{old, current} {
		generation, space, dimensions := view.Identity()
		query := []float64{1, 0}
		if generation == 2 {
			query = []float64{0, 1}
		}
		hits, err := view.Search(ctx, "reader", "timeout", space, query, 1, 1)
		if err != nil || dimensions != 2 || len(hits) != 1 {
			t.Fatal(hits, err)
		}
		want := "v1"
		if generation == 2 {
			want = "v2"
		}
		if hits[0].Chunk.Version != want {
			t.Fatal("mixed snapshot", hits)
		}
		hits[0].Chunk.Vector[0] = 999
		again, _ := view.Search(ctx, "reader", "timeout", space, query, 1, 1)
		if again[0].Chunk.Vector[0] == 999 {
			t.Fatal("returned vector mutated published index")
		}
	}
	if _, err := current.Search(ctx, "reader", "timeout", "space-a", []float64{1, 0}, 1, 1); !errors.Is(err, ErrInvalid) {
		t.Fatal("same dimensions bypassed space migration")
	}
}

func TestConcurrentPublisherRejectsStaleGeneration(t *testing.T) {
	var publisher Publisher
	chunks := []Chunk{snapshotChunk("v1", "s", "x", []float64{1})}
	if err := publisher.Publish(context.Background(), 0, "s", 1, chunks); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- publisher.Publish(context.Background(), 1, "s", 1, chunks)
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	success, stale := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrGeneration) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatal(success, stale)
	}
	view, _ := publisher.Capture()
	if generation, _, _ := view.Identity(); generation != 2 {
		t.Fatal(generation)
	}
}

func TestRejectedBuildAndCancellationPreserveOldSnapshot(t *testing.T) {
	var publisher Publisher
	chunks := []Chunk{snapshotChunk("v1", "s", "x", []float64{1})}
	publisher.Publish(context.Background(), 0, "s", 1, chunks)
	old, _ := publisher.Capture()
	bad := append([]Chunk(nil), chunks...)
	bad[0].Vector = []float64{math.NaN()}
	if err := publisher.Publish(context.Background(), 1, "s", 1, bad); err == nil {
		t.Fatal("bad build published")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := publisher.Publish(ctx, 1, "s", 1, chunks); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	current, _ := publisher.Capture()
	if current != old {
		t.Fatal("failed build replaced active snapshot")
	}
	publisher.current = &Snapshot{generation: math.MaxUint64, index: old.index}
	if err := publisher.Publish(context.Background(), math.MaxUint64, "s", 1, chunks); !errors.Is(err, ErrGeneration) {
		t.Fatal("generation wrapped", err)
	}
}
