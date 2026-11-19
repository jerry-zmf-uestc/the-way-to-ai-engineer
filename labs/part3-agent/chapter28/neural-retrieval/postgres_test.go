package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestActualEncoderToPgvector(t *testing.T) {
	if os.Getenv("EMBEDDING_E2E") != "1" || os.Getenv("VECTOR_E2E") != "1" {
		t.Skip("explicit joint encoder/vector E2E")
	}
	if os.Getenv("VECTOR_TEST_DATABASE") != "1" || os.Getenv("VECTOR_DSN") == "" {
		t.Fatal("isolated vector database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	docs, queries, labels := fixtures()
	texts := []string{}
	for _, d := range docs {
		texts = append(texts, d.Text)
	}
	for _, q := range queries {
		texts = append(texts, q.Text)
	}
	batch, err := Encode(ctx, os.Getenv("EMBEDDING_PYTHON"), os.Getenv("EMBEDDING_ASSETS"), texts)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, os.Getenv("VECTOR_DSN"))
	if err != nil {
		t.Fatal("local DB connection failed")
	}
	defer conn.Close(context.Background())
	if err = SeedEncoded(ctx, conn, docs, batch); err != nil {
		t.Fatal(err)
	}
	known, totalRecall := 0, 0.0
	for n, q := range queries {
		baseline, err := Dense(docs, batch.Vectors[:len(docs)], batch.Vectors[len(docs)+n], "demo", "v1", 1)
		if err != nil {
			t.Fatal(err)
		}
		exact, ep, err := SQLDense(ctx, conn, batch, len(docs)+n, "demo", "v1", 1, false)
		if err != nil || len(exact) != 1 || exact[0].ID != baseline[0].ID {
			t.Fatal(q.ID, exact, err)
		}
		ann, ap, err := SQLDense(ctx, conn, batch, len(docs)+n, "demo", "v1", 1, true)
		if err != nil || len(ann) != 1 {
			t.Fatal(q.ID, ann, err)
		}
		score, covered, err := Recall(labels[n], ann)
		if err != nil {
			t.Fatal(err)
		}
		if covered {
			known++
			totalRecall += score
		}
		for _, hit := range ann {
			if hit.ID == "other-user" || hit.ID == "old-timeout" {
				t.Fatal("scope mismatch")
			}
		}
		t.Logf("case=%s go=%s sql=%s ann=%s recall_known=%t ann_semantic_recall@1=%.0f exact_plan=%t ann_plan=%t", q.ID, baseline[0].ID, exact[0].ID, ann[0].ID, covered, score, len(ep) > 0, len(ap) > 0)
	}
	if known != 5 {
		t.Fatal(known)
	}
	t.Logf("joint_actual_dim=%d space=%s labelled_cases=%d semantic_recall@1=%.2f", batch.Dimension, batch.Space, known, totalRecall/float64(known))
	bad := batch
	bad.Space = "wrong"
	if _, _, err = SQLDense(ctx, conn, bad, len(docs), "demo", "v1", 1, true); !errors.Is(err, ErrInput) {
		t.Fatal("wrong space accepted")
	}
	stopped, stop := context.WithCancel(ctx)
	stop()
	if _, _, err = SQLDense(stopped, conn, batch, len(docs), "demo", "v1", 1, true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
