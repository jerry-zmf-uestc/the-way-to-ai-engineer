// Package tasks is the bounded same-process task contract shared by independent course Labs.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrInput    = errors.New("invalid input")
	ErrNotFound = errors.New("task not found")
	ErrConflict = errors.New("revision or idempotency conflict")
	ErrCapacity = errors.New("task capacity exhausted")
	ErrClosed   = errors.New("manager closed")
)

type Scope struct{ Tenant, User string }
type Task struct {
	ID         string
	Scope      Scope
	Input      string
	Revision   uint64
	Attempt    int
	State      string
	Output     string
	Failure    string
	FinishedAt time.Time
}
type Worker func(context.Context, Task) (string, error)
type startKey struct {
	Scope Scope
	Key   string
}
type record struct {
	task    Task
	key     startKey
	initial string
	cancel  context.CancelFunc
	life    context.Context
	stop    context.CancelFunc
	done    chan struct{}
}
type Manager struct {
	// ponytail: one mutex and at most 16 records; persistent/multi-instance ownership needs a separate store contract.
	mu            sync.Mutex
	records       map[string]*record
	keys          map[startKey]string
	worker        Worker
	limit, active int
	timeout       time.Duration
	serial        uint64
	closed        bool
	wg            sync.WaitGroup
}

func NewManager(limit int, timeout time.Duration, worker Worker) (*Manager, error) {
	if limit < 1 || limit > 8 || timeout <= 0 || timeout > time.Minute || worker == nil {
		return nil, ErrInput
	}
	return &Manager{records: map[string]*record{}, keys: map[startKey]string{}, worker: worker, limit: limit, timeout: timeout}, nil
}
func validScope(s Scope) bool {
	return s.Tenant != "" && s.User != "" && len(s.Tenant) <= 128 && len(s.User) <= 128
}
func validInput(input string) bool { return input != "" && len(input) <= 4096 }

// Start belongs to a trusted application caller. Scope must not come from a model-selected argument.
// Its lifetime is independent of a request's context; application shutdown calls Close.
func (m *Manager) Start(scope Scope, key, input string) (Task, error) {
	if !validScope(scope) || key == "" || len(key) > 128 || !validInput(input) {
		return Task{}, ErrInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Task{}, ErrClosed
	}
	k := startKey{scope, key}
	if id, ok := m.keys[k]; ok {
		r := m.records[id]
		if r.initial != input {
			return Task{}, ErrConflict
		}
		return r.task, nil
	}
	if m.active >= m.limit || len(m.records) >= 16 {
		return Task{}, ErrCapacity
	}
	m.serial++
	life, stop := context.WithTimeout(context.Background(), m.timeout)
	ctx, cancel := context.WithCancel(life)
	r := &record{task: Task{ID: fmt.Sprintf("task-%d", m.serial), Scope: scope, Input: input, Revision: 1, State: "running"}, key: k, initial: input, cancel: cancel, life: life, stop: stop, done: make(chan struct{})}
	m.records[r.task.ID] = r
	m.keys[k] = r.task.ID
	m.active++
	m.wg.Add(1)
	snapshot := r.task
	go m.run(ctx, r, snapshot)
	return snapshot, nil
}

