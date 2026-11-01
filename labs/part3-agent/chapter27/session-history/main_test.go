package main

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSnapshotsIsolationAndExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewStore(func() time.Time { return now })
	scope := Scope{"a", "u"}
	if err := s.Create(scope, "x", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(scope, "x", time.Hour); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate accepted")
	}
	initial, _ := s.Read(scope, "x")
	batch := []Message{{Role: "user", Content: "q"}, {Role: "assistant", Calls: []Call{{ID: "c", Name: "read", Args: json.RawMessage(`{"q":"original"}`)}}}}
	snap, err := s.Append(scope, "x", initial.Version, batch)
	if err != nil {
		t.Fatal(err)
	}
	batch[1].Calls[0].Args[6] = 'X'
	snap.Messages[1].Calls[0].Args[6] = 'Y'
	actual, err := s.Read(scope, "x")
	if err != nil || string(actual.Messages[1].Calls[0].Args) != `{"q":"original"}` {
		t.Fatalf("shared argument storage: %+v %v", actual, err)
	}
	actual.Messages[0].Content = "changed"
	again, _ := s.Read(scope, "x")
	if again.Messages[0].Content != "q" {
		t.Fatal("shared message storage")
	}
	for _, other := range []Scope{{"a", "other"}, {"b", "u"}} {
		if _, err := s.Read(other, "x"); !errors.Is(err, ErrNotFound) {
			t.Fatal("scope leaked")
		}
	}
	now = now.Add(time.Hour)
	if _, err := s.Read(scope, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatal("exact expiry not honored")
	}
	if err := s.Create(scope, "x", time.Hour); err != nil {
		t.Fatal(err)
	}
}
func TestCallAssociationAndAtomicRejection(t *testing.T) {
	s := NewStore(time.Now)
	scope := Scope{"t", "u"}
	if err := s.Create(scope, "x", time.Hour); err != nil {
		t.Fatal(err)
	}
	initial, _ := s.Read(scope, "x")
	start := []Message{{Role: "user", Content: "q"}, {Role: "assistant", Calls: []Call{{ID: "a", Name: "read", Args: json.RawMessage(`{}`)}, {ID: "b", Name: "read", Args: json.RawMessage(`{}`)}}}}
	snap, err := s.Append(scope, "x", initial.Version, start)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]Message{
		{{Role: "user", Content: "too early"}}, {{Role: "tool", CallID: "unknown"}}, {{Role: "assistant", Content: "early final"}},
		{{Role: "tool", CallID: "a"}, {Role: "tool", CallID: "a"}},
	} {
		if _, err := s.Append(scope, "x", snap.Version, bad); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid batch accepted: %v", err)
		}
		actual, _ := s.Read(scope, "x")
		if actual.Version != snap.Version || len(actual.Messages) != 2 {
			t.Fatal("partial commit")
		}
	}
	final, err := s.Append(scope, "x", snap.Version, []Message{{Role: "tool", CallID: "b", Content: "B"}, {Role: "tool", CallID: "a", Content: "A"}, {Role: "assistant", Content: "final"}})
	if err != nil || final.Version != initial.Version+2 || len(final.Messages) != 5 {
		t.Fatalf("association failed: %+v %v", final, err)
	}
}
func TestConcurrentCompareAndAppend(t *testing.T) {
	s := NewStore(time.Now)
	scope := Scope{"t", "u"}
	s.Create(scope, "x", time.Hour)
	initial, _ := s.Read(scope, "x")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Append(scope, "x", initial.Version, []Message{{Role: "user", Content: "q"}})
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
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	snap, _ := s.Read(scope, "x")
	if snap.Version != initial.Version+1 || len(snap.Messages) != 1 {
		t.Fatal("lost update")
	}
}
func TestInvalidDefinitionsAndBounds(t *testing.T) {
	s := NewStore(time.Now)
	scope := Scope{"t", "u"}
	if err := s.Create(Scope{}, "x", time.Hour); !errors.Is(err, ErrInvalid) {
		t.Fatal("empty scope accepted")
	}
	s.Create(scope, "x", time.Hour)
	initial, _ := s.Read(scope, "x")
	for _, batch := range [][]Message{
		{}, {{Role: "alien", Content: "q"}}, {{Role: "user", Content: ""}}, {{Role: "user", Content: "q", CallID: "x"}},
		{{Role: "assistant", Calls: []Call{{ID: "a", Name: "read", Args: json.RawMessage(`bad`)}}}},
		{{Role: "assistant", Calls: []Call{{ID: "a", Name: "read", Args: json.RawMessage(`{}`)}, {ID: "a", Name: "read", Args: json.RawMessage(`{}`)}}}},
	} {
		if _, err := s.Append(scope, "x", initial.Version, batch); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted invalid: %v", err)
		}
	}
	large := make([]Message, 129)
	for i := range large {
		large[i] = Message{Role: "user", Content: "q"}
	}
	if _, err := s.Append(scope, "x", initial.Version, large); !errors.Is(err, ErrInvalid) {
		t.Fatal("limit ignored")
	}
}

func TestExpiredIdentityDoesNotReuseVersion(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewStore(func() time.Time { return now })
	scope := Scope{"t", "u"}
	if err := s.Create(scope, "x", time.Second); err != nil {
		t.Fatal(err)
	}
	old, _ := s.Read(scope, "x")
	now = now.Add(time.Second)
	if _, err := s.Append(scope, "x", old.Version, []Message{{Role: "user", Content: "late"}}); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired append accepted")
	}
	if err := s.Create(scope, "x", time.Second); err != nil {
		t.Fatal(err)
	}
	fresh, _ := s.Read(scope, "x")
	if fresh.Version <= old.Version {
		t.Fatal("version reused")
	}
	if _, err := s.Append(scope, "x", old.Version, []Message{{Role: "user", Content: "stale"}}); !errors.Is(err, ErrConflict) {
		t.Fatal("old identity accepted")
	}
	if _, err := s.Append(Scope{"t", "other"}, "x", fresh.Version, []Message{{Role: "user", Content: "foreign"}}); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign append accepted")
	}
}
