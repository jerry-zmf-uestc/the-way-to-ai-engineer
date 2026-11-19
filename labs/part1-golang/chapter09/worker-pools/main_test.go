package main

import (
	"errors"
	"testing"
)

func TestRunWorkerPool(t *testing.T) {
	items := []job{
		{id: 1, units: 2},
		{id: 2, units: 4},
		{id: 3, units: 3},
	}

	reports, stats, err := runWorkerPool(2, items)
	if err != nil {
		t.Fatalf("runWorkerPool() error = %v, want nil", err)
	}

	wantScores := []int{4, 16, 9}
	if len(reports) != len(wantScores) {
		t.Fatalf("runWorkerPool() reports = %d, want %d", len(reports), len(wantScores))
	}

	for i, got := range reports {
		if got.jobID != i+1 {
			t.Fatalf("reports[%d].jobID = %d, want %d", i, got.jobID, i+1)
		}
		if got.quality != wantScores[i] {
			t.Fatalf("reports[%d].quality = %d, want %d", i, got.quality, wantScores[i])
		}
		if got.worker < 1 || got.worker > 2 {
			t.Fatalf("reports[%d].worker = %d, want between 1 and 2", i, got.worker)
		}
	}

	if stats.started != len(items) || stats.completed != len(items) {
		t.Fatalf("stats = %+v, want started/completed %d", stats, len(items))
	}
	if stats.total != 29 {
		t.Fatalf("stats.total = %d, want 29", stats.total)
	}
}

func TestRunWorkerPoolRequiresWorker(t *testing.T) {
	_, _, err := runWorkerPool(0, []job{{id: 1, units: 1}})
	if !errors.Is(err, errInvalidWorkerCount) {
		t.Fatalf("runWorkerPool() error = %v, want worker count validation", err)
	}
}

func TestScoreJob(t *testing.T) {
	tests := []struct {
		name string
		job  job
		want int
	}{
		{name: "small job", job: job{id: 1, units: 3}, want: 9},
		{name: "empty job", job: job{id: 2, units: 0}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scoreJob(tt.job); got != tt.want {
				t.Fatalf("scoreJob(%+v) = %d, want %d", tt.job, got, tt.want)
			}
		})
	}
}
