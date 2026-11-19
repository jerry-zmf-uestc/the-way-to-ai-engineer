package main

import (
	"context"
	"errors"
	"math"
	"sync"
)

var ErrGeneration = errors.New("retrieval: stale index generation")

type Snapshot struct {
	generation uint64
	index      Index
}

func (s *Snapshot) Identity() (uint64, string, int) {
	return s.generation, s.index.Space, s.index.Dimensions
}

func (s *Snapshot) Search(ctx context.Context, tenant, query, space string, vector []float64, candidateK, topK int) ([]Hit, error) {
	return s.index.Search(ctx, tenant, query, space, vector, candidateK, topK)
}

type Publisher struct {
	mu      sync.RWMutex
	current *Snapshot
}

func (p *Publisher) Capture() (*Snapshot, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.current == nil {
		return nil, ErrInvalid
	}
	return p.current, nil
}

func (p *Publisher) Publish(ctx context.Context, expected uint64, space string, dimensions int, chunks []Chunk) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	index, err := NewIndex(space, dimensions, chunks)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	var generation uint64
	if p.current != nil {
		generation = p.current.generation
	}
	if generation != expected || generation == math.MaxUint64 {
		return ErrGeneration
	}
	p.current = &Snapshot{generation: generation + 1, index: index}
	return nil
}
