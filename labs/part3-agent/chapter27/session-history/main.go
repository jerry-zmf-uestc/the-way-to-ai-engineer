package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound          = errors.New("session: not found")
	ErrConflict          = errors.New("session: version conflict")
	ErrInvalid           = errors.New("session: invalid history")
	ErrRevisionExhausted = errors.New("session: revision space exhausted")
)

// Scope由可信认证层提供，不能从模型参数中取得。
type Scope struct{ Tenant, User string }
type Call struct {
	ID, Name string
	Args     json.RawMessage
}
type Message struct {
	Role, Content, CallID string
	Calls                 []Call
}
type Snapshot struct {
	Version   uint64
	ExpiresAt time.Time
	Messages  []Message
}
type key struct {
	Scope
	ID string
}
type Store struct {
	mu       sync.Mutex
	revision uint64
	sessions map[key]Snapshot
	now      func() time.Time
}

func NewStore(now func() time.Time) *Store { return &Store{sessions: make(map[key]Snapshot), now: now} }
func clone(messages []Message) []Message {
	out := append([]Message(nil), messages...)
	for i, m := range out {
		out[i].Calls = append([]Call(nil), m.Calls...)
		for j, c := range out[i].Calls {
			out[i].Calls[j].Args = append(json.RawMessage(nil), c.Args...)
		}
	}
	return out
}

// bounds在复制与JSON解析之前限制体积；协议检查另行进行。
func bounds(messages []Message) error {
	if len(messages) > 128 {
		return fmt.Errorf("%w: message limit", ErrInvalid)
	}
	size := 0
	for _, m := range messages {
		if len(m.Calls) > 64 {
			return fmt.Errorf("%w: call limit", ErrInvalid)
		}
		size += len(m.Content) + len(m.CallID)
		if size > 64<<10 {
			return fmt.Errorf("%w: history byte limit", ErrInvalid)
		}
		for _, c := range m.Calls {
			size += len(c.ID) + len(c.Name) + len(c.Args)
			if size > 64<<10 {
				return fmt.Errorf("%w: history byte limit", ErrInvalid)
			}
		}
	}
	return nil
}
func validate(messages []Message) error {
	if len(messages) > 0 && messages[0].Role != "user" {
		return ErrInvalid
	}
	if err := bounds(messages); err != nil {
		return err
	}
	pending := map[string]bool{}
	seen := map[string]bool{}
	for _, m := range messages {
		if len(m.Calls) > 0 && m.Role != "assistant" {
			return fmt.Errorf("%w: calls need assistant role", ErrInvalid)
		}
		if m.CallID != "" && m.Role != "tool" {
			return fmt.Errorf("%w: call ID needs tool role", ErrInvalid)
		}
		switch m.Role {
		case "user":
			if len(pending) > 0 || strings.TrimSpace(m.Content) == "" {
				return fmt.Errorf("%w: user before observations or empty input", ErrInvalid)
			}
		case "assistant":
			if len(pending) > 0 {
				return fmt.Errorf("%w: assistant before observations", ErrInvalid)
			}
			if len(m.Calls) == 0 && strings.TrimSpace(m.Content) == "" {
				return fmt.Errorf("%w: empty assistant", ErrInvalid)
			}
			for _, c := range m.Calls {
				if c.ID == "" || c.Name == "" || seen[c.ID] || !json.Valid(c.Args) {
					return fmt.Errorf("%w: invalid or duplicate call", ErrInvalid)
				}
				seen[c.ID] = true
				pending[c.ID] = true
			}
		case "tool":
			if !pending[m.CallID] {
				return fmt.Errorf("%w: unmatched observation", ErrInvalid)
			}
			delete(pending, m.CallID)
		default:
			return fmt.Errorf("%w: unknown role", ErrInvalid)
		}
	}
	return nil
}
func validKey(scope Scope, id string) bool { return scope.Tenant != "" && scope.User != "" && id != "" }
func (s *Store) Create(scope Scope, id string, ttl time.Duration) error {
	if !validKey(scope, id) || ttl <= 0 {
		return fmt.Errorf("%w: scope or TTL", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key{scope, id}
	now := s.now()
	if current, ok := s.sessions[k]; ok && now.Before(current.ExpiresAt) {
		return ErrConflict
	}
	if s.revision == ^uint64(0) {
		return ErrRevisionExhausted
	}
	s.revision++
	s.sessions[k] = Snapshot{Version: s.revision, ExpiresAt: now.Add(ttl)}
	return nil
}

// Read返回快照；过期条目在访问时删除，不延长生命周期。
func (s *Store) Read(scope Scope, id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key{scope, id}
	current, ok := s.sessions[k]
	if !ok || !s.now().Before(current.ExpiresAt) {
		delete(s.sessions, k)
		return Snapshot{}, ErrNotFound
	}
	current.Messages = clone(current.Messages)
	return current, nil
}

// Append在同一临界区检查版本、校验完整候选历史并提交。失败不改旧版本。
func (s *Store) Append(scope Scope, id string, expected uint64, batch []Message) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key{scope, id}
	current, ok := s.sessions[k]
	if !ok || !s.now().Before(current.ExpiresAt) {
		delete(s.sessions, k)
		return Snapshot{}, ErrNotFound
	}
	if current.Version != expected {
		return Snapshot{}, ErrConflict
	}
	if len(batch) == 0 {
		return Snapshot{}, fmt.Errorf("%w: empty append", ErrInvalid)
	}
	if len(batch) > 128-len(current.Messages) {
		return Snapshot{}, fmt.Errorf("%w: message limit", ErrInvalid)
	}
	if err := bounds(batch); err != nil {
		return Snapshot{}, err
	}
	candidate := clone(current.Messages)
	candidate = append(candidate, clone(batch)...)
	if err := validate(candidate); err != nil {
		return Snapshot{}, err
	}
	if s.revision == ^uint64(0) {
		return Snapshot{}, ErrRevisionExhausted
	}
	current.Messages = candidate
	s.revision++
	current.Version = s.revision
	s.sessions[k] = current
	current.Messages = clone(current.Messages)
	return current, nil
}
func main() {
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	store := NewStore(func() time.Time { return now })
	scope := Scope{"demo", "reader"}
	if err := store.Create(scope, "chat", time.Hour); err != nil {
		panic(err)
	}
	initial, _ := store.Read(scope, "chat")
	first, err := store.Append(scope, "chat", initial.Version, []Message{{Role: "user", Content: "查资料"}, {Role: "assistant", Calls: []Call{{ID: "read-1", Name: "read", Args: json.RawMessage(`{"query":"context"}`)}}}})
	if err != nil {
		panic(err)
	}
	final, err := store.Append(scope, "chat", first.Version, []Message{{Role: "tool", CallID: "read-1", Content: "固定资料"}, {Role: "assistant", Content: "依据资料回答"}})
	if err != nil {
		panic(err)
	}
	_, conflict := store.Append(scope, "chat", first.Version, []Message{{Role: "user", Content: "旧版本"}})
	fmt.Printf("version=%d messages=%d stale_conflict=%t\n", final.Version, len(final.Messages), errors.Is(conflict, ErrConflict))
}
