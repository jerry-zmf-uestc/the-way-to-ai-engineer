package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type readBackend struct {
	Backend
	active func(context.Context, Scope, time.Time) ([]Record, error)
	peek   func(context.Context, Scope, string) (Record, error)
}

func (b readBackend) Active(ctx context.Context, s Scope, now time.Time) ([]Record, error) {
	return b.active(ctx, s, now)
}
func (b readBackend) Peek(ctx context.Context, s Scope, key string) (Record, error) {
	return b.peek(ctx, s, key)
}
func TestRecallRejectsLateAndCorruptBackendRows(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	valid := Record{Key: "locale", Value: "zh", Source: "confirmed", Revision: 1, ExpiresAt: now.Add(time.Second)}
	rows := []Record{valid}
	backend := readBackend{active: func(context.Context, Scope, time.Time) ([]Record, error) { return rows, nil }}
	m := Memory{Store: backend, Now: func() time.Time { return now }}
	scope := Scope{"t", "u"}
	for _, bad := range []Record{{Key: "locale", Value: "injected", Source: "confirmed", Revision: 1, ExpiresAt: valid.ExpiresAt}, {Key: "locale", Value: "zh", Source: "confirmed", Revision: 0, ExpiresAt: valid.ExpiresAt}, valid} {
		rows = []Record{valid, bad}
		out, err := m.Recall(context.Background(), scope)
		if !errors.Is(err, ErrInput) || out != nil {
			t.Fatal("corrupt partial result", out, err)
		}
	}
	backend.active = func(context.Context, Scope, time.Time) ([]Record, error) {
		now = now.Add(time.Second)
		return []Record{valid}, nil
	}
	m.Store = backend
	out, err := m.Recall(context.Background(), scope)
	if err != nil || len(out) != 0 {
		t.Fatal("expiry during backend read", out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend.active = func(context.Context, Scope, time.Time) ([]Record, error) { cancel(); return []Record{valid}, nil }
	m.Store = backend
	if out, err = m.Recall(ctx, scope); !errors.Is(err, context.Canceled) || out != nil {
		t.Fatal("late canceled recall", out, err)
	}
}
func TestResolveChecksReturnedIdentityAndCancel(t *testing.T) {
	now := time.Now()
	record := Record{Key: "response-style", Value: "concise", Source: "confirmed", Revision: 1, ExpiresAt: now.Add(time.Hour)}
	backend := readBackend{peek: func(context.Context, Scope, string) (Record, error) { return record, nil }}
	m := Memory{Store: backend, Now: func() time.Time { return now }}
	scope := Scope{"t", "u"}
	if out, err := m.Resolve(context.Background(), scope, []string{"locale"}); !errors.Is(err, ErrInput) || out != nil {
		t.Fatal("different key accepted", out, err)
	}
	record = Record{Key: "locale", Deleted: true, Revision: 2, ExpiresAt: now.Add(time.Hour)}
	if out, err := m.Resolve(context.Background(), scope, []string{"locale"}); err != nil || len(out) != 0 {
		t.Fatal("tombstone treated as active invalid preference", out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := m.Resolve(ctx, scope, nil); !errors.Is(err, context.Canceled) || out != nil {
		t.Fatal("empty candidates hide cancellation", out, err)
	}
}
func TestRecallOrderingAndBackendFailure(t *testing.T) {
	now := time.Now()
	rows := []Record{{Key: "response-style", Value: "concise", Source: "confirmed", Revision: 1, ExpiresAt: now.Add(time.Hour)}, {Key: "locale", Value: "zh", Source: "confirmed", Revision: 1, ExpiresAt: now.Add(time.Hour)}}
	cause := errors.New("storage unavailable")
	failure := false
	backend := readBackend{active: func(context.Context, Scope, time.Time) ([]Record, error) {
		if failure {
			return nil, cause
		}
		return rows, nil
	}}
	m := Memory{Store: backend, Now: func() time.Time { return now }}
	scope := Scope{"t", "u"}
	out, err := m.Recall(context.Background(), scope)
	if err != nil || len(out) != 2 || out[0].Key != "locale" {
		t.Fatal("unstable order", out, err)
	}
	failure = true
	if out, err = m.Recall(context.Background(), scope); !errors.Is(err, cause) || out != nil {
		t.Fatal("error masked as absence", out, err)
	}
}
