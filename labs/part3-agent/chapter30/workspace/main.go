package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrDenied  = errors.New("workspace: unregistered or expired artifact")
	ErrQuota   = errors.New("workspace: quota exceeded")
	ErrChanged = errors.New("workspace: artifact changed")
)

type Scope struct{ Tenant, User, Task string }
type Artifact struct {
	ID, Digest string
	Bytes      int
	ExpiresAt  time.Time
	path       string
	revoked    bool
}
type Page struct {
	ArtifactID, Digest string
	Text               string
	StartLine          int
	NextLine           int
	Complete           bool
}
type Workspace struct {
	mu        sync.Mutex
	root      *os.Root
	scope     Scope
	now       func() time.Time
	artifacts map[string]Artifact
	Used      int
}

func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
func digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
func New(base *os.Root, scope Scope, now func() time.Time) (*Workspace, error) {
	if !validID(scope.Tenant) || !validID(scope.User) || !validID(scope.Task) || now == nil {
		return nil, ErrDenied
	}
	path := scope.Tenant + "/" + scope.User + "/" + scope.Task
	if err := base.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	root, err := base.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &Workspace{root: root, scope: scope, now: now, artifacts: map[string]Artifact{}}, nil
}
func (w *Workspace) Close() error { return w.root.Close() }

// Publish只由应用发布产物，Agent工具不暴露这个入口。ID在当前Workspace不可复用。
func (w *Workspace) Publish(ctx context.Context, id, text string, ttl time.Duration) (Artifact, error) {
	if err := ctx.Err(); err != nil {
		return Artifact{}, err
	}
	if !validID(id) || !utf8.ValidString(text) || ttl <= 0 {
		return Artifact{}, ErrDenied
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.artifacts[id]; exists {
		return Artifact{}, ErrDenied
	}
	if len(w.artifacts) >= 16 || len(text) > 32<<10 || len(text) > 64<<10-w.Used {
		return Artifact{}, ErrQuota
	}
	path := digest(id) + ".txt"
	f, err := w.root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Artifact{}, err
	}
	if _, err = f.WriteString(text); err != nil {
		f.Close()
		w.root.Remove(path)
		return Artifact{}, err
	}
	if err = f.Close(); err != nil {
		w.root.Remove(path)
		return Artifact{}, err
	}
	if err := ctx.Err(); err != nil {
		w.root.Remove(path)
		return Artifact{}, err
	}
	artifact := Artifact{ID: id, Digest: digest(text), Bytes: len(text), ExpiresAt: w.now().Add(ttl), path: path}
	w.artifacts[id] = artifact
	w.Used += len(text)
	public := artifact
	public.path = ""
	return public, nil
}

// Read按行读取注册产物，读取前后检查context，并验证完整有界文件摘要。
func (w *Workspace) Read(ctx context.Context, id string, start, count int) (Page, error) {
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	if !validID(id) || start < 0 || count <= 0 || count > 128 {
		return Page{}, ErrDenied
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	artifact, ok := w.artifacts[id]
	if !ok || artifact.revoked || !w.now().Before(artifact.ExpiresAt) {
		return Page{}, ErrDenied
	}
	f, err := w.root.Open(artifact.path)
	if err != nil {
		return Page{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (32<<10)+1))
	if err != nil {
		return Page{}, err
	}
	if len(data) != artifact.Bytes || digest(string(data)) != artifact.Digest || !utf8.Valid(data) {
		return Page{}, ErrChanged
	}
	lines := strings.SplitAfter(string(data), "\n")
	if len(data) == 0 {
		lines = nil
	} else if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if start > len(lines) {
		return Page{}, ErrDenied
	}
	end := start + min(count, len(lines)-start)
	text := strings.Join(lines[start:end], "")
	if len(text) > 4096 {
		return Page{}, ErrQuota
	}
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	if !w.now().Before(artifact.ExpiresAt) {
		return Page{}, ErrDenied
	}
	return Page{ArtifactID: artifact.ID, Digest: artifact.Digest, Text: text, StartLine: start, NextLine: end, Complete: end == len(lines)}, nil
}
func (w *Workspace) List() []Artifact {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []Artifact{}
	for _, a := range w.artifacts {
		if !a.revoked && w.now().Before(a.ExpiresAt) {
			a.path = ""
			out = append(out, a)
		}
	}
	return out
}

// Cleanup由应用调用，撤销过期产物；失败保留目录记录以便复验，ID墓碑仍在。
func (w *Workspace) Cleanup() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, a := range w.artifacts {
		if a.revoked || w.now().Before(a.ExpiresAt) {
			continue
		}
		if err := w.root.Remove(a.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		w.Used -= a.Bytes
		a.revoked = true
		w.artifacts[id] = a
	}
	return nil
}
func main() {
	dir, err := os.MkdirTemp("", "workspace-demo-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	base, err := os.OpenRoot(dir)
	if err != nil {
		panic(err)
	}
	defer base.Close()
	w, err := New(base, Scope{"demo", "reader", "task"}, time.Now)
	if err != nil {
		panic(err)
	}
	defer w.Close()
	artifact, err := w.Publish(context.Background(), "report-1", "line1\nline2\nline3\n", time.Hour)
	if err != nil {
		panic(err)
	}
	page, err := w.Read(context.Background(), artifact.ID, 0, 2)
	if err != nil {
		panic(err)
	}
	fmt.Printf("artifact=%s bytes=%d next_line=%d complete=%t page=%q\n", artifact.ID, artifact.Bytes, page.NextLine, page.Complete, page.Text)
}
