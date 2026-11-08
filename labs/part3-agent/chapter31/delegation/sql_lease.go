package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrLeaseUnavailable = errors.New("lease: live or completed claim unavailable")

type SQLLease struct{ DB *sql.DB }
type LeaseGrant struct {
	Epoch int64
	Until time.Time
}

func leaseIdentity(scope Scope, task, owner string) bool {
	for _, value := range []string{scope.Tenant, scope.User, task, owner} {
		if strings.TrimSpace(value) == "" || len(value) > 128 || !utf8.ValidString(value) {
			return false
		}
	}
	return true
}
func (s SQLLease) Init(ctx context.Context) error {
	if s.DB == nil {
		return ErrDenied
	}
	_, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS course_delegation_leases (
 tenant TEXT NOT NULL,user_id TEXT NOT NULL,task_id TEXT NOT NULL,owner TEXT NOT NULL,
 epoch BIGINT NOT NULL CHECK(epoch>0),lease_until TIMESTAMPTZ NOT NULL,result TEXT,
 PRIMARY KEY(tenant,user_id,task_id))`)
	if err != nil {
		return err
	}
	return s.initAttempts(ctx)
}
func (s SQLLease) Claim(ctx context.Context, scope Scope, task, owner string, ttl time.Duration) (LeaseGrant, error) {
	if err := ctx.Err(); err != nil {
		return LeaseGrant{}, err
	}
	if s.DB == nil || !leaseIdentity(scope, task, owner) || ttl < time.Second || ttl > time.Minute {
		return LeaseGrant{}, ErrDenied
	}
	var grant LeaseGrant
	err := s.DB.QueryRowContext(ctx, `INSERT INTO course_delegation_leases(tenant,user_id,task_id,owner,epoch,lease_until)
 VALUES($1,$2,$3,$4,1,clock_timestamp()+$5::double precision*interval '1 second')
 ON CONFLICT(tenant,user_id,task_id) DO UPDATE SET owner=EXCLUDED.owner,
 epoch=course_delegation_leases.epoch+1,lease_until=clock_timestamp()+$5::double precision*interval '1 second'
 WHERE course_delegation_leases.lease_until<=clock_timestamp()
 AND course_delegation_leases.result IS NULL AND course_delegation_leases.epoch<9223372036854775807
 RETURNING epoch,lease_until`, scope.Tenant, scope.User, task, owner, ttl.Seconds()).Scan(&grant.Epoch, &grant.Until)
	if errors.Is(err, sql.ErrNoRows) {
		return LeaseGrant{}, ErrLeaseUnavailable
	}
	if err != nil {
		return LeaseGrant{}, err
	}
	if err := ctx.Err(); err != nil {
		return LeaseGrant{}, err
	}
	return grant, nil
}
func (s SQLLease) Commit(ctx context.Context, scope Scope, task, owner string, epoch int64, result string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.DB == nil || !leaseIdentity(scope, task, owner) || epoch <= 0 || strings.TrimSpace(result) == "" || len(result) > 4096 || !utf8.ValidString(result) {
		return ErrDenied
	}
	var accepted int
	err := s.DB.QueryRowContext(ctx, `WITH locked AS MATERIALIZED (
 SELECT tenant,user_id,task_id,owner,epoch,lease_until,result FROM course_delegation_leases
 WHERE tenant=$1 AND user_id=$2 AND task_id=$3 FOR UPDATE)
 UPDATE course_delegation_leases AS target SET result=$6 FROM locked
 WHERE target.tenant=locked.tenant AND target.user_id=locked.user_id AND target.task_id=locked.task_id
 AND locked.owner=$4 AND locked.epoch=$5 AND locked.lease_until>clock_timestamp()
 AND locked.result IS NULL RETURNING 1`, scope.Tenant, scope.User, task, owner, epoch, result).Scan(&accepted)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHandoff
	}
	if err != nil {
		return err
	}
	return ctx.Err()
}
func (s SQLLease) Result(ctx context.Context, scope Scope, task string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.DB == nil || !leaseIdentity(scope, task, "reader") {
		return "", ErrDenied
	}
	var result sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT result FROM course_delegation_leases WHERE tenant=$1 AND user_id=$2 AND task_id=$3`, scope.Tenant, scope.User, task).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrHandoff
	}
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !result.Valid {
		return "", ErrHandoff
	}
	if len(result.String) > 4096 || !utf8.ValidString(result.String) {
		return "", ErrResult
	}
	return result.String, nil
}
