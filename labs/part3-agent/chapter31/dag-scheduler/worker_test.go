package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStoppedWorkerDoesNotEnterCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	text, err := executeWork(ctx, func(context.Context) (string, error) { calls++; return "late", nil })
	if !errors.Is(err, context.Canceled) || calls != 0 || text != "" {
		t.Fatalf("text=%q calls=%d err=%v", text, calls, err)
	}
}
func TestWorkerCancellationWinsOverLateOutput(t *testing.T) {
	for _, size := range []int{4, 4097} {
		ctx, cancel := context.WithCancel(context.Background())
		text, err := executeWork(ctx, func(context.Context) (string, error) { cancel(); return strings.Repeat("x", size), nil })
		if !errors.Is(err, context.Canceled) || text != "" {
			t.Fatalf("size=%d text_length=%d err=%v", size, len(text), err)
		}
	}
}
func TestWorkerActiveResultBoundsAndCause(t *testing.T) {
	if text, err := executeWork(context.Background(), func(context.Context) (string, error) { return strings.Repeat("x", 4097), nil }); !errors.Is(err, ErrGraph) || text != "" {
		t.Fatal("oversized accepted")
	}
	cause := errors.New("tool failed")
	if text, err := executeWork(context.Background(), func(context.Context) (string, error) { return "partial", cause }); !errors.Is(err, cause) || text != "" {
		t.Fatal("failed output released", text, err)
	}
}