func (m *Manager) lookup(scope Scope, id string) (*record, error) {
	r, ok := m.records[id]
	if !ok || r.task.Scope != scope {
		return nil, ErrNotFound
	}
	return r, nil
}
func (m *Manager) Check(scope Scope, id string) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.lookup(scope, id)
	if err != nil {
		return Task{}, err
	}
	return r.task, nil
}
func (m *Manager) List(scope Scope) []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	tasks := []Task{}
	for _, r := range m.records {
		if r.task.Scope == scope {
			tasks = append(tasks, r.task)
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return tasks
}

// Update invalidates the running attempt. A second attempt starts only after the old one drains.
// One update per task bounds repeated repair; start a new task for another revision.
func (m *Manager) Update(scope Scope, id string, expected uint64, input string) (Task, error) {
	if !validInput(input) {
		return Task{}, ErrInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.lookup(scope, id)
	if err != nil {
		return Task{}, err
	}
	if m.closed {
		return Task{}, ErrClosed
	}
	if r.task.State != "running" || r.task.Revision != expected || r.task.Revision >= 2 {
		return Task{}, ErrConflict
	}
	r.task.Input = input
	r.task.Revision++
	r.task.State = "updating"
	r.cancel()
	return r.task, nil
}
func (m *Manager) Cancel(scope Scope, id string, expected uint64) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.lookup(scope, id)
	if err != nil {
		return Task{}, err
	}
	if r.task.Revision != expected {
		return Task{}, ErrConflict
	}
	if r.task.State == "canceled" || r.task.State == "canceling" {
		return r.task, nil
	}
	if r.task.State != "running" && r.task.State != "updating" {
		return Task{}, ErrConflict
	}
	r.task.Revision++
	r.task.State = "canceling"
	r.cancel()
	return r.task, nil
}

func (m *Manager) run(ctx context.Context, r *record, snapshot Task) {
	defer m.wg.Done()
	for {
		m.mu.Lock()
		invoke := ctx.Err() == nil && r.task.State != "canceling" && !m.closed
		if invoke {
			r.task.Attempt++
			snapshot.Attempt = r.task.Attempt
		}
		m.mu.Unlock()
		output, err := "", ctx.Err()
		if invoke {
			output, err = m.worker(ctx, snapshot)
		} else if err == nil {
			err = context.Canceled
		}
		m.mu.Lock()
		// The mutex orders update, cancellation and completion; stale output cannot become a result.
		if r.task.State == "canceling" || m.closed {
			r.task.State = "canceled"
		} else if r.life.Err() != nil {
			r.task.State = "failed"
			r.task.Failure = "deadline_exceeded"
		} else if r.task.Revision != snapshot.Revision {
			ctx, r.cancel = context.WithCancel(r.life)
			r.task.State = "running"
			snapshot = r.task
			m.mu.Unlock()
			continue
		} else if errors.Is(err, context.Canceled) {
			r.task.State = "canceled"
			r.task.Failure = "worker_canceled"
		} else if errors.Is(err, context.DeadlineExceeded) {
			r.task.State = "failed"
			r.task.Failure = "worker_deadline_exceeded"
		} else if err != nil {
			r.task.State = "failed"
			r.task.Failure = "worker_failed" // Do not persist arbitrary err.Error() in a public task status.
		} else if output == "" || len(output) > 8192 {
			r.task.State = "failed"
			r.task.Failure = "invalid_output"
		} else {
			r.task.State = "completed"
			r.task.Output = output
		}
		r.cancel()
		r.stop()
		r.task.FinishedAt = time.Now()
		m.active--
		close(r.done)
		m.mu.Unlock()
		return
	}
}

func (m *Manager) Wait(ctx context.Context, scope Scope, id string) (Task, error) {
	m.mu.Lock()
	r, err := m.lookup(scope, id)
	if err != nil {
		m.mu.Unlock()
		return Task{}, err
	}
	done := r.done
	m.mu.Unlock()
	select {
	case <-ctx.Done():
		return Task{}, ctx.Err()
	case <-done:
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return r.task, nil
}

// Cleanup expires both terminal records and their idempotency window. IDs are never reused.
func (m *Manager) Cleanup(now time.Time, ttl time.Duration) (int, error) {
	if ttl <= 0 {
		return 0, ErrInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, r := range m.records {
		if !r.task.FinishedAt.IsZero() && !now.Before(r.task.FinishedAt.Add(ttl)) {
			delete(m.records, id)
			delete(m.keys, r.key)
			count++
		}
	}
	return count, nil
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, r := range m.records {
		if r.task.State == "running" || r.task.State == "updating" {
			r.task.Revision++
			r.task.State = "canceling"
			r.cancel()
		}
	}
	m.mu.Unlock()
	m.wg.Wait()
}
