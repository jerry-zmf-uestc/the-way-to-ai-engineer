package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

var ErrInput = errors.New("invalid vector retrieval contract")

type Scope struct{ Tenant, Space, Version string }
type Vector [3]float64
type Row struct {
	ID     string
	Scope  Scope
	Text   string
	Vector Vector
}
type Hit struct {
	ID, Text string
	Distance float64
	Scope    Scope
}

func valid(scope Scope, query Vector, k int) bool {
	if scope.Tenant == "" || scope.Space == "" || scope.Version == "" || len(scope.Tenant) > 128 || len(scope.Space) > 128 || len(scope.Version) > 128 || k < 1 || k > 20 {
		return false
	}
	norm := 0.0
	for _, v := range query {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1_000_000 {
			return false
		}
		norm += v * v
	}
	return norm > 0
}
func literal(vector Vector) string {
	values := make([]string, len(vector))
	for n, v := range vector {
		values[n] = strconv.FormatFloat(v, 'g', 17, 64)
	}
	return "[" + strings.Join(values, ",") + "]"
}
func distance(a, b Vector) float64 {
	dot, aa, bb := 0.0, 0.0, 0.0
	for n, v := range a {
		dot += v * b[n]
		aa += v * v
		bb += b[n] * b[n]
	}
	return 1 - dot/(math.Sqrt(aa)*math.Sqrt(bb))
}
func Exact(ctx context.Context, rows []Row, scope Scope, query Vector, k int) ([]Hit, error) {
	if !valid(scope, query, k) || len(rows) > 4096 {
		return nil, ErrInput
	}
	hits := []Hit{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !valid(row.Scope, row.Vector, 1) || row.ID == "" || len(row.ID) > 128 || len(row.Text) > 4096 {
			return nil, ErrInput
		}
		if row.Scope == scope {
			hits = append(hits, Hit{ID: row.ID, Text: row.Text, Scope: row.Scope, Distance: distance(query, row.Vector)})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Distance == hits[j].Distance {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Distance < hits[j].Distance
	})
	return hits[:min(k, len(hits))], nil
}
func Recall(exact, approx []Hit) (float64, error) {
	if len(exact) == 0 || len(exact) > 20 || len(approx) > 20 {
		return 0, ErrInput
	}
	relevant := map[string]bool{}
	for _, hit := range exact {
		if hit.ID == "" || relevant[hit.ID] {
			return 0, ErrInput
		}
		relevant[hit.ID] = true
	}
	seen := map[string]bool{}
	matched := 0
	for _, hit := range approx {
		if hit.ID == "" || seen[hit.ID] {
			return 0, ErrInput
		}
		seen[hit.ID] = true
		if relevant[hit.ID] {
			matched++
		}
	}
	return float64(matched) / float64(len(relevant)), nil
}
func fixtures() []Row {
	rows := make([]Row, 1024)
	for n := range rows {
		theta := 2 * math.Pi * (float64(n) + .137) / float64(len(rows))
		tenant := "other"
		if n%4 == 0 {
			tenant = "demo"
		}
		version := "v1"
		if n%17 == 0 {
			version = "old"
		}
		space := "fixture-v1"
		if n%31 == 0 {
			space = "another-space"
		}
		rows[n] = Row{ID: fmt.Sprintf("chunk-%04d", n), Scope: Scope{Tenant: tenant, Space: space, Version: version}, Text: fmt.Sprintf("synthetic row %d", n), Vector: Vector{math.Cos(theta), math.Sin(theta), .21 + float64(n)*1e-5}}
	}
	return rows
}
func main() {
	hits, err := Exact(context.Background(), fixtures(), Scope{Tenant: "demo", Space: "fixture-v1", Version: "v1"}, Vector{1, .23, .13}, 5)
	if err != nil {
		panic(err)
	}
	fmt.Printf("baseline_hits=%d first=%s distance=%.6f\n", len(hits), hits[0].ID, hits[0].Distance)
	fmt.Println("vector_database=not_run; enable explicit VECTOR_E2E for real pgvector comparison")
}
