package main

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	code := run(&stdout)
	if code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}

	want := "POST /api/echo -> 200 {\"echo\":\"hello, json api\",\"length\":15}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}
