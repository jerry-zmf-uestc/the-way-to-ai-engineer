package main

import (
	"context"
	"errors"
	"sync"
)

var ErrHandoff = errors.New("handoff: stale owner or invalid transition")

type HandoffState struct {
	ID            string
	Scope         Scope
	Owner, Status string
	Revision      uint64
	Hops, Turns   int
	Facts         []Fact
}
type HandoffRequest struct {
	ID       string
	Scope    Scope
	From, To string
	Revision uint64
}
type Handoff struct {
	mu                sync.Mutex
	state             HandoffState
	task              Task
	dispatcher        Dispatcher
	edges             map[string]map[string]bool
	maxHops, maxTurns int
	initialOwner      string
}

func NewHandoff(task Task, d Dispatcher, owner string, edges map[string][]string, maxHops, maxTurns int) (*Handoff, error) {
	role, ok := d.Roles[owner]
	if !ok || role.Name != owner || role.Run == nil || d.Ledger == nil || d.Ledger.Limit <= 0 || d.MaxDepth < 1 || task.ID == "" || task.Parent == "" || task.Scope.Tenant == "" || task.Scope.User == "" || len(task.Goal) > 256 || len(task.Context) > 8 || task.Depth != 0 || maxHops < 1 || maxHops > 16 || maxTurns < 1 || maxTurns > 32 || len(d.Roles) > 8 {
		return nil, ErrDenied
	}
	roles := map[string]Role{}
	for name, r := range d.Roles {
		r.Tools = append([]string(nil), r.Tools...)
		roles[name] = r
	}
	tools := map[string]Tool{}
	for name, tool := range d.ParentTools {
		tools[name] = tool
	}
	d.Roles = roles
	d.ParentTools = tools
	policy := map[string]map[string]bool{}
	for from, targets := range edges {
		if _, ok := roles[from]; !ok {
			return nil, ErrDenied
		}
		policy[from] = map[string]bool{}
		for _, to := range targets {
			r, ok := roles[to]
			if !ok || r.Name != to || r.Run == nil || from == to {
				return nil, ErrDenied
			}
			policy[from][to] = true
		}
	}
	task.Context = append([]Fact(nil), task.Context...)
	known := map[string]string{}
	for _, fact := range task.Context {
		if !validFact(fact) {
			return nil, ErrResult
		}
		if value, ok := known[fact.Key]; ok && value != fact.Value {
			return nil, ErrResult
		}
		known[fact.Key] = fact.Value
	}
	return &Handoff{state: HandoffState{ID: task.ID, Scope: task.Scope, Owner: owner, Status: "ready", Revision: 1, Facts: append([]Fact(nil), task.Context...)}, task: task, dispatcher: d, edges: policy, maxHops: maxHops, maxTurns: maxTurns, initialOwner: owner}, nil
}
func (h *Handoff) Snapshot() HandoffState {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.state
	out.Facts = append([]Fact(nil), out.Facts...)
	return out
}
func (h *Handoff) authorized(req HandoffRequest) bool {
	return req.ID == h.state.ID && req.Scope == h.state.Scope && req.From == h.state.Owner && req.Revision == h.state.Revision && h.state.Status == "ready"
}
func (h *Handoff) Transfer(ctx context.Context, req HandoffRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !h.authorized(req) || !h.edges[req.From][req.To] {
		return ErrHandoff
	}
	if h.state.Hops >= h.maxHops {
		return ErrBudget
	}
	h.state.Owner = req.To
	h.state.Hops++
	h.state.Revision++
	return nil
}
func (h *Handoff) Execute(ctx context.Context, req HandoffRequest) (Result, error) {
	h.mu.Lock()
	if err := ctx.Err(); err != nil {
		h.mu.Unlock()
		return Result{}, err
	}
	if !h.authorized(req) {
		h.mu.Unlock()
		return Result{}, ErrHandoff
	}
	if h.state.Turns >= h.maxTurns {
		h.state.Status = "failed"
		h.state.Revision++
		h.mu.Unlock()
		return Result{}, ErrBudget
	}
	h.state.Status = "running"
	h.state.Turns++
	h.state.Revision++
	task := h.task
	task.Context = append([]Fact(nil), h.state.Facts...)
	owner := h.state.Owner
	h.mu.Unlock()
	output, err := h.dispatcher.Delegate(ctx, task, owner)
	h.mu.Lock()
	defer h.mu.Unlock()
	if err == nil {
		err = ctx.Err()
	}
	combined := append([]Fact(nil), h.state.Facts...)
	if err == nil {
		for _, fact := range output.Facts {
			found := false
			for _, old := range combined {
				if old.Key == fact.Key {
					found = true
					if old.Value != fact.Value {
						err = ErrResult
					}
				}
			}
			if !found {
				combined = append(combined, fact)
			}
		}
		if len(combined) > 8 {
			err = ErrResult
		}
	}
	h.state.Revision++
	if err != nil {
		h.state.Status = "failed"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			h.state.Status = "canceled"
		}
		return Result{}, err
	}
	h.state.Facts = combined
	h.state.Status = "ready"
	return output, nil
}
func (h *Handoff) Complete(ctx context.Context, req HandoffRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !h.authorized(req) {
		return ErrHandoff
	}
	h.state.Status = "completed"
	h.state.Revision++
	return nil
}
func handoffDemo() (HandoffState, error) {
	d := Dispatcher{Roles: roles(), ParentTools: tools(), Ledger: &Ledger{Limit: 2}, MaxDepth: 2}
	task := Task{ID: "report", Parent: "request", Scope: Scope{"demo", "reader"}, Goal: "read timeout and retry"}
	h, err := NewHandoff(task, d, "timeout-reader", map[string][]string{"timeout-reader": {"retry-reader"}}, 2, 4)
	if err != nil {
		return HandoffState{}, err
	}
	request := func(to string) HandoffRequest {
		s := h.Snapshot()
		return HandoffRequest{ID: s.ID, Scope: s.Scope, From: s.Owner, To: to, Revision: s.Revision}
	}
	if _, err = h.Execute(context.Background(), request("")); err != nil {
		return HandoffState{}, err
	}
	if err = h.Transfer(context.Background(), request("retry-reader")); err != nil {
		return HandoffState{}, err
	}
	if _, err = h.Execute(context.Background(), request("")); err != nil {
		return HandoffState{}, err
	}
	if err = h.Complete(context.Background(), request("")); err != nil {
		return HandoffState{}, err
	}
	return h.Snapshot(), nil
}
