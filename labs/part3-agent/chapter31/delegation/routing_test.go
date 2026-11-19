package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRoleRoutingBeforeDelegation(t *testing.T) {
	calls := 0
	worker := func(ctx context.Context, task Task, box *Toolbox) (Result, error) {
		calls++
		if _, err := box.Read(ctx, "write"); !errors.Is(err, ErrDenied) {
			t.Fatal("route expanded permissions", err)
		}
		fact, err := box.Read(ctx, "read")
		return Result{TaskID: task.ID, Parent: task.Parent, Actor: "reader", Scope: task.Scope, Facts: []Fact{fact}}, err
	}
	d := Dispatcher{
		Roles: map[string]Role{
			"reader": {Name: "reader", Tools: []string{"read", "write"}, Run: worker},
			"other":  {Name: "other", Run: worker},
		},
		ParentTools: map[string]Tool{"read": func(context.Context, Scope) (Fact, error) {
			return Fact{Key: "timeout", Value: "30", Source: "policy-v1"}, nil
		}},
		Ledger: &Ledger{Limit: 1}, MaxDepth: 2,
	}
	routes := map[string][]string{"timeout": {"reader"}, "ambiguous": {"reader", "other"}}
	task := Task{ID: "child", Parent: "parent", Scope: Scope{Tenant: "demo", User: "reader"}, Goal: "read timeout"}
	for _, tc := range []struct {
		kind, proposal string
		want           error
	}{
		{"timeout", "other", ErrDenied},
		{"timeout", "invented", ErrDenied},
		{"ambiguous", "", ErrClarify},
		{"unknown", "reader", ErrRoute},
	} {
		role, err := SelectRole(tc.kind, tc.proposal, routes, d.Roles)
		if !errors.Is(err, tc.want) || role != "" {
			t.Fatalf("%+v: role=%q err=%v", tc, role, err)
		}
	}
	if calls != 0 || d.Ledger.Used != 0 {
		t.Fatal("rejected routing executed worker or charged tools")
	}
	for _, proposal := range []string{"", "reader"} {
		role, err := SelectRole("timeout", proposal, routes, d.Roles)
		if err != nil || role != "reader" {
			t.Fatal(role, err)
		}
	}
	role, err := SelectRole("ambiguous", "reader", routes, d.Roles)
	if err != nil {
		t.Fatal(err)
	}
	result, err := d.Delegate(context.Background(), task, role)
	if err != nil || result.Actor != "reader" || result.Parent != "parent" || len(result.Facts) != 1 || calls != 1 || d.Ledger.Used != 1 {
		t.Fatal(result, err, calls, d.Ledger.Used)
	}
	if !reflect.DeepEqual(routes, map[string][]string{"timeout": {"reader"}, "ambiguous": {"reader", "other"}}) {
		t.Fatal("routing mutated configuration")
	}
}

func TestRoleRoutingRejectsBrokenConfiguration(t *testing.T) {
	noop := func(context.Context, Task, *Toolbox) (Result, error) { return Result{}, nil }
	roles := map[string]Role{"reader": {Name: "reader", Run: noop}}
	for _, routes := range []map[string][]string{
		nil, {}, {"": {"reader"}}, {"timeout": nil},
		{"timeout": {"reader", "reader"}}, {"timeout": {"missing"}},
		{"timeout": {"reader"}, "broken": {"missing"}},
		{strings.Repeat("x", 129): {"reader"}},
		{"timeout": {"reader", "a", "b", "c", "d", "e", "f", "g", "h"}},
	} {
		if role, err := SelectRole("timeout", "reader", routes, roles); role != "" || !errors.Is(err, ErrRoute) {
			t.Fatal("broken route accepted", routes, role, err)
		}
	}
	for _, invalid := range []map[string]Role{
		nil, {"reader": {Name: "reader"}}, {"reader": {Name: "wrong", Run: noop}},
	} {
		if _, err := SelectRole("timeout", "reader", map[string][]string{"timeout": {"reader"}}, invalid); !errors.Is(err, ErrRoute) {
			t.Fatal("broken registry accepted", err)
		}
	}
	if _, err := SelectRole("timeout", strings.Repeat("x", 129), map[string][]string{"timeout": {"reader"}}, roles); !errors.Is(err, ErrDenied) {
		t.Fatal("oversized proposal accepted", err)
	}
	tooMany := map[string][]string{}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		tooMany[name] = []string{"reader"}
	}
	if _, err := SelectRole("a", "", tooMany, roles); !errors.Is(err, ErrRoute) {
		t.Fatal("too many routes accepted", err)
	}
}
