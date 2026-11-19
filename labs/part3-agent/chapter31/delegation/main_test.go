package main

import (
	"context"
	"errors"
	"testing"
)

func dispatch() Dispatcher {
	return Dispatcher{Roles: roles(), ParentTools: tools(), Ledger: &Ledger{Limit: 2}, MaxDepth: 2}
}
func task() Task {
	return Task{ID: "t", Parent: "parent", Scope: Scope{"tenant", "reader"}, Goal: "timeout", Context: []Fact{{"constraint", "readonly", "user"}}}
}
func TestChildPolicyContextAndResultIdentity(t *testing.T) {
	d := dispatch()
	d.Roles["probe"] = Role{Name: "probe", Tools: []string{"read-timeout", "write-file"}, Run: func(ctx context.Context, t Task, box *Toolbox) (Result, error) {
		if len(t.Context) != 1 || t.Depth != 1 {
			panic("wrong child input")
		}
		t.Context[0].Value = "changed"
		if _, err := box.Read(ctx, "write-file"); !errors.Is(err, ErrDenied) {
			panic("permission inherited")
		}
		fact, err := box.Read(ctx, "read-timeout")
		return Result{TaskID: t.ID, Parent: t.Parent, Actor: "probe", Scope: t.Scope, Facts: []Fact{fact}}, err
	}}
	parent := task()
	result, err := d.Delegate(context.Background(), parent, "probe")
	if err != nil || len(result.Facts) != 1 || parent.Context[0].Value != "readonly" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
func TestProvenanceIdentityDepthAndBudget(t *testing.T) {
	d := dispatch()
	d.Roles["liar"] = Role{Name: "liar", Run: func(_ context.Context, t Task, _ *Toolbox) (Result, error) {
		return Result{TaskID: t.ID, Parent: t.Parent, Actor: "liar", Scope: t.Scope, Facts: []Fact{{"timeout", "999", "invented"}}}, nil
	}}
	if _, err := d.Delegate(context.Background(), task(), "liar"); !errors.Is(err, ErrResult) {
		t.Fatal("invented fact accepted")
	}
	deep := task()
	deep.Depth = 2
	if _, err := d.Delegate(context.Background(), deep, "timeout-reader"); !errors.Is(err, ErrDenied) {
		t.Fatal("unbounded delegation")
	}
	d.Ledger.Limit = 1
	d.Delegate(context.Background(), task(), "timeout-reader")
	if _, err := d.Delegate(context.Background(), task(), "timeout-reader"); !errors.Is(err, ErrBudget) {
		t.Fatal("budget reset per child")
	}
}
func TestConflictingFactsAndLateCancellation(t *testing.T) {
	if _, err := Aggregate(Scope{"t", "u"}, "parent", map[string]string{"a": "reader-a", "b": "reader-b"}, []Result{{TaskID: "a", Parent: "parent", Actor: "reader-a", Scope: Scope{"t", "u"}, Facts: []Fact{{"timeout", "30", "s"}}}, {TaskID: "b", Parent: "parent", Actor: "reader-b", Scope: Scope{"t", "u"}, Facts: []Fact{{"timeout", "60", "s2"}}}}); !errors.Is(err, ErrResult) {
		t.Fatal("conflict hidden")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := dispatch()
	d.Roles["late"] = Role{Name: "late", Run: func(_ context.Context, t Task, _ *Toolbox) (Result, error) {
		cancel()
		return Result{TaskID: t.ID, Parent: t.Parent, Actor: "late", Scope: t.Scope}, nil
	}}
	if _, err := d.Delegate(ctx, task(), "late"); !errors.Is(err, context.Canceled) {
		t.Fatal("late result accepted")
	}
}

func TestSameUserDifferentParentsCannotBeAggregated(t *testing.T) {
	d := dispatch()
	scope := Scope{"same-tenant", "same-user"}
	a, err := d.Delegate(context.Background(), Task{ID: "a-child", Parent: "parent-a", Scope: scope, Goal: "timeout"}, "timeout-reader")
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.Delegate(context.Background(), Task{ID: "b-child", Parent: "parent-b", Scope: scope, Goal: "retry"}, "retry-reader")
	if err != nil {
		t.Fatal(err)
	}
	if facts, err := Aggregate(scope, "parent-a", map[string]string{"a-child": "timeout-reader", "b-child": "retry-reader"}, []Result{a, b}); !errors.Is(err, ErrResult) || facts != nil {
		t.Fatal("different parent result merged", facts, err)
	}
	if facts, err := Aggregate(scope, "parent-a", map[string]string{a.TaskID: "timeout-reader"}, []Result{a}); err != nil || facts["timeout"].Value != "30" {
		t.Fatal("valid parent rejected", err)
	}
	if _, err := Aggregate(scope, "", map[string]string{a.TaskID: "timeout-reader"}, []Result{a}); !errors.Is(err, ErrResult) {
		t.Fatal("unbound aggregate parent")
	}
}

func TestWorkerCannotRelabelResultParent(t *testing.T) {
	d := dispatch()
	d.Roles["wrong-parent"] = Role{Name: "wrong-parent", Run: func(_ context.Context, t Task, _ *Toolbox) (Result, error) {
		return Result{TaskID: t.ID, Parent: "another-run", Actor: "wrong-parent", Scope: t.Scope}, nil
	}}
	if _, err := d.Delegate(context.Background(), task(), "wrong-parent"); !errors.Is(err, ErrResult) {
		t.Fatal("worker changed parent binding", err)
	}
}

func TestAggregateChecksTrustedExpectedChildren(t *testing.T) {
	d := dispatch()
	scope := Scope{"tenant", "reader"}
	expected := map[string]string{"timeout-child": "timeout-reader", "retry-child": "retry-reader"}
	a, err := d.Delegate(context.Background(), Task{ID: "timeout-child", Parent: "parent", Scope: scope, Goal: "timeout"}, expected["timeout-child"])
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.Delegate(context.Background(), Task{ID: "retry-child", Parent: "parent", Scope: scope, Goal: "retry"}, expected["retry-child"])
	if err != nil {
		t.Fatal(err)
	}
	wrongActor, unrequested := b, b
	wrongActor.Actor = "timeout-reader"
	unrequested.TaskID = "another-child"
	for _, results := range [][]Result{{a}, {a, b, unrequested}, {a, a}, {a, unrequested}, {a, wrongActor}} {
		if facts, err := Aggregate(scope, "parent", expected, results); !errors.Is(err, ErrResult) || facts != nil {
			t.Fatal("incomplete or unrequested result accepted", results, facts, err)
		}
	}
	for _, results := range [][]Result{{a, b}, {b, a}} {
		facts, err := Aggregate(scope, "parent", expected, results)
		if err != nil || len(facts) != 2 {
			t.Fatal("complete set or arrival reorder rejected", err)
		}
	}
	if _, err := Aggregate(scope, "parent", map[string]string{"timeout-child": ""}, []Result{a}); !errors.Is(err, ErrResult) {
		t.Fatal("missing trusted role accepted")
	}
}

func TestCollectedResultDoesNotEraseMissingGoal(t *testing.T) {
	d := dispatch()
	d.Roles["missing-reader"] = Role{Name: "missing-reader", Run: func(_ context.Context, t Task, _ *Toolbox) (Result, error) {
		return Result{TaskID: t.ID, Parent: t.Parent, Actor: "missing-reader", Scope: t.Scope, Missing: []string{"timeout"}}, nil
	}}
	target := task()
	expected := map[string]string{target.ID: "missing-reader"}
	r, err := d.Delegate(context.Background(), target, "missing-reader")
	if err != nil {
		t.Fatal(err)
	}
	facts, err := Aggregate(target.Scope, target.Parent, expected, []Result{r})
	if err != nil || len(facts) != 0 || len(r.Missing) != 1 || r.Missing[0] != "timeout" {
		t.Fatal("collection completion hid missing evidence", facts, r, err)
	}
}
