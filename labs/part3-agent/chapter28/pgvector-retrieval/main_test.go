package main

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestExactScopeSpaceVersionAndInvalidInputs(t *testing.T) {
	rows := fixtures()
	scope := Scope{Tenant: "demo", Space: "fixture-v1", Version: "v1"}
	query := Vector{1, .23, .13}
	hits, err := Exact(context.Background(), rows, scope, query, 5)
	if err != nil || len(hits) != 5 {
		t.Fatal(hits, err)
	}
	byID := map[string]Row{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, hit := range hits {
		if byID[hit.ID].Scope != scope {
			t.Fatal(hit)
		}
	}
	if _, err = Exact(context.Background(), rows, scope, Vector{}, 5); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	if _, err = Exact(context.Background(), rows, scope, Vector{math.NaN(), 1, 1}, 5); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Exact(ctx, rows, scope, query, 5); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = Recall(hits, append(hits, hits[0])); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
}
func TestActualPgvectorExactAndHNSW(t *testing.T) {
	if os.Getenv("VECTOR_E2E") != "1" {
		t.Skip("explicit vector E2E")
	}
	if os.Getenv("VECTOR_TEST_DATABASE") != "1" || os.Getenv("VECTOR_DSN") == "" {
		t.Fatal("isolated database guard required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, os.Getenv("VECTOR_DSN"))
	if err != nil {
		t.Fatal("vector DB connection failed")
	}
	defer conn.Close(context.Background())
	rows := fixtures()
	if err = Seed(ctx, conn, rows); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Tenant: "demo", Space: "fixture-v1", Version: "v1"}
	query := Vector{1, .23, .13}
	baseline, err := Exact(ctx, rows, scope, query, 10)
	if err != nil {
		t.Fatal(err)
	}
	exact, exactPlan, err := Search(ctx, conn, scope, query, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	match, err := Recall(baseline, exact)
	if err != nil || match != 1 {
		t.Fatal(match, err)
	}
	approx, approxPlan, err := Search(ctx, conn, scope, query, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	match, err = Recall(exact, approx)
	if err != nil || match < .8 {
		t.Fatal(match, err)
	}
	allowed := map[string]Row{}
	for _, row := range rows {
		allowed[row.ID] = row
	}
	for _, hit := range approx {
		if allowed[hit.ID].Scope != scope {
			t.Fatal("cross scope result")
		}
	}
	t.Logf("extension=%s exact_plan_seq=%t ann_plan_hnsw=%t hits=%d recall_vs_exact=%.2f", extensionVersion, strings.Contains(exactPlan, "Seq Scan"), strings.Contains(approxPlan, "vector_items_hnsw"), len(approx), match)
	stopped, stop := context.WithCancel(ctx)
	stop()
	if _, _, err = Search(stopped, conn, scope, query, 5, true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
