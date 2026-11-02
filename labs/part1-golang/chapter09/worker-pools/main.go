package main

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

var errInvalidWorkerCount = errors.New("worker count must be positive")

type job struct {
	id    int
	units int
}

type report struct {
	jobID   int
	worker  int
	quality int
}

type poolStats struct {
	mu        sync.Mutex
	started   int
	completed int
	total     int
}

type statsSnapshot struct {
	started   int
	completed int
	total     int
}

func (s *poolStats) markStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.started++
}

func (s *poolStats) markCompleted(score int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.completed++
	s.total += score
}

func (s *poolStats) snapshot() statsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return statsSnapshot{
		started:   s.started,
		completed: s.completed,
		total:     s.total,
	}
}

func scoreJob(j job) int {
	return j.units * j.units
}

func worker(id int, jobs <-chan job, reports chan<- report, stats *poolStats, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		stats.markStarted()
		score := scoreJob(j)
		stats.markCompleted(score)
		reports <- report{jobID: j.id, worker: id, quality: score}
	}
}

func runWorkerPool(workerCount int, items []job) ([]report, statsSnapshot, error) {
	if workerCount < 1 {
		return nil, statsSnapshot{}, errInvalidWorkerCount
	}

	jobs := make(chan job)
	reports := make(chan report, len(items))
	stats := &poolStats{}

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for id := 1; id <= workerCount; id++ {
		go worker(id, jobs, reports, stats, &wg)
	}

	for _, item := range items {
		jobs <- item
	}
	close(jobs)

	wg.Wait()
	close(reports)

	results := make([]report, 0, len(items))
	for item := range reports {
		results = append(results, item)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].jobID < results[j].jobID
	})

	return results, stats.snapshot(), nil
}

func main() {
	items := []job{
		{id: 1, units: 2},
		{id: 2, units: 4},
		{id: 3, units: 3},
	}

	reports, stats, err := runWorkerPool(2, items)
	if err != nil {
		fmt.Println("worker pool error:", err)
		return
	}

	for _, item := range reports {
		fmt.Printf("job %d handled by worker %d scored %d\n", item.jobID, item.worker, item.quality)
	}
	fmt.Printf("completed %d jobs, total quality %d\n", stats.completed, stats.total)
}
