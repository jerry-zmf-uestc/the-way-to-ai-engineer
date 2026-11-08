package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSQLAttemptBudgetInvalidBeforeDatabase(t *testing.T) {
	s := SQLLease{}
	scope := Scope{Tenant: "t", User: "u"}
	if _, err := s.PrepareAttempts(context.Background(), scope, "task", 129); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	if _, err := s.ReserveAttempt(context.Background(), scope, "task", "a", 0); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.SnapshotAttempts(ctx, scope, "task"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestSQLAttemptBudgetProcessHelper(t *testing.T) {
	if os.Getenv("DELEGATION_ATTEMPT_HELPER") != "1" {
		return
	}
	_, s := leaseDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	scope := Scope{Tenant: "lease-budget", User: "reader"}
	grant, err := s.Claim(ctx, scope, "report", "b", time.Minute)
	if err != nil || grant.Epoch != 2 {
		t.Fatal("takeover failed")
	}
	inherited, err := s.PrepareAttempts(ctx, scope, "report", 2)
	if err != nil || inherited != (AttemptBudget{2, 1}) {
		t.Fatal("original budget lost")
	}
	if _, err := s.PrepareAttempts(ctx, scope, "report", 3); !errors.Is(err, ErrAttemptUnavailable) {
		t.Fatal("budget enlarged")
	}
	budget, err := s.ReserveAttempt(ctx, scope, "report", "b", grant.Epoch)
	if err != nil || budget != (AttemptBudget{2, 2}) {
		t.Fatal("remaining reservation failed")
	}
	if _, err := s.ReserveAttempt(ctx, scope, "report", "b", grant.Epoch); !errors.Is(err, ErrAttemptUnavailable) {
		t.Fatal("exhausted budget accepted")
	}
	if err := s.Commit(ctx, scope, "report", "b", grant.Epoch, "current-B"); err != nil {
		t.Fatal("current result rejected")
	}
	data, _ := json.Marshal(struct {
		Epoch  int64
		PID    int
		Budget AttemptBudget
	}{grant.Epoch, os.Getpid(), budget})
	os.Stdout.Write(append(data, '\n'))
	os.Exit(0)
}
func TestActualSQLAttemptBudgetSurvivesTakeover(t *testing.T) {
	db, s := leaseDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	scope := Scope{Tenant: "lease-budget", User: "reader"}
	if budget, err := s.PrepareAttempts(ctx, scope, "report", 2); err != nil || budget != (AttemptBudget{2, 0}) {
		t.Fatal("original policy not stored")
	}
	grant, err := s.Claim(ctx, scope, "report", "a", time.Minute)
	if err != nil {
		t.Fatal("initial lease failed")
	}
	if budget, err := s.ReserveAttempt(ctx, scope, "report", "a", grant.Epoch); err != nil || budget != (AttemptBudget{2, 1}) {
		t.Fatal("first reservation failed")
	}
	if _, err := db.ExecContext(ctx, `UPDATE course_delegation_leases SET lease_until=clock_timestamp()-interval '1 second' WHERE tenant=$1 AND user_id=$2 AND task_id=$3`, scope.Tenant, scope.User, "report"); err != nil {
		t.Fatal("expiry injection failed")
	}
	if _, err := s.ReserveAttempt(ctx, scope, "report", "a", grant.Epoch); !errors.Is(err, ErrAttemptUnavailable) {
		t.Fatal("expired owner reserved")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestSQLAttemptBudgetProcessHelper$")
	cmd.Env = append(os.Environ(), "DELEGATION_ATTEMPT_HELPER=1")
	data, err := cmd.Output()
	if err != nil || len(data) > 1024 {
		t.Fatal("new process failed")
	}
	var record struct {
		Epoch  int64
		PID    int
		Budget AttemptBudget
	}
	if json.Unmarshal(data, &record) != nil || record.Epoch != 2 || record.PID == os.Getpid() || record.Budget != (AttemptBudget{2, 2}) {
		t.Fatal("new process budget invalid")
	}
	if budget, err := s.SnapshotAttempts(ctx, scope, "report"); err != nil || budget != (AttemptBudget{2, 2}) {
		t.Fatal("reservation lost")
	}
	if _, err := s.PrepareAttempts(ctx, Scope{Tenant: scope.Tenant, User: "other"}, "report", 2); err != nil {
		t.Fatal("other isolated policy setup failed")
	}
	if budget, err := s.SnapshotAttempts(ctx, scope, "report"); err != nil || budget.Reserved != 2 {
		t.Fatal("other user changed budget")
	}
	t.Log("processes=2 original_limit=2 inherited_reserved=1 final_reserved=2 old_owner_rejected=true enlarged_limit_rejected=true")
}

func TestActualSQLAttemptReservationsDoNotOversubscribe(t *testing.T) {
	_, s := leaseDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	scope := Scope{Tenant: "lease-concurrent-budget", User: "reader"}
	if _, err := s.PrepareAttempts(ctx, scope, "report", 1); err != nil {
		t.Fatal("budget setup failed")
	}
	grant, err := s.Claim(ctx, scope, "report", "a", time.Minute)
	if err != nil {
		t.Fatal("lease setup failed")
	}
	start := make(chan struct{})
	done := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func() { <-start; _, err := s.ReserveAttempt(ctx, scope, "report", "a", grant.Epoch); done <- err }()
	}
	close(start)
	accepted, rejected := 0, 0
	for n := 0; n < 2; n++ {
		select {
		case err := <-done:
			if err == nil {
				accepted++
			} else if errors.Is(err, ErrAttemptUnavailable) {
				rejected++
			} else {
				t.Fatal("unexpected reservation error")
			}
		case <-ctx.Done():
			t.Fatal("reservations did not finish")
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatal("concurrent allowance exceeded", accepted, rejected)
	}
	if budget, err := s.SnapshotAttempts(ctx, scope, "report"); err != nil || budget != (AttemptBudget{1, 1}) {
		t.Fatal("budget total wrong")
	}
	t.Log("concurrent_reservations=2 accepted=1 rejected=1 final_reserved=1")
}
