package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryPreservesCurrentFactsAndOwnership(t *testing.T) {
	current := summaryExample()
	original := Snapshot{Version: current.Version, Facts: append([]Fact(nil), current.Facts...)}
	candidate := Snapshot{Version: current.Version, Facts: []Fact{current.Facts[2], current.Facts[0], current.Facts[1]}}
	got, err := ValidateSummary(context.Background(), current, candidate)
	if err != nil || !reflect.DeepEqual(got, candidate) {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	got.Facts[0].State = "known"
	if candidate.Facts[0].State != "unknown" || !reflect.DeepEqual(current, original) {
		t.Fatal("published summary aliases source or candidate")
	}
}

func TestSummaryRejectsLossAndFabricationWithoutPublishing(t *testing.T) {
	current := summaryExample()
	changes := map[string]func(*Snapshot){
		"omitted gap":     func(s *Snapshot) { s.Facts = s.Facts[:2] },
		"stale version":   func(s *Snapshot) { s.Version = "task-v1" },
		"old amount":      func(s *Snapshot) { s.Facts[0].Value = "5000" },
		"changed unit":    func(s *Snapshot) { s.Facts[0].Key = "budget_usd" },
		"changed source":  func(s *Snapshot) { s.Facts[0].Source = "model-suggestion" },
		"changed scope":   func(s *Snapshot) { s.Facts[1].Value = "false" },
		"invented known":  func(s *Snapshot) { s.Facts[2].State, s.Facts[2].Value = "known", "true" },
		"invented fact":   func(s *Snapshot) { s.Facts = append(s.Facts, Fact{"extra", "yes", "user", "known"}) },
		"duplicate fact":  func(s *Snapshot) { s.Facts[2] = s.Facts[0] },
		"empty source":    func(s *Snapshot) { s.Facts[0].Source = " " },
		"invalid state":   func(s *Snapshot) { s.Facts[0].State = "maybe" },
		"unknown value":   func(s *Snapshot) { s.Facts[2].Value = "true" },
		"excess bytes":    func(s *Snapshot) { s.Facts[0].Value = strings.Repeat("a", 64<<10) },
		"empty candidate": func(s *Snapshot) { s.Facts = nil },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			candidate := Snapshot{Version: current.Version, Facts: append([]Fact(nil), current.Facts...)}
			change(&candidate)
			got, err := ValidateSummary(context.Background(), current, candidate)
			if !errors.Is(err, ErrSummary) || got.Version != "" || len(got.Facts) != 0 {
				t.Fatalf("partial or invalid summary published: %+v %v", got, err)
			}
		})
	}
}

func TestSummaryRejectsInvalidSnapshotAndCancellation(t *testing.T) {
	current := summaryExample()
	invalid := current
	invalid.Version = ""
	if _, err := ValidateSummary(context.Background(), invalid, current); !errors.Is(err, ErrSummary) {
		t.Fatal("unversioned authority accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := ValidateSummary(ctx, current, current)
	if !errors.Is(err, context.Canceled) || len(got.Facts) != 0 {
		t.Fatal("canceled summary published")
	}
}
