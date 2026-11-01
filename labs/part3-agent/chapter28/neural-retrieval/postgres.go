package main

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const neuralSQL = `SELECT id,tenant,version,space,model_revision,1-(embedding <=> $5::vector) AS score
 FROM neural_items WHERE tenant=$1 AND version=$2 AND space=$3 AND model_revision=$4
 ORDER BY embedding <=> $5::vector LIMIT $6`

func vectorLiteral(vec []float64) string {
	parts := make([]string, len(vec))
	for n, v := range vec {
		parts[n] = strconv.FormatFloat(v, 'g', 17, 64)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// SeedEncoded is trusted fixture preparation, not a model tool. One isolated session owns its temp table.
func SeedEncoded(ctx context.Context, conn *pgx.Conn, docs []Document, batch Batch) error {
	if len(docs) == 0 || len(docs) > 32 || len(batch.Vectors) < len(docs) {
		return ErrInput
	}
	if err := Validate(batch, len(batch.Vectors)); err != nil {
		return err
	}
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
	if version != "0.8.7" {
		return ErrInput
	}
	if _, err := conn.Exec(ctx, `CREATE TEMP TABLE neural_items(id text PRIMARY KEY,tenant text NOT NULL,version text NOT NULL,space text NOT NULL,model_revision text NOT NULL,embedding vector(384) NOT NULL)`); err != nil {
		return err
	}
	for n, doc := range docs {
		if doc.ID == "" || len(doc.ID) > 128 || doc.Tenant == "" || doc.Version == "" || batch.Truncated[n] {
			return ErrInput
		}
		if _, err := conn.Exec(ctx, `INSERT INTO neural_items VALUES($1,$2,$3,$4,$5,$6::vector)`, doc.ID, doc.Tenant, doc.Version, batch.Space, batch.Revision, vectorLiteral(batch.Vectors[n])); err != nil {
			return err
		}
	}
	_, err := conn.Exec(ctx, `CREATE INDEX neural_items_hnsw ON neural_items USING hnsw(embedding vector_cosine_ops);ANALYZE neural_items`)
	return err
}
func SQLDense(ctx context.Context, conn *pgx.Conn, batch Batch, queryIndex int, tenant, version string, k int, ann bool) (hits []Hit, plan string, err error) {
	if err := Validate(batch, len(batch.Vectors)); err != nil {
		return nil, "", err
	}
	if queryIndex < 0 || queryIndex >= len(batch.Vectors) || tenant == "" || version == "" || len(tenant) > 128 || len(version) > 128 || k < 1 || k > 20 || batch.Truncated[queryIndex] {
		return nil, "", ErrInput
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, "", err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cleanupErr := tx.Rollback(cleanup)
		if !errors.Is(cleanupErr, pgx.ErrTxClosed) {
			err = errors.Join(err, cleanupErr)
		}
	}()
	settings := `SET LOCAL enable_indexscan=off;SET LOCAL enable_bitmapscan=off`
	if ann {
		settings = `SET LOCAL enable_seqscan=off;SET LOCAL enable_bitmapscan=off;SET LOCAL enable_sort=off;SET LOCAL hnsw.ef_search=100;SET LOCAL hnsw.iterative_scan=strict_order`
	}
	if _, err = tx.Exec(ctx, settings); err != nil {
		return nil, "", err
	}
	args := []any{tenant, version, batch.Space, batch.Revision, vectorLiteral(batch.Vectors[queryIndex]), k}
	explain, err := tx.Query(ctx, "EXPLAIN "+neuralSQL, args...)
	if err != nil {
		return nil, "", err
	}
	for explain.Next() {
		var line string
		if err = explain.Scan(&line); err != nil {
			explain.Close()
			return nil, "", err
		}
		plan += line + "\n"
	}
	err = explain.Err()
	explain.Close()
	if err != nil {
		return nil, plan, err
	}
	if (ann && !strings.Contains(plan, "neural_items_hnsw")) || (!ann && !strings.Contains(plan, "Seq Scan")) {
		return nil, plan, ErrInput
	}
	rows, err := tx.Query(ctx, neuralSQL, args...)
	if err != nil {
		return nil, plan, err
	}
	for rows.Next() {
		var hit Hit
		var gotTenant, gotVersion, gotSpace, gotRevision string
		if err = rows.Scan(&hit.ID, &gotTenant, &gotVersion, &gotSpace, &gotRevision, &hit.Score); err != nil {
			rows.Close()
			return nil, plan, err
		}
		if hit.ID == "" || len(hits) >= k || gotTenant != tenant || gotVersion != version || gotSpace != batch.Space || gotRevision != batch.Revision || math.IsNaN(hit.Score) || math.IsInf(hit.Score, 0) {
			rows.Close()
			return nil, plan, ErrInput
		}
		hits = append(hits, hit)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, plan, err
	}
	if err = ctx.Err(); err != nil {
		return nil, plan, err
	}
	err = tx.Commit(ctx)
	return hits, plan, err
}
