package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

type HandoffCheckpoint struct {
	Schema      int
	Fingerprint string
	State       HandoffState
	Used        int
}

func checkpointFact(f Fact) bool {
	return validFact(f) && utf8.ValidString(f.Key) && utf8.ValidString(f.Source) && utf8.ValidString(f.Value)
}

func handoffFingerprint(h *Handoff, initialOwner, version string) (string, error) {
	validName := func(name string) bool { return name != "" && len(name) <= 128 && utf8.ValidString(name) }
	if version == "" || len(version) > 128 || !utf8.ValidString(version) || len(h.dispatcher.ParentTools) > 8 || initialOwner != h.initialOwner || !validName(h.task.ID) || !validName(h.task.Parent) || !validName(h.task.Scope.Tenant) || !validName(h.task.Scope.User) || !utf8.ValidString(h.task.Goal) {
		return "", ErrHandoff
	}
	for _, fact := range h.task.Context {
		if !checkpointFact(fact) {
			return "", ErrHandoff
		}
	}
	roleTools := map[string][]string{}
	for name, role := range h.dispatcher.Roles {
		if !validName(name) || role.Name != name || role.Run == nil || len(role.Tools) > 8 {
			return "", ErrHandoff
		}
		for _, tool := range role.Tools {
			if !validName(tool) {
				return "", ErrHandoff
			}
		}
		roleTools[name] = append([]string(nil), role.Tools...)
	}
	names := []string{}
	for name, tool := range h.dispatcher.ParentTools {
		if !validName(name) || tool == nil {
			return "", ErrHandoff
		}
		names = append(names, name)
	}
	sort.Strings(names)
	data, err := json.Marshal(struct {
		Version, InitialOwner     string
		Task                      Task
		Roles                     map[string][]string
		Tools                     []string
		Edges                     map[string]map[string]bool
		Depth, Hops, Turns, Limit int
	}{version, initialOwner, h.task, roleTools, names, h.edges, h.dispatcher.MaxDepth, h.maxHops, h.maxTurns, h.dispatcher.Ledger.Limit})
	if err != nil || len(data) > 32<<10 {
		return "", ErrHandoff
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

// CheckpointHandoff requires an exclusively owned ledger and a quiescent task.
func CheckpointHandoff(h *Handoff, version string) ([]byte, error) {
	if h == nil || h.dispatcher.Ledger == nil {
		return nil, ErrHandoff
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state.Status != "ready" && h.state.Status != "completed" {
		return nil, ErrHandoff
	}
	keys := map[string]bool{}
	for _, fact := range h.state.Facts {
		if keys[fact.Key] || !checkpointFact(fact) {
			return nil, ErrHandoff
		}
		keys[fact.Key] = true
	}
	h.dispatcher.Ledger.mu.Lock()
	defer h.dispatcher.Ledger.mu.Unlock()
	fingerprint, err := handoffFingerprint(h, h.initialOwner, version)
	if err != nil || h.dispatcher.Ledger.Used < 0 || h.dispatcher.Ledger.Used > h.dispatcher.Ledger.Limit {
		return nil, ErrHandoff
	}
	data, err := json.Marshal(HandoffCheckpoint{1, fingerprint, h.state, h.dispatcher.Ledger.Used})
	if err != nil || len(data) > 32<<10 {
		return nil, ErrHandoff
	}
	return data, nil
}

func uniqueHandoffJSON(dec *json.Decoder, depth int) error {
	if depth > 8 {
		return ErrHandoff
	}
	token, err := dec.Token()
	if err != nil {
		return ErrHandoff
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return ErrHandoff
	}
	seen := map[string]bool{}
	for dec.More() {
		if delim == '{' {
			key, err := dec.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[strings.ToLower(name)] {
				return ErrHandoff
			}
			seen[strings.ToLower(name)] = true
		}
		if err := uniqueHandoffJSON(dec, depth+1); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return ErrHandoff
	}
	return nil
}

func RestoreHandoff(data []byte, task Task, d Dispatcher, initialOwner string, edges map[string][]string, maxHops, maxTurns int, version string) (*Handoff, error) {
	if len(data) == 0 || len(data) > 32<<10 || !utf8.Valid(data) || uniqueHandoffJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return nil, ErrHandoff
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var saved HandoffCheckpoint
	if dec.Decode(&saved) != nil || dec.Decode(new(any)) != io.EOF || saved.Schema != 1 {
		return nil, ErrHandoff
	}
	h, err := NewHandoff(task, d, initialOwner, edges, maxHops, maxTurns)
	if err != nil {
		return nil, err
	}
	fingerprint, err := handoffFingerprint(h, initialOwner, version)
	s := saved.State
	if err != nil || saved.Fingerprint != fingerprint || s.ID != task.ID || s.Scope != task.Scope || s.Hops < 0 || s.Hops > maxHops || s.Turns < 0 || s.Turns > maxTurns || len(s.Facts) > 8 || saved.Used < 0 || saved.Used > d.Ledger.Limit {
		return nil, ErrHandoff
	}
	revision := uint64(1 + s.Hops + 2*s.Turns)
	if s.Status == "completed" {
		revision++
	} else if s.Status != "ready" {
		return nil, ErrHandoff
	}
	if s.Revision != revision {
		return nil, ErrHandoff
	}
	reachable := map[string]bool{initialOwner: true}
	for n := 0; n < s.Hops; n++ {
		next := map[string]bool{}
		for owner := range reachable {
			for target := range h.edges[owner] {
				next[target] = true
			}
		}
		reachable = next
	}
	role, ok := h.dispatcher.Roles[s.Owner]
	if !reachable[s.Owner] || !ok || role.Name != s.Owner || role.Run == nil {
		return nil, ErrHandoff
	}
	known := map[string]Fact{}
	for _, fact := range s.Facts {
		if !checkpointFact(fact) {
			return nil, ErrHandoff
		}
		if _, duplicate := known[fact.Key]; duplicate {
			return nil, ErrHandoff
		}
		known[fact.Key] = fact
	}
	for _, original := range task.Context {
		if known[original.Key] != original {
			return nil, ErrHandoff
		}
	}
	d.Ledger.mu.Lock()
	defer d.Ledger.mu.Unlock()
	if d.Ledger.Used < 0 || d.Ledger.Used > d.Ledger.Limit {
		return nil, ErrHandoff
	}
	d.Ledger.Used = max(d.Ledger.Used, saved.Used)
	h.state = s
	h.state.Facts = append([]Fact(nil), s.Facts...)
	return h, nil
}
