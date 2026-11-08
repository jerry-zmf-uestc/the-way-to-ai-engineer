package main

import "testing"

func TestBacklogBuildsAndDrains(t *testing.T) {
	got := exercise()
	depth := []int{1, 2, 3, 2, 1, 0}
	oldest := []int{0, 0, 1, 1, 2, 0}
	wait := []int{0, 1, 1, 2, 2, 3}
	if len(got) != len(depth) {
		t.Fatalf("got %d samples", len(got))
	}
	total := 0
	for tick, s := range got {
		total += len(s.arrivals) - len(s.completed)
		if s.tick != tick || s.depth != depth[tick] || s.depth != total || s.oldest != oldest[tick] ||
			len(s.completed) != 1 || s.completed[0] != (completion{tick + 1, wait[tick]}) {
			t.Fatalf("tick %d: unexpected sample %+v", tick, s)
		}
	}
}
