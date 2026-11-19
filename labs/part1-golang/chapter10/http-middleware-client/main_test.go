package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestRun_Output(t *testing.T) {
	got := captureStdout(t, run)
	want := "" +
		"demo=echo status=200 body={\"echo\":\"ok\"} attempts=1\n" +
		"demo=recover status=500 body=internal error attempts=1\n" +
		"demo=timeout status=503 body=timeout attempts=1\n" +
		"demo=retry-then-ok status=200 body={\"echo\":\"ok\"} attempts=3\n"
	if got != want {
		t.Fatalf("run() output:\n%s\nwant:\n%s", got, want)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
