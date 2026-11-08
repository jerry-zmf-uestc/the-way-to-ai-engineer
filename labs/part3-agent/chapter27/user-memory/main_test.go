package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func contract(t *testing.T, backend Backend) {
	t.Helper()
	ctx, cancelAll := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelAll()
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	scope := Scope{"fixture-" + t.Name(), "reader"}
	m := Memory{backend, func() time.Time { return now }}
	consent := Consent{Scope: scope, Key: "response-style", Operation: "write", Value: "concise", Allowed: true, ExpiresAt: now.Add(time.Minute)}
	denied := consent
	denied.Allowed = false
	if _, err := m.Write(ctx, scope, consent.Key, "concise", "user", 0, time.Hour, denied); !errors.Is(err, ErrDenied) {
		t.Fatal("unapproved persisted")
	}
	record, err := m.Write(ctx, scope, consent.Key, "concise", "user", 0, time.Hour, consent)
	if err != nil || record.Revision != 1 {
		t.Fatalf("create revision=%d err=%v", record.Revision, err)
	}
	for _, other := range []Scope{{scope.Tenant, "other"}, {"other", scope.User}} {
		rows, err := m.Recall(ctx, other)
		if err != nil || len(rows) != 0 {
			t.Fatal("scope leak")
		}
	}
	consent.Value = "detailed"
	if _, err := m.Write(ctx, scope, consent.Key, "detailed", "user", 0, time.Hour, consent); !errors.Is(err, ErrConflict) {
		t.Fatal("lost-update check missing")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.Write(ctx, scope, consent.Key, "detailed", "user", 1, time.Hour, consent)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("CAS race")
	}
	current, err := backend.Peek(ctx, scope, consent.Key)
	if err != nil {
		t.Fatal(err)
	}
	consent.Operation = "delete"
	deleted, err := m.Delete(ctx, scope, consent.Key, current.Revision, consent)
	if err != nil || !deleted.Deleted || deleted.Value != "" || deleted.Source != "" {
		t.Fatal("delete failed")
	}
	resolved, err := m.Resolve(ctx, scope, []string{consent.Key})
	if err != nil || len(resolved) != 0 {
		t.Fatal("stale index resurrected deleted value")
	}
	consent.Operation = "write"
	consent.Value = "concise"
	if _, err := m.Write(ctx, scope, consent.Key, "concise", "user", 1, time.Hour, consent); !errors.Is(err, ErrConflict) {
		t.Fatal("stale revision after deletion")
	}
	recreated, err := m.Write(ctx, scope, consent.Key, "concise", "new-user-confirmation", deleted.Revision, time.Second, consent)
	if err != nil || recreated.Revision <= deleted.Revision {
		t.Fatal("recreation reused revision")
	}
	now = now.Add(time.Second)
	rows, err := m.Recall(ctx, scope)
	if err != nil || len(rows) != 0 {
		t.Fatal("expiry not filtered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Recall(ctx, scope); !errors.Is(err, context.Canceled) {
		t.Fatal("context ignored")
	}
}
func TestFakeContract(t *testing.T) { contract(t, NewFake()) }
func TestPostgresContract(t *testing.T) {
	if os.Getenv("MEMORY_E2E") != "1" {
		t.Skip("explicit isolated PostgreSQL only")
	}
	if os.Getenv("MEMORY_TEST_DATABASE") != "1" {
		t.Fatal("fixture database confirmation required")
	}
	db, err := sql.Open("pgx", os.Getenv("MEMORY_DSN"))
	if err != nil {
		t.Fatal("database configuration failed")
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var name string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil || name != "memory_lab" {
		t.Fatal("requires isolated memory_lab database")
	}
	store := SQLStore{db}
	if err := store.Init(ctx); err != nil {
		t.Fatal("fixture schema setup failed")
	}
	contract(t, store)
}

func TestConsentBindsValueAndOperation(t *testing.T) {
	now := time.Now()
	scope := Scope{"t", "u"}
	m := Memory{NewFake(), func() time.Time { return now }}
	consent := Consent{Scope: scope, Key: "response-style", Operation: "write", Value: "concise", Allowed: true, ExpiresAt: now.Add(time.Minute)}
	if _, err := m.Write(context.Background(), scope, consent.Key, "detailed", "user", 0, time.Hour, consent); !errors.Is(err, ErrDenied) {
		t.Fatal("confirmed value replaced")
	}
	record, err := m.Write(context.Background(), scope, consent.Key, "concise", "user", 0, time.Hour, consent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Delete(context.Background(), scope, consent.Key, record.Revision, consent); !errors.Is(err, ErrDenied) {
		t.Fatal("write consent reused for deletion")
	}
	consent.ExpiresAt = now
	if _, err := m.Write(context.Background(), scope, consent.Key, "concise", "user", record.Revision, time.Hour, consent); !errors.Is(err, ErrDenied) {
		t.Fatal("expired consent accepted")
	}
}
