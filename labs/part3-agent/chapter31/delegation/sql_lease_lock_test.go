package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

func TestActualSQLLeaseExpiryDuringRowLockWait(t *testing.T) {
	db, store := leaseDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	scope := Scope{Tenant: "lease-lock", User: "reader"}
	grant, err := store.Claim(ctx, scope, "report", "a", time.Second)
	if err != nil {
		t.Fatal("initial short lease failed")
	}
	holder, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal("holder transaction failed")
	}
	defer holder.Rollback()
	var epoch int64
	if holder.QueryRowContext(ctx, `SELECT epoch FROM course_delegation_leases WHERE tenant=$1 AND user_id=$2 AND task_id=$3 FOR UPDATE`, scope.Tenant, scope.User, "report").Scan(&epoch) != nil {
		t.Fatal("row lock not held")
	}
	client, err := sql.Open("pgx", os.Getenv("DELEGATION_DSN"))
	if err != nil {
		t.Fatal("client setup failed")
	}
	defer client.Close()
	client.SetMaxOpenConns(1)
	var pid int
	if client.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&pid) != nil {
		t.Fatal("client PID missing")
	}
	result := make(chan error, 1)
	go func() { result <- (SQLLease{client}).Commit(ctx, scope, "report", "a", grant.Epoch, "late-after-lock") }()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	locked := false
	for !locked {
		var waiting string
		if db.QueryRowContext(ctx, "SELECT COALESCE(wait_event_type,'') FROM pg_stat_activity WHERE pid=$1", pid).Scan(&waiting) != nil {
			t.Fatal("wait observation failed")
		}
		locked = waiting == "Lock"
		if !locked {
			select {
			case err := <-result:
				t.Fatalf("commit did not wait: %v", err)
			case <-tick.C:
			case <-ctx.Done():
				t.Fatal("lock wait not observed")
			}
		}
	}
	expired := false
	for !expired {
		if db.QueryRowContext(ctx, "SELECT clock_timestamp() >= $1", grant.Until).Scan(&expired) != nil {
			t.Fatal("database expiry observation failed")
		}
		if !expired {
			select {
			case <-tick.C:
			case <-ctx.Done():
				t.Fatal("database deadline not reached")
			}
		}
	}
	if holder.Rollback() != nil {
		t.Fatal("holder release failed")
	}
	select {
	case err := <-result:
		if !errors.Is(err, ErrHandoff) {
			t.Fatalf("expired commit accepted after actual row lock wait: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("commit did not exit")
	}
	if _, err := store.Result(ctx, scope, "report"); !errors.Is(err, ErrHandoff) {
		t.Fatal("expired result stored")
	}
	t.Log("row_wait_observed=true database_expiry_observed=true late_commit_rejected=true")
}
