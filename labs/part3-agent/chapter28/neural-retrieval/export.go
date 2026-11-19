package main

import (
	"context"
	"strings"
)

type ExportEvidence struct{ ID, Source, Version, Tenant, Text string }
type ExportRow struct {
	ID       string
	Question string
	Evidence []ExportEvidence
}
type RetrievalExport struct {
	Protocol, Revision, Space string
	Rows                      []ExportRow
}

// ExportCandidates carries selected original documents, never relevance labels or generated text.
func ExportCandidates(ctx context.Context, docs []Document, questions []Question, batch Batch) (RetrievalExport, error) {
	if err := ctx.Err(); err != nil {
		return RetrievalExport{}, err
	}
	if len(questions) < 1 || len(questions) > 6 || len(docs) == 0 {
		return RetrievalExport{}, ErrInput
	}
	if err := Validate(batch, len(docs)+len(questions)); err != nil {
		return RetrievalExport{}, err
	}
	result := RetrievalExport{Protocol: "retrieved-evidence-v1", Revision: revision, Space: space}
	docIDs := map[string]bool{}
	for _, d := range docs {
		if d.ID == "" || len(d.ID) > 128 || docIDs[d.ID] || strings.TrimSpace(d.Text) == "" || len(d.Text) > 4096 {
			return RetrievalExport{}, ErrInput
		}
		docIDs[d.ID] = true
	}
	seen := map[string]bool{}
	for n, q := range questions {
		if q.ID == "" || len(q.ID) > 128 || seen[q.ID] || strings.TrimSpace(q.Text) == "" || len(q.Text) > 4096 || batch.Truncated[len(docs)+n] {
			return RetrievalExport{}, ErrInput
		}
		seen[q.ID] = true
		hits, err := Dense(docs, batch.Vectors[:len(docs)], batch.Vectors[len(docs)+n], "demo", "v1", 1)
		if err != nil {
			return RetrievalExport{}, err
		}
		row := ExportRow{ID: q.ID, Question: q.Text, Evidence: []ExportEvidence{}}
		for _, hit := range hits {
			for i, d := range docs {
				if d.ID == hit.ID {
					if !eligible(d, "demo", "v1") || batch.Truncated[i] {
						return RetrievalExport{}, ErrInput
					}
					row.Evidence = append(row.Evidence, ExportEvidence{d.ID, "fixture:" + d.ID, d.Version, d.Tenant, d.Text})
				}
			}
		}
		if len(row.Evidence) != len(hits) {
			return RetrievalExport{}, ErrInput
		}
		result.Rows = append(result.Rows, row)
	}
	return result, ctx.Err()
}
