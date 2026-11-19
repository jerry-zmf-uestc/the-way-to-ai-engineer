package main

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrPageSequence = errors.New("workspace: incomplete or inconsistent page sequence")

type Collected struct {
	ArtifactID, Digest, Text string
	Lines                    int
}

// Assemble checks app-read pages against this Workspace's current catalog.
// It does not attest who read a page or automatically provide content to a model.
func (w *Workspace) Assemble(ctx context.Context, id string, pages []Page, maxBytes int) (Collected, error) {
	if err := ctx.Err(); err != nil {
		return Collected{}, err
	}
	if !validID(id) {
		return Collected{}, ErrDenied
	}
	if len(pages) == 0 || len(pages) > 256 || maxBytes <= 0 || maxBytes > 32<<10 {
		return Collected{}, ErrQuota
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	artifact, ok := w.artifacts[id]
	if !ok || artifact.revoked || !w.now().Before(artifact.ExpiresAt) {
		return Collected{}, ErrDenied
	}
	if artifact.Bytes > maxBytes {
		return Collected{}, ErrQuota
	}
	var text strings.Builder
	text.Grow(artifact.Bytes)
	next := 0
	for n, page := range pages {
		if page.ArtifactID != id || page.Digest != artifact.Digest || page.StartLine != next || page.NextLine < next || !utf8.ValidString(page.Text) {
			return Collected{}, ErrPageSequence
		}
		if len(page.Text) > 4096 || len(page.Text) > maxBytes-text.Len() {
			return Collected{}, ErrQuota
		}
		lines := strings.Count(page.Text, "\n")
		if page.Text != "" && !strings.HasSuffix(page.Text, "\n") {
			lines++
		}
		if lines > 128 {
			return Collected{}, ErrQuota
		}
		if page.NextLine-page.StartLine != lines || page.Complete != (n == len(pages)-1) || (!page.Complete && (lines == 0 || !strings.HasSuffix(page.Text, "\n"))) {
			return Collected{}, ErrPageSequence
		}
		text.WriteString(page.Text)
		next = page.NextLine
	}
	if text.Len() != artifact.Bytes || digest(text.String()) != artifact.Digest {
		return Collected{}, ErrPageSequence
	}
	if err := ctx.Err(); err != nil {
		return Collected{}, err
	}
	if !w.now().Before(artifact.ExpiresAt) {
		return Collected{}, ErrDenied
	}
	return Collected{ArtifactID: id, Digest: artifact.Digest, Text: text.String(), Lines: next}, nil
}
