package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSQLLeaseRejectsInvalidBeforeDatabase(t *testing.T) {
	s := SQLLease{}
	scope := Scope{Tenant: "t", User: "u"}
	if _, err := s.Claim(context.Background(), scope, "task", "a", time.Second); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	if err := s.Commit(context.Background(), scope, "task", "a", 0, "value"); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Claim(ctx, scope, "task", "a", time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if leaseIdentity(Scope{}, "task", "a") || leaseIdentity(scope, "task", string([]byte{0xff})) {
		t.Fatal("invalid identity")
	}
}
func leaseDatabase(t *testing.T) (*sql.DB, SQLLease) {
	t.Helper()
	if os.Getenv("DELEGATION_LEASE_E2E") != "1" {
		t.Skip("explicit isolated SQL lease only")
	}
	if os.Getenv("DELEGATION_TEST_DATABASE") != "1" {
		t.Fatal("fixture database confirmation required")
	}
	db, err := sql.Open("pgx", os.Getenv("DELEGATION_DSN"))
	if err != nil {
		t.Fatal("database configuration failed")
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var name string
	if db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name) != nil || name != "delegation_lab" {
		t.Fatal("isolated delegation_lab required")
	}
	s := SQLLease{db}
	if s.Init(ctx) != nil {
		t.Fatal("lease schema setup failed")
	}
	return db, s
}
func TestSQLLeaseProcessHelper(t *testing.T) {
	if os.Getenv("DELEGATION_LEASE_HELPER") != "1" {
		return
	}
	_, s := leaseDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	scope := Scope{Tenant: "lease-process", User: "reader"}
	grant, err := s.Claim(ctx, scope, "report", "b", time.Minute)
	if err != nil {
		t.Fatal("new owner claim failed")
	}
	raw, _ := json.Marshal(struct {
		Epoch int64
		PID   int
	}{grant.Epoch, os.Getpid()})
	os.Stdout.Write(append(raw, '\n'))
	ready := bufio.NewScanner(os.Stdin)
	if !ready.Scan() || ready.Text() != "commit" {
		t.Fatal("commit signal missing")
	}
	if err := s.Commit(ctx, scope, "report", "b", grant.Epoch, "current-B"); err != nil {
		t.Fatal("new owner commit failed")
	}
	os.Exit(0)
}
func TestActualSQLLeaseRejectsOldProcessResult(t *testing.T) {
	db, s := leaseDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	scope := Scope{Tenant: "lease-process", User: "reader"}
	first, err := s.Claim(ctx, scope, "report", "a", time.Minute)
	if err != nil || first.Epoch != 1 {
		t.Fatal("initial lease failed")
	}
	if _, err := s.Claim(ctx, scope, "report", "b", time.Minute); !errors.Is(err, ErrLeaseUnavailable) {
		t.Fatal("live lease stolen")
	}
	// Test-owned expiry injection; no sleep or claim of a network partition.
	if _, err := db.ExecContext(ctx, `UPDATE course_delegation_leases SET lease_until=clock_timestamp()-interval '1 second' WHERE tenant=$1 AND user_id=$2 AND task_id=$3`, scope.Tenant, scope.User, "report"); err != nil {
		t.Fatal("controlled expiry failed")
	}
	if err := s.Commit(ctx, scope, "report", "a", first.Epoch, "expired-A"); !errors.Is(err, ErrHandoff) {
		t.Fatal("expired result accepted")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestSQLLeaseProcessHelper$")
	cmd.Env = append(os.Environ(), "DELEGATION_LEASE_HELPER=1")
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	control, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal("separate owner start failed")
	}
	waited := false
	defer func() {
		if !waited {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || len(line) > 1024 {
		t.Fatal("separate owner response failed")
	}
	var second struct {
		Epoch int64
		PID   int
	}
	if json.Unmarshal([]byte(line), &second) != nil || second.Epoch != 2 || second.PID == os.Getpid() {
		t.Fatal("independent epoch2 owner not observed")
	}
	if err := s.Commit(ctx, scope, "report", "b", first.Epoch, "old-ticket"); !errors.Is(err, ErrHandoff) {
		t.Fatal("old epoch accepted under current owner")
	}
	if err := s.Commit(ctx, scope, "report", "a", second.Epoch, "wrong-owner"); !errors.Is(err, ErrHandoff) {
		t.Fatal("wrong owner accepted under current epoch")
	}
	if _, err := control.Write([]byte("commit\n")); err != nil {
		t.Fatal("commit signal failed")
	}
	control.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal("separate owner commit failed")
	}
	waited = true
	if err := s.Commit(ctx, scope, "report", "a", first.Epoch, "late-A"); !errors.Is(err, ErrHandoff) {
		t.Fatal("old process result accepted")
	}
	result, err := s.Result(ctx, scope, "report")
	if err != nil || result != "current-B" {
		t.Fatal("current result overwritten")
	}
	if _, err := s.Result(ctx, Scope{Tenant: scope.Tenant, User: "other"}, "report"); !errors.Is(err, ErrHandoff) {
		t.Fatal("scope leak")
	}
	if _, err := db.ExecContext(ctx, `UPDATE course_delegation_leases SET lease_until=clock_timestamp()-interval '1 second' WHERE tenant=$1 AND user_id=$2 AND task_id=$3`, scope.Tenant, scope.User, "report"); err != nil {
		t.Fatal("completed expiry injection failed")
	}
	if _, err := s.Claim(ctx, scope, "report", "c", time.Minute); !errors.Is(err, ErrLeaseUnavailable) {
		t.Fatal("completed task reclaimed")
	}
	t.Log("processes=2 epoch=1->2 old_commit_rejected=true current_result=B scope_isolated=true expiry=controlled")
}
