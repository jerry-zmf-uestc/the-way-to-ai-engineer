package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"-item", "gofmt=5", "-item", "errors=4"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr=%q", code, stderr.String())
	}
	want := "items=2 average=4.50 status=on-track weakest=errors\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunValidationError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"-item", "bad"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "review-cli: bad: review: invalid item") {
		t.Fatalf("stderr = %q, want validation message", stderr.String())
	}
}
