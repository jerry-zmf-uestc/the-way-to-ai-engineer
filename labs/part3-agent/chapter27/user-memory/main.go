package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrDenied   = errors.New("memory: consent denied")
	ErrConflict = errors.New("memory: revision conflict")
	ErrNotFound = errors.New("memory: inactive or missing")
	ErrInput    = errors.New("memory: invalid input")
)

type Scope struct{ Tenant, User string }
type Record struct {
	Key, Value, Source string
	Revision           int64
	ExpiresAt          time.Time
	Deleted            bool
}

// Consent由可信应用根据用户操作产生，不从模型输出解码。
type Consent struct {
	Scope                 Scope
	Key, Operation, Value string
	Allowed               bool
	ExpiresAt             time.Time
}
type Backend interface {
	Peek(context.Context, Scope, string) (Record, error)
	CAS(context.Context, Scope, Record, int64) (Record, error)
	Active(context.Context, Scope, time.Time) ([]Record, error)
}
type Memory struct {
	Store Backend
	Now   func() time.Time
}

func scopeValid(s Scope) bool {
	return s.Tenant != "" && s.User != "" && len(s.Tenant) <= 128 && len(s.User) <= 128
}
func validate(key, value, source string) error {
	if !utf8.ValidString(value) || len(source) == 0 || len(source) > 256 {
		return ErrInput
	}
	switch key {
	case "response-style":
		if value != "concise" && value != "detailed" {
			return ErrInput
		}
	case "locale":
		if value != "zh" && value != "en" {
			return ErrInput
		}
	default:
		return ErrInput
	}
	return nil
}
func (m Memory) authorized(scope Scope, key, operation string, c Consent, now time.Time) bool {
	return scopeValid(scope) && c.Allowed && c.Scope == scope && c.Key == key && c.Operation == operation && now.Before(c.ExpiresAt)
}
func (m Memory) Write(ctx context.Context, s Scope, key, value, source string, expected int64, ttl time.Duration, c Consent) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	now := m.Now()
	if !m.authorized(s, key, "write", c, now) || c.Value != value {
		return Record{}, ErrDenied
	}
	if expected < 0 || expected == int64(^uint64(0)>>1) || ttl <= 0 || ttl > 30*24*time.Hour {
		return Record{}, ErrInput
	}
	if err := validate(key, value, source); err != nil {
		return Record{}, err
	}
	return m.Store.CAS(ctx, s, Record{Key: key, Value: value, Source: source, ExpiresAt: now.Add(ttl)}, expected)
}

// 删除清空值与来源，保留修订墓碑，防止旧写入在对象重建后重新有效。
func (m Memory) Delete(ctx context.Context, s Scope, key string, expected int64, c Consent) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	now := m.Now()
	if !m.authorized(s, key, "delete", c, now) {
		return Record{}, ErrDenied
	}
	if key != "locale" && key != "response-style" || expected <= 0 || expected == int64(^uint64(0)>>1) {
		return Record{}, ErrInput
	}
	return m.Store.CAS(ctx, s, Record{Key: key, ExpiresAt: now, Deleted: true}, expected)
}
func (m Memory) Recall(ctx context.Context, s Scope) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !scopeValid(s) {
		return nil, ErrDenied
	}
	rows, err := m.Store.Active(ctx, s, m.Now())
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if len(rows) > 128 {
		return nil, ErrInput
	}
	out := []Record{}
	seen := map[string]bool{}
	now := m.Now()
	for _, record := range rows {
		if record.Deleted || !now.Before(record.ExpiresAt) {
			continue
		}
		if record.Revision <= 0 || seen[record.Key] || validate(record.Key, record.Value, record.Source) != nil {
			return nil, ErrInput
		}
		seen[record.Key] = true
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Resolve模拟向量/缓存返回候选键后的事实回查；旧索引不能直接进入模型。
func (m Memory) Resolve(ctx context.Context, s Scope, keys []string) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !scopeValid(s) || len(keys) > 16 {
		return nil, ErrDenied
	}
	for _, key := range keys {
		if strings.TrimSpace(key) == "" || !utf8.ValidString(key) || len(key) > 128 {
			return nil, ErrInput
		}
	}
	out := []Record{}
	seen := map[string]bool{}
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		record, err := m.Store.Peek(ctx, s, key)
		if stop := ctx.Err(); stop != nil {
			return nil, stop
		}
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !record.Deleted && m.Now().Before(record.ExpiresAt) {
			if record.Key != key || record.Revision <= 0 || validate(record.Key, record.Value, record.Source) != nil {
				return nil, ErrInput
			}
			out = append(out, record)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type memoryKey struct {
	Scope
	Key string
}
type FakeStore struct {
	mu   sync.Mutex
	Rows map[memoryKey]Record
}

func NewFake() *FakeStore { return &FakeStore{Rows: map[memoryKey]Record{}} }
func (f *FakeStore) Peek(ctx context.Context, s Scope, key string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	record, ok := f.Rows[memoryKey{s, key}]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}
func (f *FakeStore) CAS(ctx context.Context, s Scope, record Record, expected int64) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := memoryKey{s, record.Key}
	current, exists := f.Rows[key]
	if exists && current.Revision != expected || !exists && expected != 0 {
		return Record{}, ErrConflict
	}
	record.Revision = expected + 1
	f.Rows[key] = record
	return record, nil
}
func (f *FakeStore) Active(ctx context.Context, s Scope, now time.Time) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Record{}
	for key, record := range f.Rows {
		if key.Scope == s && !record.Deleted && now.Before(record.ExpiresAt) {
			out = append(out, record)
		}
	}
	return out, nil
}
func main() {
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	m := Memory{NewFake(), func() time.Time { return now }}
	s := Scope{"demo", "reader"}
	c := Consent{Scope: s, Key: "response-style", Operation: "write", Value: "concise", Allowed: true, ExpiresAt: now.Add(time.Minute)}
	record, err := m.Write(context.Background(), s, c.Key, "concise", "user-confirmation", 0, time.Hour, c)
	if err != nil {
		panic(err)
	}
	active, err := m.Recall(context.Background(), s)
	if err != nil {
		panic(err)
	}
	c.Operation = "delete"
	deleted, err := m.Delete(context.Background(), s, c.Key, record.Revision, c)
	if err != nil {
		panic(err)
	}
	stale, err := m.Resolve(context.Background(), s, []string{c.Key})
	if err != nil {
		panic(err)
	}
	fmt.Printf("recalled=%d deleted_revision=%d stale_candidates_resolved=%d\n", len(active), deleted.Revision, len(stale))
}
