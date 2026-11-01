package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T, reader Reader) (*Store, Policy, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	p := demoPolicy()
	s, err := NewStore(dir, p, reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, p, dir
}
func request(t *testing.T, s *Store, p Policy) Record {
	t.Helper()
	r, err := s.Create(p.Owner, Action{Tool: "read_document", DocumentID: "guide"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func approve(t *testing.T, s *Store, p Policy, r Record) Record {
	t.Helper()
	r, err := s.Decide(p.Reviewer, r.Revision, r.Fingerprint, true)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestPauseAcrossStoreRestartThenExplicitDecision(t *testing.T) {
	var calls atomic.Int32
	reader := func(ctx context.Context, a Action) (string, error) { calls.Add(1); return readApproved(ctx, a) }
	s, p, dir := fixture(t, reader)
	r := request(t, s, p)
	if _, err := s.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint); !errors.Is(err, ErrConflict) || calls.Load() != 0 {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := NewStore(dir, p, reader)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	r, err = next.Show(p.Owner)
	if err != nil || r.State != "waiting" {
		t.Fatal(r, err)
	}
	r = approve(t, next, p, r)
	if _, err = next.Decide(p.Reviewer, r.Revision, r.Fingerprint, true); err != nil {
		t.Fatal(err)
	}
	result, err := next.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint)
	if err != nil || result.State != "completed" || calls.Load() != 1 || result.Output != "Original readonly guide." {
		t.Fatal(result, err)
	}
	if _, err = next.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint); err != nil || calls.Load() != 1 {
		t.Fatal(err)
	}
	if len(result.History) != 4 || result.History[1].Actor != p.Reviewer {
		t.Fatal(result.History)
	}
}
func TestModifyInvalidatesApprovalAndCannotExpandPolicy(t *testing.T) {
	s, p, _ := fixture(t, readApproved)
	old := approve(t, s, p, request(t, s, p))
	changed, err := s.Modify(p.Owner, old.Revision, old.Fingerprint, Action{Tool: "read_document", DocumentID: "alternate"})
	if err != nil || changed.Revision != 2 || changed.State != "waiting" || changed.DecisionBy != (Scope{}) {
		t.Fatal(changed, err)
	}
	if _, err = s.Decide(p.Reviewer, old.Revision, old.Fingerprint, true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = s.Modify(p.Owner, changed.Revision, changed.Fingerprint, Action{Tool: "delete_database", DocumentID: "guide"}); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	current := approve(t, s, p, changed)
	result, err := s.Resume(context.Background(), p.Owner, current.Revision, current.Fingerprint)
	if err != nil || result.Output != "Alternate readonly guide." {
		t.Fatal(result, err)
	}
}
func TestScopeRejectCancelExpiryAndPolicyVersion(t *testing.T) {
	s, p, dir := fixture(t, readApproved)
	r := request(t, s, p)
	stranger := Scope{Tenant: "demo", User: "stranger"}
	if _, err := s.Show(stranger); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	if _, err := s.Decide(p.Owner, r.Revision, r.Fingerprint, true); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	if _, err := s.Decide(p.Reviewer, r.Revision, "wrong", true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	rejected, err := s.Decide(p.Reviewer, r.Revision, r.Fingerprint, false)
	if err != nil || rejected.State != "rejected" {
		t.Fatal(err)
	}
	if _, err = s.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	newer := p
	newer.Version = "policy-v2"
	other, err := NewStore(dir, newer, readApproved)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err = other.Decide(p.Reviewer, r.Revision, r.Fingerprint, true); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	canceled, p2, _ := fixture(t, readApproved)
	r2 := request(t, canceled, p2)
	if _, err = canceled.Cancel(p2.Owner, r2.Revision, r2.Fingerprint); err != nil {
		t.Fatal(err)
	}
	if _, err = canceled.Decide(p2.Reviewer, r2.Revision, r2.Fingerprint, true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	expired, p3, _ := fixture(t, readApproved)
	r3 := request(t, expired, p3)
	_, err = expired.change(func() (Record, error) {
		r3.ExpiresAt = time.Now().Add(-time.Second)
		r3.Fingerprint = fingerprint(r3)
		return r3, expired.save(&r3)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = expired.Decide(p3.Reviewer, r3.Revision, r3.Fingerprint, true); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
}
func TestRevokeAndCancelDiscardLateOutput(t *testing.T) {
	for _, revoke := range []bool{true, false} {
		entered := make(chan struct{})
		release := make(chan struct{})
		s, p, _ := fixture(t, func(context.Context, Action) (string, error) { close(entered); <-release; return "late", nil })
		r := approve(t, s, p, request(t, s, p))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result := make(chan error, 1)
		go func() {
			out, err := s.Resume(ctx, p.Owner, r.Revision, r.Fingerprint)
			if out.Output != "" {
				result <- errors.New("late released")
			} else {
				result <- err
			}
		}()
		<-entered
		if revoke {
			s.Revoke()
		} else {
			cancel()
		}
		close(release)
		want := ErrDenied
		if !revoke {
			want = context.Canceled
		}
		if err := <-result; !errors.Is(err, want) {
			t.Fatal(err)
		}
	}
}
func TestConcurrentDecisionsAndBusyWriter(t *testing.T) {
	s, p, _ := fixture(t, readApproved)
	r := request(t, s, p)
	var wg sync.WaitGroup
	var accepted atomic.Int32
	bad := make(chan error, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Decide(p.Reviewer, r.Revision, r.Fingerprint, true)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrBusy) {
				bad <- err
			}
		}()
	}
	wg.Wait()
	close(bad)
	for err := range bad {
		t.Fatal(err)
	}
	if accepted.Load() < 1 {
		t.Fatal("no decision committed")
	}
}
func TestCrashChild(t *testing.T) {
	dir := os.Getenv("HITL_CHILD_ROOT")
	if dir == "" {
		return
	}
	p := demoPolicy()
	s, err := NewStore(dir, p, func(ctx context.Context, a Action) (string, error) {
		_, _ = readApproved(ctx, a)
		os.Exit(42)
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Show(p.Owner)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint)
	t.Fatal("expected hard exit")
}
func TestHardExitRequiresExplicitReadOnlyRecovery(t *testing.T) {
	s, p, dir := fixture(t, readApproved)
	r := approve(t, s, p, request(t, s, p))
	_ = s.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashChild$")
	cmd.Env = append(os.Environ(), "HITL_CHILD_ROOT="+dir)
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatal(err)
	}
	next, err := NewStore(dir, p, readApproved)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if _, err = next.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	// Test operator has confirmed the exact child exited. Never infer stale ownership from lock age.
	if err = os.Remove(filepath.Join(dir, "writer.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err = next.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	recovered, err := next.RetryReadOnly(p.Reviewer, r.Revision, r.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	result, err := next.Resume(context.Background(), p.Owner, recovered.Revision, recovered.Fingerprint)
	if err != nil || result.State != "completed" || result.Attempts != 2 {
		t.Fatal(result, err)
	}
}

func TestRefusalDoesNotReturnPrivateSnapshotAndTamperFails(t *testing.T) {
	s, p, _ := fixture(t, readApproved)
	r := approve(t, s, p, request(t, s, p))
	stranger := Scope{Tenant: "other", User: "unknown"}
	out, err := s.Decide(stranger, r.Revision, r.Fingerprint, true)
	if !errors.Is(err, ErrDenied) || out.Run != "" || out.Output != "" {
		t.Fatal(out, err)
	}
	out, err = s.Resume(context.Background(), stranger, r.Revision, r.Fingerprint)
	if !errors.Is(err, ErrDenied) || out.Run != "" {
		t.Fatal(out, err)
	}
	_, err = s.change(func() (Record, error) {
		changed := r
		changed.Action.DocumentID = "alternate"
		return changed, s.save(&changed)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resume(context.Background(), p.Owner, r.Revision, r.Fingerprint); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
}
func TestPrivateDirectoryAndBoundedJournal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "public")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(dir, demoPolicy(), readApproved); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	s, p, _ := fixture(t, readApproved)
	r := request(t, s, p)
	for range 40 {
		next, err := s.Modify(p.Owner, r.Revision, r.Fingerprint, Action{Tool: "read_document", DocumentID: "guide"})
		if err != nil {
			if !errors.Is(err, ErrState) && !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			break
		}
		r = next
	}
	stored, err := s.Show(p.Owner)
	if err != nil || len(stored.History) > 32 {
		t.Fatal(stored, err)
	}
}
