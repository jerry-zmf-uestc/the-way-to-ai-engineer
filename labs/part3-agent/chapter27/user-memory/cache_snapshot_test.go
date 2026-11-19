package main

import (
	"context"
	"testing"
	"time"
)

func TestLateCacheFillCannotReplaceCurrentMemory(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	scope := Scope{"cache-demo", "reader"}
	memory := Memory{NewFake(), func() time.Time { return now }}
	consent := Consent{Scope: scope, Key: "response-style", Operation: "write", Value: "concise", Allowed: true, ExpiresAt: now.Add(time.Minute)}
	first, err := memory.Write(ctx, scope, consent.Key, "concise", "confirmation-1", 0, time.Hour, consent)
	if err != nil {
		t.Fatal(err)
	}
	// A read has obtained this snapshot but has not yet filled its cache.
	lateRead, err := memory.Store.Peek(ctx, scope, consent.Key)
	if err != nil || lateRead != first {
		t.Fatal("initial source read failed", lateRead, err)
	}
	consent.Value = "detailed"
	current, err := memory.Write(ctx, scope, consent.Key, "detailed", "confirmation-2", first.Revision, time.Hour, consent)
	if err != nil {
		t.Fatal(err)
	}
	var cache []Record              // The writer has invalidated the cache after committing revision 2.
	cache = append(cache, lateRead) // The earlier reader now fills revision 1.
	if cache[0].Value != "concise" || cache[0].Revision >= current.Revision || !now.Before(cache[0].ExpiresAt) {
		t.Fatal("counterexample no longer represents a fresh but stale cache")
	}
	candidates := []string{cache[0].Key}
	resolved, err := memory.Resolve(ctx, scope, candidates)
	if err != nil || len(resolved) != 1 || resolved[0] != current {
		t.Fatal("old cache value reached input", resolved, err)
	}
	for _, other := range []Scope{{scope.Tenant, "another-reader"}, {"another-tenant", scope.User}} {
		resolved, err := memory.Resolve(ctx, other, candidates)
		if err != nil || len(resolved) != 0 {
			t.Fatal("cached candidate bypassed scope", resolved, err)
		}
	}
	consent.Operation = "delete"
	if _, err := memory.Delete(ctx, scope, consent.Key, current.Revision, consent); err != nil {
		t.Fatal(err)
	}
	resolved, err = memory.Resolve(ctx, scope, candidates)
	if err != nil || len(resolved) != 0 || cache[0].Value != "concise" {
		t.Fatal("unchanged cache revived deletion", resolved, err)
	}
}
