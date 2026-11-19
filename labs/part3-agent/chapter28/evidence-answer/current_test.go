package main

import (
	"context"
	"errors"
	"testing"
)

func TestCurrentEvidenceRecheckedBeforeDelivery(t *testing.T) {
	for _, change := range []string{"unchanged", "text", "version", "deleted", "tenant", "read-error", "cancel", "abstained-update"} {
		t.Run(change, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			item := Evidence{ID: "a", Source: "doc", Version: "1", Tenant: "u", Text: "Timeout is 30 seconds."}
			current := item
			calls := 0
			failed := false
			cause := errors.New("store unavailable")
			read := func(context.Context, string, string) (Evidence, error) {
				calls++
				if failed {
					return Evidence{}, cause
				}
				return current, nil
			}
			model := func(context.Context, string, []Evidence) (string, error) {
				switch change {
				case "text", "abstained-update":
					current.Text = "Timeout is 60 seconds."
				case "version":
					current.Version = "2"
				case "deleted":
					current = Evidence{}
				case "tenant":
					current.Tenant = "other"
				case "read-error":
					failed = true
				case "cancel":
					cancel()
				}
				if change == "abstained-update" {
					return `{"abstained":true,"reason":"insufficient_evidence","claims":[]}`, nil
				}
				return `{"claims":[{"text":"Timeout is 30 seconds.","citations":[{"evidence_id":"a","version":"1","quote":"Timeout is 30 seconds."}]}]}`, nil
			}
			judge := func(context.Context, JointClaim, []Evidence) (Verdict, error) { return Supported, nil }
			answer, err := RunCurrentJoint(ctx, "u", "timeout?", []Evidence{item}, read, model, judge)
			if change == "unchanged" {
				if err != nil || len(answer.Claims) != 1 || calls != 2 {
					t.Fatalf("answer=%+v calls=%d err=%v", answer, calls, err)
				}
				return
			}
			if err == nil || len(answer.Claims) != 0 || answer.Abstained {
				t.Fatalf("released answer=%+v err=%v", answer, err)
			}
			if change == "read-error" && !errors.Is(err, cause) {
				t.Fatal(err)
			}
			if change == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestCurrentEvidenceRejectsBeforeGeneration(t *testing.T) {
	item := Evidence{ID: "a", Source: "doc", Version: "1", Tenant: "u", Text: "x"}
	for _, kind := range []string{"stale", "foreign", "duplicate", "missing-reader"} {
		t.Run(kind, func(t *testing.T) {
			items := []Evidence{item}
			reads, models := 0, 0
			read := CurrentEvidenceReader(func(context.Context, string, string) (Evidence, error) {
				reads++
				v := item
				v.Text = "changed"
				return v, nil
			})
			switch kind {
			case "foreign":
				items[0].Tenant = "other"
			case "duplicate":
				items = append(items, item)
			case "missing-reader":
				read = nil
			}
			model := func(context.Context, string, []Evidence) (string, error) { models++; return "", nil }
			answer, err := RunCurrentJoint(context.Background(), "u", "q", items, read, model, nil)
			if !errors.Is(err, ErrEvidence) || models != 0 || len(answer.Claims) != 0 {
				t.Fatalf("models=%d answer=%+v err=%v", models, answer, err)
			}
			if kind != "stale" && reads != 0 {
				t.Fatalf("reads=%d", reads)
			}
		})
	}
}
