package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPageExpiresAtReturnBoundary(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	now := func() time.Time {
		calls++
		if calls >= 3 {
			return start.Add(time.Second)
		}
		return start
	}
	w, _ := fixture(t, Scope{"t", "u", "task"}, now)
	a, err := w.Publish(context.Background(), "a", "private synthetic page\n", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	page, err := w.Read(context.Background(), a.ID, 0, 1)
	if !errors.Is(err, ErrDenied) || page.Text != "" || page.Complete {
		t.Fatal("expired page delivered", page, err)
	}
	if w.Used != a.Bytes {
		t.Fatal("access expiry was mistaken for deletion")
	}
}
