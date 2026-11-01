package main

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const extensionVersion = "0.8.7"
const searchSQL = `SELECT id,body,tenant,space,version,embedding <=> $4::vector AS distance
 FROM vector_items WHERE tenant=$1 AND space=$2 AND version=$3
 ORDER BY embedding <=> $4::vector LIMIT $5`

// Seed belongs to the fixture application, only after an explicit isolated-DB guard.
func Seed(ctx context.Context, conn *pgx.Conn, rows []Row) error {
	var database string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
		return err
	}
	if database != "vector_lab" {
		return ErrInput
	}
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
		return err
	}
	var version string
	if err := conn.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname='vector'").Scan(&version); err != nil {
		return err
	}
	if version != extensionVersion {
		return ErrInput
	}
	if _, err := conn.Exec(ctx, `CREATE TEMP TABLE vector_items(id text PRIMARY KEY,tenant text NOT NULL,space text NOT NULL,version text NOT NULL,body text NOT NULL,embedding vector(3) NOT NULL)`); err != nil {
		return err
	}
	for _, row := range rows {
		if !valid(row.Scope, row.Vector, 1) || row.ID == "" || len(row.Text) > 4096 {
			return ErrInput
		}
		if _, err := conn.Exec(ctx, `INSERT INTO vector_items VALUES($1,$2,$3,$4,$5,$6::vector)`, row.ID, row.Scope.Tenant, row.Scope.Space, row.Scope.Version, row.Text, literal(row.Vector)); err != nil {
			return err
		}
	}
	_, err := conn.Exec(ctx, `CREATE INDEX vector_items_hnsw ON vector_items USING hnsw(embedding vector_cosine_ops);ANALYZE vector_items`)
	return err
}

// Search owns one connection transaction. Settings are local, output is authorized in SQL and checked again.
func Search(ctx context.Context, conn *pgx.Conn, scope Scope, query Vector, k int, approx bool) (hits []Hit, plan string, err error) {
	if !valid(scope, query, k) {
		return nil, "", ErrInput
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, "", err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		rollbackErr := tx.Rollback(cleanup)
		if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	settings := `SET LOCAL enable_indexscan=off;SET LOCAL enable_bitmapscan=off`
	if approx {
		settings = `SET LOCAL enable_seqscan=off;SET LOCAL enable_bitmapscan=off;SET LOCAL enable_sort=off;SET LOCAL hnsw.ef_search=100;SET LOCAL hnsw.iterative_scan=strict_order`
	}
	if _, err = tx.Exec(ctx, settings); err != nil {
		return nil, "", err
	}
	args := []any{scope.Tenant, scope.Space, scope.Version, literal(query), k}
	explanation, err := tx.Query(ctx, "EXPLAIN "+searchSQL, args...)
	if err != nil {
		return nil, "", err
	}
	lines := []string{}
	for explanation.Next() {
		var line string
		if err = explanation.Scan(&line); err != nil {
			explanation.Close()
			return nil, "", err
		}
		lines = append(lines, line)
	}
	err = explanation.Err()
	explanation.Close()
	if err != nil {
		return nil, "", err
	}
	plan = strings.Join(lines, "\n")
	if approx && !strings.Contains(plan, "vector_items_hnsw") {
		return nil, plan, ErrInput
	}
	if !approx && !strings.Contains(plan, "Seq Scan") {
		return nil, plan, ErrInput
	}
	result, err := tx.Query(ctx, searchSQL, args...)
	if err != nil {
		return nil, plan, err
	}
	for result.Next() {
		var hit Hit
		if err = result.Scan(&hit.ID, &hit.Text, &hit.Scope.Tenant, &hit.Scope.Space, &hit.Scope.Version, &hit.Distance); err != nil {
			result.Close()
			return nil, plan, err
		}
		if len(hits) >= k || hit.ID == "" || len(hit.Text) > 4096 || hit.Scope != scope || math.IsNaN(hit.Distance) || math.IsInf(hit.Distance, 0) {
			result.Close()
			return nil, plan, ErrInput
		}
		hits = append(hits, hit)
	}
	err = result.Err()
	result.Close()
	if err != nil {
		return nil, plan, err
	}
	if err = ctx.Err(); err != nil {
		return nil, plan, err
	}
	err = tx.Commit(ctx)
	return hits, plan, err
}
