package main

import (
	"context"
	"database/sql"
	"errors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"time"
)

type SQLStore struct{ DB *sql.DB }

const ddl = `CREATE TABLE IF NOT EXISTS course_memory_entries (
 tenant TEXT NOT NULL, user_id TEXT NOT NULL, memory_key TEXT NOT NULL,
 value TEXT NOT NULL, source TEXT NOT NULL, revision BIGINT NOT NULL CHECK (revision>0),
 expires_at TIMESTAMPTZ NOT NULL, deleted BOOLEAN NOT NULL,
 PRIMARY KEY(tenant,user_id,memory_key)
)`

func (s SQLStore) Init(ctx context.Context) error { _, err := s.DB.ExecContext(ctx, ddl); return err }
func (s SQLStore) Peek(ctx context.Context, scope Scope, key string) (Record, error) {
	var r Record
	r.Key = key
	err := s.DB.QueryRowContext(ctx, `SELECT value,source,revision,expires_at,deleted FROM course_memory_entries WHERE tenant=$1 AND user_id=$2 AND memory_key=$3`, scope.Tenant, scope.User, key).Scan(&r.Value, &r.Source, &r.Revision, &r.ExpiresAt, &r.Deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return r, err
}

// CAS使用参数化条件更新，主键始终包含可信tenant/user/key。
func (s SQLStore) CAS(ctx context.Context, scope Scope, r Record, expected int64) (Record, error) {
	var row *sql.Row
	if expected == 0 {
		row = s.DB.QueryRowContext(ctx, `INSERT INTO course_memory_entries(tenant,user_id,memory_key,value,source,revision,expires_at,deleted) VALUES($1,$2,$3,$4,$5,1,$6,$7) ON CONFLICT(tenant,user_id,memory_key) DO NOTHING RETURNING revision`, scope.Tenant, scope.User, r.Key, r.Value, r.Source, r.ExpiresAt, r.Deleted)
	} else {
		row = s.DB.QueryRowContext(ctx, `UPDATE course_memory_entries SET value=$4,source=$5,revision=revision+1,expires_at=$6,deleted=$7 WHERE tenant=$1 AND user_id=$2 AND memory_key=$3 AND revision=$8 RETURNING revision`, scope.Tenant, scope.User, r.Key, r.Value, r.Source, r.ExpiresAt, r.Deleted, expected)
	}
	if err := row.Scan(&r.Revision); errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrConflict
	} else if err != nil {
		return Record{}, err
	}
	return r, nil
}
func (s SQLStore) Active(ctx context.Context, scope Scope, now time.Time) ([]Record, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT memory_key,value,source,revision,expires_at,deleted FROM course_memory_entries WHERE tenant=$1 AND user_id=$2 AND NOT deleted AND expires_at>$3 ORDER BY memory_key LIMIT 128`, scope.Tenant, scope.User, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.Key, &r.Value, &r.Source, &r.Revision, &r.ExpiresAt, &r.Deleted); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
