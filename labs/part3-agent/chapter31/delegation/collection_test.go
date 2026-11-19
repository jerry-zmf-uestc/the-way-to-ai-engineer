package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCollectResultsPreservesEvidenceAndMissing(t *testing.T) {
	for _, secondValue := range []string{"30", "60"} {
		d := dispatch()
		scope := Scope{"t", "u"}
		expected := map[string]string{"a": "a-reader", "b": "b-reader"}
		var results []Result
		for _, id := range []string{"a", "b"} {
			value := "30"
			if id == "b" {
				value = secondValue
			}
			fact := Fact{"timeout", value, "source-" + id}
			name := id + "-reader"
			d.Roles[name] = Role{Name: name, Run: func(_ context.Context, task Task, _ *Toolbox) (Result, error) {
				return Result{TaskID: task.ID, Parent: task.Parent, Actor: name, Scope: task.Scope, Facts: task.Context, Missing: []string{"environment"}}, nil
			}}
			result, err := d.Delegate(context.Background(), Task{ID: id, Parent: "p", Scope: scope, Context: []Fact{fact}}, name)
			if err != nil {
				t.Fatal(err)
			}
			results = append(results, result)
		}
		a, err := CollectResults(scope, "p", expected, results)
		if err != nil || len(a) != 2 || a[0].Facts[0].Source != "source-a" || a[1].Facts[0].Source != "source-b" || len(a[1].Missing) != 1 {
			t.Fatal(a, err)
		}
		b, err := CollectResults(scope, "p", expected, []Result{results[1], results[0]})
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("arrival order changed collection", b, err)
		}
		a[0].Facts[0].Value = "mutated"
		a[0].Missing[0] = "mutated"
		if results[0].Facts[0].Value != "30" || results[0].Missing[0] != "environment" {
			t.Fatal("collection shares mutable slices")
		}
		_, err = Aggregate(scope, "p", expected, results)
		if (secondValue == "60") != errors.Is(err, ErrResult) {
			t.Fatal("legacy conflict policy changed", err)
		}
	}
}

func TestCollectResultsRejectsBrokenSet(t *testing.T) {
	scope := Scope{"t", "u"}
	a := Result{TaskID: "a", Parent: "p", Actor: "reader", Scope: scope, Facts: []Fact{{"timeout", "30", "source"}}}
	expected := map[string]string{"a": "reader"}
	for _, change := range []func(*Result){
		func(r *Result) { r.Parent = "other" },
		func(r *Result) { r.Actor = "other" },
		func(r *Result) { r.Scope.User = "other" },
		func(r *Result) { r.TaskID = "other" },
		func(r *Result) { r.Facts = []Fact{{"timeout", strings.Repeat("x", 4097), "source"}} },
		func(r *Result) { r.Facts = make([]Fact, 9) },
		func(r *Result) { r.Missing = []string{""} },
		func(r *Result) { r.Missing = []string{strings.Repeat("x", 257)} },
	} {
		bad := a
		change(&bad)
		if out, err := CollectResults(scope, "p", expected, []Result{bad}); !errors.Is(err, ErrResult) || out != nil {
			t.Fatal("bad set returned partial collection", out, err)
		}
	}
	for _, results := range [][]Result{nil, {a, a}} {
		if out, err := CollectResults(scope, "p", expected, results); !errors.Is(err, ErrResult) || out != nil {
			t.Fatal(out, err)
		}
	}
}
