package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToolOutcomeKeepsPartialErrorWithoutObservation(t *testing.T) {
	cause := errors.New("fixture private diagnostic")
	observed := -1
	factory := func() (*Handoff, error) {
		h, err := readonlyHandoffFactory()
		if err == nil {
			h.dispatcher.ParentTools["read-timeout"] = func(context.Context, Scope) (Fact, error) {
				return Fact{"timeout", "30", "guide@v1:timeout"}, cause
			}
			h.dispatcher.Roles["timeout-reader"] = Role{Name: "timeout-reader", Tools: []string{"read-timeout"}, Run: func(ctx context.Context, task Task, box *Toolbox) (Result, error) {
				_, err := box.Read(ctx, "read-timeout")
				observed = len(box.observed)
				return Result{}, err
			}}
		}
		return h, err
	}
	s, err := OpenHandoffStore(filepath.Join(t.TempDir(), "private"), factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); !errors.Is(err, cause) || observed != 0 {
		t.Fatal("partial failed result became observation or lost cause", err, observed)
	}
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Phase != "failed" || len(shown.Attempts) != 1 || shown.Attempts[0].Outcome != "error" || shown.Attempts[0].ErrorCode != "tool_error" || shown.Attempts[0].Raw.Value != "30" || len(shown.RoleResults) != 0 {
		t.Fatal(shown, err)
	}
	data, _ := json.Marshal(shown)
	if strings.Contains(string(data), cause.Error()) {
		t.Fatal("private diagnostic persisted")
	}
}

func TestToolReturnedBeforeRoleCommitSurvivesHardExit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	s, err := OpenHandoffStore(dir, readonlyHandoffFactory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(storeRequest(r, ""))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHandoffStoreProcessHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "HANDOFF_STORE_HELPER=1", "HANDOFF_STORE_DIR="+dir, "HANDOFF_STORE_OP=execute", "HANDOFF_STORE_REQUEST="+string(request), "HANDOFF_STORE_CRASH_AFTER_RETURN=1")
	_, err = cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatal("not a confirmed child hard exit", err)
	}
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Phase != "executing" || shown.Used != 1 || len(shown.Attempts) != 1 || shown.Attempts[0].Outcome != "returned" || shown.Attempts[0].Raw.Key != "timeout" || len(shown.RoleResults) != 0 || shown.Stable.State.Turns != 0 {
		t.Fatal("tool return confused with role commit", shown, err)
	}
}

func TestToolOutcomeRejectsUnboundedRawPayload(t *testing.T) {
	factory := func() (*Handoff, error) {
		h, err := readonlyHandoffFactory()
		if err == nil {
			h.dispatcher.ParentTools["read-timeout"] = func(context.Context, Scope) (Fact, error) {
				return Fact{"timeout", strings.Repeat("x", 4097), "source"}, nil
			}
		}
		return h, err
	}
	s, err := OpenHandoffStore(filepath.Join(t.TempDir(), "private"), factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); !errors.Is(err, ErrResult) {
		t.Fatal(err)
	}
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Attempts[0].Outcome != "rejected" || shown.Attempts[0].Raw != (Fact{}) || shown.Used != 1 {
		t.Fatal("bad raw result silently truncated or budget lost", shown, err)
	}
}

func TestToolReturnSaveFailureDoesNotReachObservation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	observed := -1
	factory := func() (*Handoff, error) {
		h, err := readonlyHandoffFactory()
		if err == nil {
			h.dispatcher.ParentTools["read-timeout"] = func(context.Context, Scope) (Fact, error) {
				if err := os.WriteFile(filepath.Join(dir, ".record.tmp"), []byte("occupied"), 0600); err != nil {
					return Fact{}, err
				}
				return Fact{"timeout", "30", "guide@v1:timeout"}, nil
			}
			h.dispatcher.Roles["timeout-reader"] = Role{Name: "timeout-reader", Tools: []string{"read-timeout"}, Run: func(ctx context.Context, task Task, box *Toolbox) (Result, error) {
				_, err := box.Read(ctx, "read-timeout")
				observed = len(box.observed)
				return Result{}, err
			}}
		}
		return h, err
	}
	s, err := OpenHandoffStore(dir, factory, "readonly-demo-v1")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Apply(context.Background(), "init", HandoffRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), "execute", storeRequest(r, "")); err == nil || observed != 0 {
		t.Fatal("uncommitted return became observation", err, observed)
	}
	shown, err := s.Apply(context.Background(), "show", HandoffRequest{})
	if err != nil || shown.Used != 1 || shown.Attempts[0].Outcome != "reserved" || shown.Attempts[0].Raw != (Fact{}) || len(shown.RoleResults) != 0 {
		t.Fatal("memory return masqueraded as durable result", shown, err)
	}
}
