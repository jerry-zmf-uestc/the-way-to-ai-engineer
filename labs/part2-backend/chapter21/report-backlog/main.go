package main

import "fmt"

type job struct{ id, enqueuedAt int }
type completion struct{ jobID, waited int }

type sample struct {
	tick, depth, oldest int
	arrivals            []job
	completed           []completion
}

// shortcut: virtual seconds and an in-memory queue make the workload repeatable.
func exercise() []sample {
	var queue []job
	var samples []sample
	nextID := 1
	for tick := range 6 {
		s := sample{tick: tick}
		if tick < 3 {
			for range 2 {
				arrival := job{nextID, tick}
				queue = append(queue, arrival)
				s.arrivals = append(s.arrivals, arrival)
				nextID++
			}
		}
		if len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			s.completed = append(s.completed, completion{current.id, tick - current.enqueuedAt})
		}
		s.depth = len(queue)
		if s.depth > 0 {
			s.oldest = tick - queue[0].enqueuedAt
		}
		samples = append(samples, s)
	}
	return samples
}

func main() {
	for _, s := range exercise() {
		for _, j := range s.arrivals {
			fmt.Printf("event=enqueued tick=%d job=export-%d\n", s.tick, j.id)
		}
		for _, c := range s.completed {
			fmt.Printf("event=completed tick=%d job=export-%d queue_wait_seconds=%d\n", s.tick, c.jobID, c.waited)
		}
		fmt.Printf("metric tick=%d arrivals=%d completed=%d depth=%d oldest_seconds=%d\n",
			s.tick, len(s.arrivals), len(s.completed), s.depth, s.oldest)
	}
}
