package main

import "fmt"

type delivery struct{ eventID, jobID string }

// broker models one unacked message and its redelivery after a channel loss.
// shortcut: single message in memory; use a real broker to study durability.
type broker struct {
	pending    *delivery
	deliveries int
	acks       int
}

func (b *broker) deliver() (delivery, bool) {
	if b.pending == nil {
		return delivery{}, false
	}
	b.deliveries++
	return *b.pending, true
}

func (b *broker) ack() {
	b.pending = nil
	b.acks++
}

type reportStore struct {
	processed map[string]bool
	artifacts []string
}

func (s *reportStore) commit(d delivery, deduplicate bool) {
	if deduplicate && s.processed[d.eventID] {
		return
	}
	// One simulated commit: record the artifact and event together.
	s.artifacts = append(s.artifacts, d.jobID)
	s.processed[d.eventID] = true
}

type observation struct{ deliveries, acks, artifacts, pending int }

func exercise(fixed bool) observation {
	b := broker{pending: &delivery{"event-1", "export-1"}}
	store := reportStore{processed: make(map[string]bool)}
	first, _ := b.deliver()
	store.commit(first, fixed)

	// Drop the first ACK after commit. The broker still owns the message.
	again, _ := b.deliver()
	store.commit(again, fixed)
	b.ack()

	pending := 0
	if _, ok := b.deliver(); ok {
		pending = 1
	}
	return observation{b.deliveries, b.acks, len(store.artifacts), pending}
}

func main() {
	for _, fixed := range []bool{false, true} {
		m := exercise(fixed)
		fmt.Printf("fixed=%t deliveries=%d acks=%d artifacts=%d pending=%d\n",
			fixed, m.deliveries, m.acks, m.artifacts, m.pending)
	}
}
