package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLedgerCLIWorker(t *testing.T) {
	if os.Getenv("COST_CLI_WORKER") != "1" {
		t.Skip("helper process")
	}
	for i, arg := range os.Args {
		if arg == "--" {
			handled, code := RunLedgerCLI(os.Args[i+1:], os.Stdout)
			if !handled {
				os.Exit(2)
			}
			os.Exit(code)
		}
	}
	t.Fatal("missing CLI delimiter")
}

func invokeLedgerCLI(t *testing.T, dir, action string, extra ...string) (LedgerCLIResult, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := append([]string{"-test.run=^TestLedgerCLIWorker$", "--", "-ledger-dir", dir, "-ledger-action", action}, extra...)
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), "COST_CLI_WORKER=1")
	data, err := cmd.Output()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	if len(data) > 64<<10 {
		t.Fatal("unbounded CLI response")
	}
	var result LedgerCLIResult
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		t.Fatalf("response %s: %v", data, err)
	}
	if dec.Decode(new(any)) != io.EOF || result.Schema != 1 {
		t.Fatal("not one protocol response", string(data))
	}
	return result, code
}

func TestLedgerCLIProcessBoundaryAndReplayPreparation(t *testing.T) {
	dir := t.TempDir()
	costRoot(t, dir)
	key, hash := "attempt-1", strings.Repeat("a", 64)
	for range 2 {
		r, code := invokeLedgerCLI(t, dir, "prepare", "-attempt-key", key, "-request-hash", hash)
		if code != 0 || r.EntryID != 1 || r.AttemptKey != key || r.RequestHash != hash || r.Identity != costIdentity() || r.Snapshot.Held != 50 || len(r.Snapshot.Entries) != 1 || r.Action != "prepare" {
			t.Fatal(r, code)
		}
	}
	r, code := invokeLedgerCLI(t, dir, "lookup", "-attempt-key", key, "-request-hash", hash)
	if code != 0 || r.EntryID != 1 || r.Snapshot.Held != 50 {
		t.Fatal(r, code)
	}
	r, code = invokeLedgerCLI(t, dir, "lookup", "-attempt-key", "missing", "-request-hash", hash)
	if code != 1 || r.Error != "state" || len(r.Snapshot.Entries) != 1 || r.Snapshot.Held != 50 {
		t.Fatal(r, code)
	}
	r, code = invokeLedgerCLI(t, dir, "begin", "-call-id", "1")
	if code != 0 || r.Snapshot.Entries[0].State != "sent" {
		t.Fatal(r, code)
	}
	r, code = invokeLedgerCLI(t, dir, "snapshot")
	if code != 0 || r.Snapshot.Held != 50 || r.Snapshot.Entries[0].State != "unknown" {
		t.Fatal(r, code)
	}
	r, code = invokeLedgerCLI(t, dir, "begin", "-call-id", "1")
	if code != 1 || r.Error != "state" || r.Snapshot.Held != 50 {
		t.Fatal("redispatched original attempt", r, code)
	}
	for range 2 {
		r, code = invokeLedgerCLI(t, dir, "settle", "-call-id", "1", "-usage-input", "40")
		if code != 0 || r.Snapshot.Used != 40 || r.Snapshot.Held != 0 {
			t.Fatal(r, code)
		}
	}
	r, code = invokeLedgerCLI(t, dir, "settle", "-call-id", "1", "-usage-input", "41")
	if code != 1 || r.Error != "state" || r.Snapshot.Used != 40 {
		t.Fatal(r, code)
	}
}

func TestLedgerCLIRejectsInvalidFlagsBeforeStorage(t *testing.T) {
	for _, args := range [][]string{
		{"-ledger-action", "prepare"},
		{"-ledger-action", "snapshot", "-call-id", "0"},
		{"-ledger-action", "settle", "-call-id", "1"},
		{"-ledger-action", "begin", "-call-id", "1", "-usage-input", "0"},
		{"-ledger-action", "begin", "-call-id", "65"},
		{"-ledger-action", "snapshot", "-attempt-key", ""},
		{"-ledger-action", "snapshot", "unexpected"},
		{"-ledger-action", "snapshot", "-tenant", strings.Repeat("x", 1024)},
	} {
		dir := t.TempDir()
		costRoot(t, dir)
		var output bytes.Buffer
		handled, code := RunLedgerCLI(append([]string{"-ledger-dir", dir}, args...), &output)
		if !handled || code != 2 || output.Len() > 2048 {
			t.Fatal(args, code, output.Len())
		}
		if _, err := os.Stat(filepath.Join(dir, "ledger.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("bad parameters opened ledger", err)
		}
	}
}

type failedCLIWriter struct{}

func (failedCLIWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestLedgerCLIOutputFailureDoesNotErasePreparation(t *testing.T) {
	dir := t.TempDir()
	costRoot(t, dir)
	args := []string{"-ledger-dir", dir, "-ledger-action", "prepare", "-attempt-key", "attempt-1", "-request-hash", strings.Repeat("a", 64)}
	if handled, code := RunLedgerCLI(args, failedCLIWriter{}); !handled || code != 2 {
		t.Fatal(handled, code)
	}
	var output bytes.Buffer
	if handled, code := RunLedgerCLI(args, &output); !handled || code != 0 {
		t.Fatal(handled, code)
	}
	var r LedgerCLIResult
	if err := json.Unmarshal(output.Bytes(), &r); err != nil || r.EntryID != 1 || r.Snapshot.Held != 50 || len(r.Snapshot.Entries) != 1 {
		t.Fatal(r, err)
	}
	if handled, code := RunLedgerCLI(nil, &output); handled || code != 0 {
		t.Fatal("default demo changed", handled, code)
	}
}
