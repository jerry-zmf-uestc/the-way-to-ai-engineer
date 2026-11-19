package main

import "testing"

func TestLostAck(t *testing.T) {
	if got, want := exercise(false), (observation{2, 1, 2, 0}); got != want {
		t.Fatalf("fault: got %+v, want %+v", got, want)
	}
	if got, want := exercise(true), (observation{2, 1, 1, 0}); got != want {
		t.Fatalf("repair: got %+v, want %+v", got, want)
	}
}

func TestDistinctEventsProduceDistinctReports(t *testing.T) {
	store := reportStore{processed: make(map[string]bool)}
	store.commit(delivery{"event-1", "export-1"}, true)
	store.commit(delivery{"event-2", "export-2"}, true)
	if len(store.artifacts) != 2 {
		t.Fatalf("distinct events were collapsed: %v", store.artifacts)
	}
}
