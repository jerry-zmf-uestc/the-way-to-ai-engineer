package main

import (
	"context"
	"database/sql"
	"errors"
)

var ErrAttemptUnavailable = errors.New("lease: attempt admission unavailable")

type AttemptBudget struct{ Limit, Reserved int }

func (s SQLLease) initAttempts(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS course_delegation_attempt_budgets (
 tenant TEXT NOT NULL,user_id TEXT NOT NULL,task_id TEXT NOT NULL,
 max_attempts INTEGER NOT NULL CHECK(max_attempts BETWEEN 1 AND 128),
 reserved INTEGER NOT NULL CHECK(reserved>=0 AND reserved<=max_attempts),
 PRIMARY KEY(tenant,user_id,task_id))`)
	return err
}
func (s SQLLease) PrepareAttempts(ctx context.Context, scope Scope, task string, limit int) (AttemptBudget, error) {
	if err := ctx.Err(); err != nil {
		return AttemptBudget{}, err
	}
	if s.DB == nil || !leaseIdentity(scope, task, "application") || limit < 1 || limit > 128 {
		return AttemptBudget{}, ErrDenied
	}
	var budget AttemptBudget
	err := s.DB.QueryRowContext(ctx, `INSERT INTO course_delegation_attempt_budgets(tenant,user_id,task_id,max_attempts,reserved)
 VALUES($1,$2,$3,$4,0) ON CONFLICT(tenant,user_id,task_id) DO UPDATE
 SET max_attempts=course_delegation_attempt_budgets.max_attempts
 WHERE course_delegation_attempt_budgets.max_attempts=EXCLUDED.max_attempts
 RETURNING max_attempts,reserved`, scope.Tenant, scope.User, task, limit).Scan(&budget.Limit, &budget.Reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return AttemptBudget{}, ErrAttemptUnavailable
	}
	if err != nil {
		return AttemptBudget{}, err
	}
	if err := ctx.Err(); err != nil {
		return AttemptBudget{}, err
	}
	return budget, nil
}
func (s SQLLease) ReserveAttempt(ctx context.Context, scope Scope, task, owner string, epoch int64) (AttemptBudget, error) {
	if err := ctx.Err(); err != nil {
		return AttemptBudget{}, err
	}
	if s.DB == nil || !leaseIdentity(scope, task, owner) || epoch <= 0 {
		return AttemptBudget{}, ErrDenied
	}
	var budget AttemptBudget
	err := s.DB.QueryRowContext(ctx, `WITH locked AS MATERIALIZED (
 SELECT l.tenant,l.user_id,l.task_id,l.owner,l.epoch,l.lease_until,l.result,b.max_attempts,b.reserved
 FROM course_delegation_leases AS l JOIN course_delegation_attempt_budgets AS b
 USING(tenant,user_id,task_id) WHERE l.tenant=$1 AND l.user_id=$2 AND l.task_id=$3 FOR UPDATE OF l,b)
 UPDATE course_delegation_attempt_budgets AS target SET reserved=target.reserved+1 FROM locked
 WHERE target.tenant=locked.tenant AND target.user_id=locked.user_id AND target.task_id=locked.task_id
 AND locked.owner=$4 AND locked.epoch=$5 AND locked.lease_until>clock_timestamp()
 AND locked.result IS NULL AND locked.reserved<locked.max_attempts
 RETURNING target.max_attempts,target.reserved`, scope.Tenant, scope.User, task, owner, epoch).Scan(&budget.Limit, &budget.Reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return AttemptBudget{}, ErrAttemptUnavailable
	}
	if err != nil {
		return AttemptBudget{}, err
	}
	if err := ctx.Err(); err != nil {
		return AttemptBudget{}, err
	}
	return budget, nil
}
func (s SQLLease) SnapshotAttempts(ctx context.Context, scope Scope, task string) (AttemptBudget, error) {
	if err := ctx.Err(); err != nil {
		return AttemptBudget{}, err
	}
	if s.DB == nil || !leaseIdentity(scope, task, "application") {
		return AttemptBudget{}, ErrDenied
	}
	var budget AttemptBudget
	err := s.DB.QueryRowContext(ctx, `SELECT max_attempts,reserved FROM course_delegation_attempt_budgets WHERE tenant=$1 AND user_id=$2 AND task_id=$3`, scope.Tenant, scope.User, task).Scan(&budget.Limit, &budget.Reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return AttemptBudget{}, ErrAttemptUnavailable
	}
	if err != nil {
		return AttemptBudget{}, err
	}
	if err := ctx.Err(); err != nil {
		return AttemptBudget{}, err
	}
	return budget, nil
}
