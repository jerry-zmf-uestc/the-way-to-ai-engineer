package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDelegationRejectsOversizedContextBeforeWorker(t *testing.T) {
	for _, fact := range []Fact{
		{"key", strings.Repeat("v", 4097), "source"},
		{strings.Repeat("k", 129), "value", "source"},
		{"key", "value", strings.Repeat("s", 257)},
		{"", "value", "source"},
	} {
		d := dispatch()
		called := false
		d.Roles["probe"] = Role{Name: "probe", Run: func(_ context.Context, task Task, _ *Toolbox) (Result, error) {
			called = true
			return Result{TaskID: task.ID, Parent: task.Parent, Actor: "probe", Scope: task.Scope}, nil
		}}
		input := task()
		input.Context = []Fact{fact}
		if _, err := d.Delegate(context.Background(), input, "probe"); !errors.Is(err, ErrResult) || called || d.Ledger.Used != 0 {
			t.Errorf("context admitted: err=%v called=%t used=%d", err, called, d.Ledger.Used)
		}
		if _, err := NewHandoff(input, d, "probe", nil, 1, 1); !errors.Is(err, ErrResult) {
			t.Errorf("handoff context admitted: %v", err)
		}
	}
}

func TestToolRejectsOversizedProvenanceBeforeObservation(t *testing.T) {
	for _, fact := range []Fact{
		{strings.Repeat("k", 129), "value", "source"},
		{"key", "value", strings.Repeat("s", 257)},
	} {
		calls := 0
		ledger := &Ledger{Limit: 1}
		box := &Toolbox{scope: Scope{"tenant", "reader"}, ledger: ledger, tools: map[string]Tool{
			"read": func(context.Context, Scope) (Fact, error) { calls++; return fact, nil },
		}}
		got, err := box.Read(context.Background(), "read")
		if !errors.Is(err, ErrResult) || got != (Fact{}) || len(box.observed) != 0 || calls != 1 || ledger.Used != 1 {
			t.Errorf("invalid observation admitted: fact=%+v err=%v observed=%d calls=%d used=%d", got, err, len(box.observed), calls, ledger.Used)
		}
	}
}

func TestMissingPayloadAndFactBoundary(t *testing.T) {
	for _, missing := range []string{"", strings.Repeat("m", 257), strings.Repeat("界", 86)} {
		d := dispatch()
		d.Roles["probe"] = Role{Name: "probe", Run: func(_ context.Context, task Task, _ *Toolbox) (Result, error) {
			return Result{TaskID: task.ID, Parent: task.Parent, Actor: "probe", Scope: task.Scope, Missing: []string{missing}}, nil
		}}
		if result, err := d.Delegate(context.Background(), task(), "probe"); !errors.Is(err, ErrResult) || result.TaskID != "" {
			t.Errorf("invalid Missing accepted: bytes=%d err=%v", len(missing), err)
		}
	}
	input := task()
	input.Context = []Fact{{strings.Repeat("k", 128), strings.Repeat("v", 4096), strings.Repeat("s", 256)}}
	d := dispatch()
	d.Roles["probe"] = Role{Name: "probe", Run: func(_ context.Context, task Task, _ *Toolbox) (Result, error) {
		return Result{TaskID: task.ID, Parent: task.Parent, Actor: "probe", Scope: task.Scope, Facts: task.Context, Missing: []string{strings.Repeat("m", 256)}}, nil
	}}
	result, err := d.Delegate(context.Background(), input, "probe")
	if err != nil || len(result.Facts) != 1 || result.Facts[0] != input.Context[0] || len(result.Missing[0]) != 256 {
		t.Fatal("legal boundary rejected", result, err)
	}
	if _, err := NewHandoff(input, d, "probe", nil, 1, 1); err != nil {
		t.Fatal("legal handoff boundary rejected", err)
	}
}
