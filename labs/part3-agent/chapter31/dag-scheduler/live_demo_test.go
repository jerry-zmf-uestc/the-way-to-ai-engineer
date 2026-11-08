package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLiveDemoStopsProducerBeforeParentDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	report, err := liveDemo(ctx)
	if ctx.Err() != nil {
		t.Fatal("producer only stopped when parent deadline expired")
	}
	if !errors.Is(err, ErrQueue) || report.Free["db"] != 1 || len(report.Results) != 0 || report.States["a1"] != "canceled" || report.States["a2"] != "canceled" {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}
