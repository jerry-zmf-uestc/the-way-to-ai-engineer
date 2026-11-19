package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRealSQL(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 CLI required for the real SQL experiment")
	}
	for _, mode := range []string{"plan", "transaction"} {
		t.Run(mode, func(t *testing.T) {
			if err := run(mode); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("recovery", func(t *testing.T) {
		binary := filepath.Join(t.TempDir(), "report-sql")
		if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
			t.Fatalf("build: %v: %s", err, out)
		}
		if out, err := exec.Command(binary, "recovery").CombinedOutput(); err != nil {
			t.Fatalf("recovery: %v: %s", err, out)
		}
	})
}
