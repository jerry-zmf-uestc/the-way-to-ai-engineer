package main

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRevisionExhaustionDoesNotReuseOrCommitHistory(t *testing.T) {
	s := NewStore(time.Now)
	scope := Scope{"demo", "reader"}
	s.revision = ^uint64(0) - 1
	if err := s.Create(scope, "last", time.Hour); err != nil {
		t.Fatal(err)
	}
	before, err := s.Read(scope, "last")
	if err != nil || before.Version != ^uint64(0) {
		t.Fatal(before, err)
	}
	got, err := s.Append(scope, "last", before.Version, []Message{{Role: "user", Content: "new input"}})
	if !errors.Is(err, ErrRevisionExhausted) || got.Version != 0 || len(got.Messages) != 0 {
		t.Fatal("exhausted append accepted", got, err)
	}
	after, err := s.Read(scope, "last")
	if err != nil || !reflect.DeepEqual(before, after) || s.revision != ^uint64(0) {
		t.Fatal("failed append changed history", after, err)
	}
	if err := s.Create(scope, "new", time.Hour); !errors.Is(err, ErrRevisionExhausted) {
		t.Fatal("exhausted create accepted", err)
	}
	if _, err := s.Read(scope, "new"); !errors.Is(err, ErrNotFound) {
		t.Fatal("failed create published zero revision", err)
	}
}

func TestLastRevisionCanCommitOnceBeforeExhaustion(t *testing.T) {
	s := NewStore(time.Now)
	scope := Scope{"demo", "reader"}
	if err := s.Create(scope, "chat", time.Hour); err != nil {
		t.Fatal(err)
	}
	before, err := s.Read(scope, "chat")
	if err != nil {
		t.Fatal(err)
	}
	s.revision = ^uint64(0) - 1
	final, err := s.Append(scope, "chat", before.Version, []Message{{Role: "user", Content: "last input"}})
	if err != nil || final.Version != ^uint64(0) || len(final.Messages) != 1 {
		t.Fatal(final, err)
	}
	if _, err := s.Append(scope, "chat", final.Version, []Message{{Role: "user", Content: "another input"}}); !errors.Is(err, ErrRevisionExhausted) {
		t.Fatal("last revision reused", err)
	}
}
