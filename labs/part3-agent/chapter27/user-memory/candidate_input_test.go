package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestResolvePreflightsEveryCandidateBeforeBackendRead(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"empty", ""}, {"blank", " \t"}, {"oversize", strings.Repeat("x", 129)},
		{"multibyte", strings.Repeat("界", 43)}, {"invalid-utf8", string([]byte{0xff})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			m := Memory{Store: readBackend{peek: func(context.Context, Scope, string) (Record, error) {
				calls++
				return Record{}, ErrNotFound
			}}}
			out, err := m.Resolve(context.Background(), Scope{"demo", "reader"}, []string{"locale", tc.value})
			if !errors.Is(err, ErrInput) || out != nil || calls != 0 {
				t.Fatalf("invalid candidate reached backend: bytes=%d calls=%d out=%v err=%v", len(tc.value), calls, out, err)
			}
		})
	}
}

func TestResolveKeepsBoundedUnknownCandidateAndDeduplication(t *testing.T) {
	calls := 0
	key := strings.Repeat("x", 128)
	m := Memory{Store: readBackend{peek: func(_ context.Context, _ Scope, received string) (Record, error) {
		calls++
		if received != key {
			t.Fatal("candidate was normalized or truncated")
		}
		return Record{}, ErrNotFound
	}}}
	out, err := m.Resolve(context.Background(), Scope{"demo", "reader"}, []string{key, key})
	if err != nil || len(out) != 0 || calls != 1 {
		t.Fatal(out, err, calls)
	}
}
