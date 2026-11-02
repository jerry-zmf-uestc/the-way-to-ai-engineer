package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
)

var (
	ErrState    = errors.New("invalid_approval_state")
	ErrDenied   = errors.New("not_authorized")
	ErrConflict = errors.New("approval_snapshot_conflict")
	ErrExpired  = errors.New("approval_expired")
	ErrBusy     = errors.New("writer_busy")
	ErrUnknown  = errors.New("execution_result_unknown")
)

type Scope struct{ Tenant, User string }
type Action struct{ Tool, DocumentID string }
type Policy struct {
	Run, Version    string
	Owner, Reviewer Scope
	Documents       map[string]bool
}
type Event struct {
	Revision           uint64
	Fingerprint, State string
	Actor              Scope
	At                 time.Time
}

type Record struct {
	Schema             int
	Run                string
	Owner, Reviewer    Scope
	Revision           uint64
	PolicyVersion      string
	Action             Action
	ExpiresAt          time.Time
	Fingerprint, State string
	DecisionBy         Scope
	Attempts           int
	Output, Failure    string
	History            []Event
}
type Reader func(context.Context, Action) (string, error)
type Store struct {
	root    *os.Root
	policy  Policy
	reader  Reader
	revoked atomic.Bool
}

func NewStore(dir string, policy Policy, reader Reader) (*Store, error) {
	if policy.Run == "" || policy.Version == "" || policy.Owner.Tenant == "" || policy.Owner.User == "" || policy.Reviewer.Tenant != policy.Owner.Tenant || policy.Reviewer.User == "" || reader == nil {
		return nil, ErrState
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, ErrDenied
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	copied := map[string]bool{}
	for id, allowed := range policy.Documents {
		if allowed {
			copied[id] = true
		}
	}
	policy.Documents = copied
	return &Store{root: root, policy: policy, reader: reader}, nil
}
func (s *Store) Close() error { return s.root.Close() }
func (s *Store) Revoke()      { s.revoked.Store(true) }
func fingerprint(r Record) string {
	// Only immutable approval scope belongs in the fingerprint, not mutable execution status/output.
	snapshot := struct {
		Run             string
		Owner, Reviewer Scope
		Revision        uint64
		PolicyVersion   string
		Action          Action
		ExpiresAt       time.Time
	}{r.Run, r.Owner, r.Reviewer, r.Revision, r.PolicyVersion, r.Action, r.ExpiresAt}
	data, _ := json.Marshal(snapshot)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *Store) allowed(action Action) bool {
	return !s.revoked.Load() && action.Tool == "read_document" && s.policy.Documents[action.DocumentID]
}
func (s *Store) authorized(r Record) bool {
	return r.Run == s.policy.Run && r.Owner == s.policy.Owner && r.Reviewer == s.policy.Reviewer && r.PolicyVersion == s.policy.Version && s.allowed(r.Action)
}
func (s *Store) load() (Record, error) {
	f, err := s.root.Open("approval.json")
	if err != nil {
		return Record{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	if err != nil {
		return Record{}, err
	}
	if len(data) > 16<<10 {
		return Record{}, ErrState
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var r Record
	if err := decoder.Decode(&r); err != nil {
		return Record{}, ErrState
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Record{}, ErrState
	}
	if r.Schema != 1 || r.Run != s.policy.Run || r.Owner != s.policy.Owner || r.Revision < 1 || r.Revision > 32 || r.Attempts < 0 || r.Attempts > 2 || len(r.History) > 32 || len(r.Output) > 4096 || r.Fingerprint != fingerprint(r) {
		return Record{}, ErrState
	}
	switch r.State {
	case "waiting", "approved", "executing", "completed", "rejected", "canceled", "expired", "failed":
	default:
		return Record{}, ErrState
	}
	if (r.State == "approved" || r.State == "executing" || r.State == "completed") && r.DecisionBy != r.Reviewer {
		return Record{}, ErrState
	}
	if r.State != "completed" && r.Output != "" {
		return Record{}, ErrState
	}
	return r, nil
}
func (s *Store) save(r *Record) error {
	if len(r.History) >= 32 {
		return ErrState
	}
	actor := r.Owner
	if r.State == "approved" || r.State == "rejected" {
		actor = r.DecisionBy
	}
	r.History = append(r.History, Event{Revision: r.Revision, Fingerprint: r.Fingerprint, State: r.State, Actor: actor, At: time.Now().UTC()})
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(data) > 16<<10 {
		return ErrState
	}
	f, err := s.root.OpenFile("approval.tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove("approval.tmp")
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = s.root.Rename("approval.tmp", "approval.json"); err != nil {
		return err
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *Store) commit(r Record) (Record, error) { err := s.save(&r); return r, err }

// change uses an exclusive local writer claim; no process owns it while a human is deciding.
func (s *Store) change(f func() (Record, error)) (r Record, err error) {
	lock, err := s.root.OpenFile("writer.lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return Record{}, ErrBusy
	}
	if err != nil {
		return Record{}, err
	}
	if _, err = fmt.Fprintf(lock, "pid=%d\n", os.Getpid()); err != nil {
		lock.Close()
		s.root.Remove("writer.lock")
		return Record{}, err
	}
	if err = lock.Close(); err != nil {
		s.root.Remove("writer.lock")
		return Record{}, err
	}
	defer func() {
		err = errors.Join(err, s.root.Remove("writer.lock"))
		if err != nil {
			r.Output = ""
			if errors.Is(err, ErrDenied) {
				r = Record{}
			}
		}
	}()
	return f()
}
func (s *Store) Create(actor Scope, action Action, ttl time.Duration) (Record, error) {
	return s.change(func() (Record, error) {
		if actor != s.policy.Owner || !s.allowed(action) || ttl <= 0 || ttl > time.Hour {
			return Record{}, ErrDenied
		}
		if _, err := s.root.Stat("approval.json"); !errors.Is(err, os.ErrNotExist) {
			return Record{}, ErrState
		}
		r := Record{Schema: 1, Run: s.policy.Run, Owner: actor, Reviewer: s.policy.Reviewer, Revision: 1, PolicyVersion: s.policy.Version, Action: action, ExpiresAt: time.Now().UTC().Add(ttl), State: "waiting"}
		r.Fingerprint = fingerprint(r)
		return s.commit(r)
	})
}
func (s *Store) Show(actor Scope) (Record, error) {
	if actor != s.policy.Owner && actor != s.policy.Reviewer {
		return Record{}, ErrDenied
	}
	r, err := s.load()
	if err != nil {
		return Record{}, err
	}
	if r.State == "completed" && !s.authorized(r) {
		r.Output = ""
		return r, ErrDenied
	}
	return r, nil
}
func (s *Store) check(r Record, revision uint64, hash string) error {
	if r.Revision != revision || hash == "" || r.Fingerprint != hash {
		return ErrConflict
	}
	if !time.Now().Before(r.ExpiresAt) {
		return ErrExpired
	}
	return nil
}
func (s *Store) Decide(actor Scope, revision uint64, hash string, approve bool) (Record, error) {
	return s.change(func() (Record, error) {
		r, err := s.load()
		if err != nil {
			return r, err
		}
		if actor != s.policy.Reviewer || actor != r.Reviewer {
			return r, ErrDenied
		}
		if err = s.check(r, revision, hash); err != nil {
			return r, err
		}
		target := "rejected"
		if approve {
			target = "approved"
			if !s.authorized(r) {
				return r, ErrDenied
			}
		}
		if r.State == target && r.DecisionBy == actor {
			return r, nil
		}
		if r.State != "waiting" {
			return r, ErrConflict
		}
		r.State = target
		r.DecisionBy = actor
		return s.commit(r)
	})
}
func (s *Store) Modify(actor Scope, revision uint64, hash string, action Action) (Record, error) {
	return s.change(func() (Record, error) {
		r, err := s.load()
		if err != nil {
			return r, err
		}
		if actor != s.policy.Owner || !s.allowed(action) {
			return r, ErrDenied
		}
		if err = s.check(r, revision, hash); err != nil {
			return r, err
		}
		if (r.State != "waiting" && r.State != "approved") || r.Revision >= 32 {
			return r, ErrConflict
		}
		r.Revision++
		r.Action = action
		r.PolicyVersion = s.policy.Version
		r.Reviewer = s.policy.Reviewer
		r.DecisionBy = Scope{}
		r.State = "waiting"
		r.Fingerprint = fingerprint(r)
		return s.commit(r)
	})
}
func (s *Store) Cancel(actor Scope, revision uint64, hash string) (Record, error) {
	return s.change(func() (Record, error) {
		r, err := s.load()
		if err != nil {
			return r, err
		}
		if actor != s.policy.Owner {
			return r, ErrDenied
		}
		if err = s.check(r, revision, hash); err != nil {
			return r, err
		}
		if r.State == "canceled" {
			return r, nil
		}
		if r.State != "waiting" && r.State != "approved" {
			return r, ErrConflict
		}
		r.State = "canceled"
		r.DecisionBy = Scope{}
		return s.commit(r)
	})
}

// Resume commits executing before Reader. An executing record after restart is not automatically replayed.
func (s *Store) Resume(ctx context.Context, actor Scope, revision uint64, hash string) (Record, error) {
	return s.change(func() (Record, error) {
		r, err := s.load()
		if err != nil {
			return r, err
		}
		if actor != s.policy.Owner {
			return r, ErrDenied
		}
		if err = s.check(r, revision, hash); err != nil {
			return r, err
		}
		if !s.authorized(r) {
			return r, ErrDenied
		}
		if r.State == "completed" {
			return r, nil
		}
		if r.State == "executing" {
			return r, ErrUnknown
		}
		if r.State != "approved" || r.Attempts >= 2 || len(r.History) > 30 {
			return r, ErrConflict
		}
		if err = ctx.Err(); err != nil {
			return r, err
		}
		r.State = "executing"
		r.Attempts++
		if err = s.save(&r); err != nil {
			return r, err
		}
		text, callErr := s.reader(ctx, r.Action)
		if stop := ctx.Err(); stop != nil {
			callErr = stop
		}
		switch {
		case errors.Is(callErr, context.Canceled):
			r.State = "canceled"
			r.Failure = "canceled"
			callErr = context.Canceled
		case errors.Is(callErr, context.DeadlineExceeded):
			r.State = "failed"
			r.Failure = "deadline_exceeded"
			callErr = context.DeadlineExceeded
		case !s.authorized(r):
			r.State = "failed"
			r.Failure = "authorization_changed"
			callErr = ErrDenied
		case !time.Now().Before(r.ExpiresAt):
			r.State = "expired"
			r.Failure = "expired"
			callErr = ErrExpired
		case callErr != nil:
			r.State = "failed"
			r.Failure = "reader_failed"
			callErr = ErrState
		case text == "" || len(text) > 4096:
			r.State = "failed"
			r.Failure = "invalid_output"
			callErr = ErrState
		default:
			r.State = "completed"
			r.Output = text
		}
		saveErr := s.save(&r)
		return r, errors.Join(callErr, saveErr)
	})
}
func (s *Store) RetryReadOnly(actor Scope, revision uint64, hash string) (Record, error) {
	return s.change(func() (Record, error) {
		r, err := s.load()
		if err != nil {
			return r, err
		}
		if actor != s.policy.Reviewer {
			return r, ErrDenied
		}
		if err = s.check(r, revision, hash); err != nil {
			return r, err
		}
		if !s.authorized(r) || r.State != "executing" || r.Attempts >= 2 {
			return r, ErrConflict
		}
		// The only registered capability is trusted readonly read_document; never use this for business writes.
		r.State = "approved"
		return s.commit(r)
	})
}
func demoPolicy() Policy {
	return Policy{Run: "demo-run", Version: "policy-v1", Owner: Scope{Tenant: "demo", User: "alice"}, Reviewer: Scope{Tenant: "demo", User: "reviewer"}, Documents: map[string]bool{"guide": true, "alternate": true}}
}
func readApproved(ctx context.Context, a Action) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text, ok := map[string]string{"guide": "Original readonly guide.", "alternate": "Alternate readonly guide."}[a.DocumentID]
	if !ok || a.Tool != "read_document" {
		return "", ErrDenied
	}
	return text, nil
}
func main() {
	dir := flag.String("dir", "", "private state directory")
	op := flag.String("action", "create", "create/show/approve/reject/modify/cancel/resume/retry-readonly")
	rev := flag.Uint64("revision", 0, "expected revision")
	hash := flag.String("fingerprint", "", "fingerprint reviewed by the human")
	document := flag.String("document", "guide", "registered document ID")
	flag.Parse()
	if *dir == "" {
		var err error
		*dir, err = os.MkdirTemp("", "hitl-lab-")
		if err != nil {
			panic(err)
		}
	}
	policy := demoPolicy()
	store, err := NewStore(*dir, policy, readApproved)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var r Record
	switch *op {
	case "create":
		r, err = store.Create(policy.Owner, Action{Tool: "read_document", DocumentID: *document}, 10*time.Minute)
	case "show":
		r, err = store.Show(policy.Owner)
	case "approve":
		r, err = store.Decide(policy.Reviewer, *rev, *hash, true)
	case "reject":
		r, err = store.Decide(policy.Reviewer, *rev, *hash, false)
	case "modify":
		r, err = store.Modify(policy.Owner, *rev, *hash, Action{Tool: "read_document", DocumentID: *document})
	case "cancel":
		r, err = store.Cancel(policy.Owner, *rev, *hash)
	case "resume":
		r, err = store.Resume(context.Background(), policy.Owner, *rev, *hash)
	case "retry-readonly":
		r, err = store.RetryReadOnly(policy.Reviewer, *rev, *hash)
	default:
		err = ErrState
	}
	err = errors.Join(err, store.Close())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("state_dir=%s\n", *dir)
	_ = json.NewEncoder(os.Stdout).Encode(r)
}
