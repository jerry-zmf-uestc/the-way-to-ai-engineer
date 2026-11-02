package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMemoryInputUsesCurrentSelectedFacts(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	scope := Scope{"tenant", "reader"}
	m := Memory{NewFake(), func() time.Time { return now }}
	consent := Consent{Scope: scope, Key: "response-style", Operation: "write", Value: "concise", Allowed: true, ExpiresAt: now.Add(time.Hour)}
	first, err := m.Write(ctx, scope, consent.Key, consent.Value, "private-confirmation", 0, time.Hour, consent)
	if err != nil {
		t.Fatal(err)
	}
	consent.Value = "detailed"
	current, err := m.Write(ctx, scope, consent.Key, consent.Value, "private-update", first.Revision, time.Hour, consent)
	if err != nil {
		t.Fatal(err)
	}
	input, err := m.Input(ctx, scope, []string{first.Key})
	if err != nil || len(input) != 1 || input[0].Value != "detailed" || input[0].Revision != current.Revision {
		t.Fatalf("input=%+v err=%v", input, err)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `[{"key":"response-style","value":"detailed","revision":2}]` {
		t.Fatalf("unexpected projection %s", raw)
	}
	input[0].Value = "concise"
	again, err := m.Input(ctx, scope, []string{first.Key})
	if err != nil || again[0].Value != "detailed" {
		t.Fatal("input mutated fact source")
	}
	foreign, err := m.Input(ctx, Scope{scope.Tenant, "other"}, []string{first.Key})
	if err != nil || len(foreign) != 0 {
		t.Fatal("foreign fact entered input")
	}
	consent.Operation = "delete"
	if _, err = m.Delete(ctx, scope, first.Key, current.Revision, consent); err != nil {
		t.Fatal(err)
	}
	deleted, err := m.Input(ctx, scope, []string{first.Key})
	if err != nil || len(deleted) != 0 {
		t.Fatal("deleted fact entered input")
	}
}

func TestMemoryInputExpiresWithoutExtendingConsent(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	scope := Scope{"tenant", "reader"}
	m := Memory{NewFake(), func() time.Time { return now }}
	c := Consent{Scope: scope, Key: "locale", Value: "en", Operation: "write", Allowed: true, ExpiresAt: now.Add(time.Minute)}
	if _, err := m.Write(ctx, scope, c.Key, c.Value, "confirmed", 0, time.Second, c); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	input, err := m.Input(ctx, scope, []string{c.Key})
	if err != nil || len(input) != 0 {
		t.Fatal("expired fact entered input")
	}
}
