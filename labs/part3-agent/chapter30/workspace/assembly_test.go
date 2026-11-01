package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func assemblyFixture(t *testing.T) (*Workspace, Artifact, []Page) {
	t.Helper()
	w, _ := fixture(t, Scope{"t", "u", "task"}, time.Now)
	a, err := w.Publish(context.Background(), "report", "甲\n乙\n丙", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pages := []Page{}
	for start := 0; ; {
		page, err := w.Read(context.Background(), a.ID, start, 1)
		if err != nil {
			t.Fatal(err)
		}
		pages = append(pages, page)
		if page.Complete {
			break
		}
		start = page.NextLine
	}
	return w, a, pages
}

func TestAssembleActualPagesPreservesFullIdentity(t *testing.T) {
	w, a, pages := assemblyFixture(t)
	got, err := w.Assemble(context.Background(), a.ID, pages, a.Bytes)
	if err != nil || got.Text != "甲\n乙\n丙" || got.Lines != 3 || got.Digest != a.Digest || got.ArtifactID != a.ID {
		t.Fatal(got, err)
	}
	pages[0].Text = "改"
	if got.Text != "甲\n乙\n丙" {
		t.Fatal("collected content aliases caller pages")
	}
	empty, err := w.Publish(context.Background(), "empty", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	page, err := w.Read(context.Background(), empty.ID, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err = w.Assemble(context.Background(), empty.ID, []Page{page}, 1)
	if err != nil || got.Text != "" || got.Lines != 0 || got.Digest != empty.Digest {
		t.Fatal("empty EOF lost identity", got, err)
	}
}

func TestAssembleRejectsBrokenSequenceWithoutPartialContent(t *testing.T) {
	w, a, original := assemblyFixture(t)
	for name, change := range map[string]func([]Page) []Page{
		"missing first":  func(p []Page) []Page { return p[1:] },
		"missing middle": func(p []Page) []Page { return []Page{p[0], p[2]} },
		"overlap":        func(p []Page) []Page { p[1].StartLine = 0; return p },
		"reordered":      func(p []Page) []Page { p[0], p[1] = p[1], p[0]; return p },
		"wrong id":       func(p []Page) []Page { p[1].ArtifactID = "other"; return p },
		"wrong digest":   func(p []Page) []Page { p[1].Digest = digest("other"); return p },
		"wrong text":     func(p []Page) []Page { p[1].Text = "改\n"; return p },
		"wrong range":    func(p []Page) []Page { p[1].NextLine++; return p },
		"early complete": func(p []Page) []Page { p[0].Complete = true; return p },
		"no complete":    func(p []Page) []Page { p[len(p)-1].Complete = false; return p },
		"after complete": func(p []Page) []Page {
			return append(p, Page{ArtifactID: a.ID, Digest: a.Digest, StartLine: 3, NextLine: 3, Complete: true})
		},
		"no progress":  func(p []Page) []Page { p[1].Text = ""; p[1].NextLine = p[1].StartLine; return p },
		"split line":   func(p []Page) []Page { p[0].Text = "甲"; return p },
		"invalid utf8": func(p []Page) []Page { p[1].Text = string([]byte{0xff}); return p },
	} {
		t.Run(name, func(t *testing.T) {
			pages := change(append([]Page(nil), original...))
			got, err := w.Assemble(context.Background(), a.ID, pages, 32<<10)
			if !errors.Is(err, ErrPageSequence) || got != (Collected{}) {
				t.Fatal("partial or invalid content published", got, err)
			}
		})
	}
}

func TestAssembleChecksBudgetTaskExpiryAndCancellation(t *testing.T) {
	w, a, pages := assemblyFixture(t)
	for _, limit := range []int{0, a.Bytes - 1, (32 << 10) + 1} {
		if got, err := w.Assemble(context.Background(), a.ID, pages, limit); !errors.Is(err, ErrQuota) || got != (Collected{}) {
			t.Fatal("invalid budget accepted", got, err)
		}
	}
	for _, batch := range [][]Page{nil, make([]Page, 257)} {
		if _, err := w.Assemble(context.Background(), a.ID, batch, 32<<10); !errors.Is(err, ErrQuota) {
			t.Fatal("page count accepted", err)
		}
	}
	large := append([]Page(nil), pages...)
	large[0].Text = strings.Repeat("x", 4097)
	if _, err := w.Assemble(context.Background(), a.ID, large, 32<<10); !errors.Is(err, ErrQuota) {
		t.Fatal("page byte bound bypassed", err)
	}
	many, err := w.Publish(context.Background(), "many-lines", strings.Repeat("\n", 129), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	onePage := Page{ArtifactID: many.ID, Digest: many.Digest, Text: strings.Repeat("\n", 129), NextLine: 129, Complete: true}
	if _, err := w.Assemble(context.Background(), many.ID, []Page{onePage}, 32<<10); !errors.Is(err, ErrQuota) {
		t.Fatal("page line bound bypassed", err)
	}
	other, _ := fixture(t, Scope{"t", "u", "other-task"}, time.Now)
	if _, err := other.Assemble(context.Background(), a.ID, pages, 32<<10); !errors.Is(err, ErrDenied) {
		t.Fatal("unregistered task accepted", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.Assemble(ctx, a.ID, pages, 32<<10); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled content published", err)
	}
	calls := 0
	w.now = func() time.Time {
		calls++
		if calls == 1 {
			return a.ExpiresAt.Add(-time.Second)
		}
		return a.ExpiresAt
	}
	if got, err := w.Assemble(context.Background(), a.ID, pages, 32<<10); !errors.Is(err, ErrDenied) || got != (Collected{}) {
		t.Fatal("expired at return boundary", got, err)
	}
}
