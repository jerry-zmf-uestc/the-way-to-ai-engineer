package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPagesCarryArtifactAndFullContentIdentity(t *testing.T) {
	w, _ := fixture(t, Scope{"t", "u", "task"}, time.Now)
	a, err := w.Publish(context.Background(), "first", "shared\nfirst-tail\n", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	b, err := w.Publish(context.Background(), "second", "shared\nother-tail\n", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	one, err := w.Read(context.Background(), a.ID, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	other, err := w.Read(context.Background(), b.ID, 0, 1)
	if err != nil || one.Text != other.Text || one.ArtifactID == other.ArtifactID || one.Digest == other.Digest || one.Digest != a.Digest || one.StartLine != 0 || one.Complete {
		t.Fatal(one, other, err)
	}
	next, err := w.Read(context.Background(), a.ID, one.NextLine, 1)
	if err != nil || next.ArtifactID != one.ArtifactID || next.Digest != one.Digest || next.StartLine != one.NextLine || !next.Complete || next.Text != "first-tail\n" {
		t.Fatal(one, next, err)
	}
	end, err := w.Read(context.Background(), a.ID, next.NextLine, 1)
	if err != nil || end.ArtifactID != a.ID || end.Digest != a.Digest || end.StartLine != 2 || end.NextLine != 2 || !end.Complete || end.Text != "" {
		t.Fatal(end, err)
	}
}

func TestChangedContinuationReturnsNoPageIdentity(t *testing.T) {
	w, _ := fixture(t, Scope{"t", "u", "task"}, time.Now)
	a, err := w.Publish(context.Background(), "report", "a\nb\n", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	first, err := w.Read(context.Background(), a.ID, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	f, err := w.root.OpenFile(w.artifacts[a.ID].path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("a\nc\n"); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	p, err := w.Read(context.Background(), a.ID, first.NextLine, 1)
	if !errors.Is(err, ErrChanged) || p != (Page{}) {
		t.Fatal("changed page leaked identity/data", p, err)
	}
}
