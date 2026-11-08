package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteRecoveryAcrossProcesses(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI required")
	}
	binary := filepath.Join(t.TempDir(), "report-flow")
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	for _, fault := range []string{"crash-before-result", "crash-after-result"} {
		t.Run(fault, func(t *testing.T) {
			db := filepath.Join(t.TempDir(), "reports.db")
			start := func(mode string) (*exec.Cmd, string) {
				t.Helper()
				cmd := exec.CommandContext(ctx, binary, "-port", "0", "-db", db, "-fault", mode)
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				cmd.Stderr = os.Stderr
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
				line, err := bufio.NewReader(stdout).ReadString('\n')
				if err != nil || !strings.HasPrefix(line, "listening=http://") {
					t.Fatalf("start: %q, %v", line, err)
				}
				return cmd, strings.TrimPrefix(strings.Fields(line)[0], "listening=")
			}
			call := func(base, method, path string, body io.Reader, want int, dest any) {
				t.Helper()
				req, err := http.NewRequestWithContext(ctx, method, base+path, body)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Idempotency-Key", "same 'key")
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != want {
					t.Fatalf("%s: got %d want %d", path, resp.StatusCode, want)
				}
				if dest != nil {
					if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
						t.Fatal(err)
					}
				}
			}
			cmd, base := start(fault)
			var original job
			call(base, "POST", "/exports", strings.NewReader(`{"format":"csv"}`), 202, &original)
			if original.ID != "export-1" || original.Status != "queued" {
				t.Fatalf("accepted job: %+v", original)
			}
			call(base, "POST", "/lab/resume", nil, 204, nil)
			var exit *exec.ExitError
			if err := cmd.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 73 {
				t.Fatalf("expected injected exit: %v", err)
			}
			s := sqliteStore{path: db}
			checkDisk := func(want string) {
				t.Helper()
				got, err := s.sql("SELECT result_writes, acked FROM report_flow_jobs JOIN report_flow_outbox USING(id)")
				if err != nil || strings.TrimSpace(string(got)) != want {
					t.Fatalf("SQLite state: %q %v, want %s", got, err, want)
				}
			}
			if fault == "crash-before-result" {
				checkDisk(`[{"result_writes":0,"acked":0}]`)
			} else {
				checkDisk(`[{"result_writes":1,"acked":0}]`)
			}
			_, base = start("none")
			var restored job
			var metrics struct {
				counters
				Recovered int `json:"recovered_pending"`
			}
			for deadline := time.Now().Add(4 * time.Second); ; {
				call(base, "GET", "/exports/"+original.ID, nil, 200, &restored)
				call(base, "GET", "/metrics", nil, 200, &metrics)
				if restored.Status == "succeeded" && metrics.Deliveries == 1 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("worker did not recover")
				}
				time.Sleep(10 * time.Millisecond)
			}
			wantDuplicates := 0
			if fault == "crash-after-result" {
				wantDuplicates = 1
			}
			if restored.Result != "report_id,total\nexport-1,42\n" || restored.ResultWrites != 1 ||
				metrics.Recovered != 1 || metrics.Duplicates != wantDuplicates {
				t.Fatalf("recovered job=%+v metrics=%+v", restored, metrics)
			}
			checkDisk(`[{"result_writes":1,"acked":1}]`)
			var replay job
			call(base, "POST", "/exports", strings.NewReader(`{"format":"csv"}`), 202, &replay)
			if replay.ID != original.ID || replay.ResultWrites != 1 {
				t.Fatalf("idempotent replay: %+v", replay)
			}
		})
	}
}
