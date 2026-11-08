package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buildJointFee(t *testing.T) (string, string) {
	t.Helper()
	program := filepath.Join(t.TempDir(), "cost-cli")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", program, "../../chapter33/cost-ledger")
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %v %s", err, data)
	}
	data, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	return program, hex.EncodeToString(h[:])
}
func jointRoots(t *testing.T, program, digest string) (Store, FeeCLI, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	feeDir := t.TempDir()
	if err := os.Chmod(feeDir, 0700); err != nil {
		t.Fatal(err)
	}
	return open(t, dir), FeeCLI{Program: program, Digest: digest, Directory: feeDir, Run: id()}, dir
}
func jointRead(root *os.Root) func(context.Context, string) JointResponse {
	return func(_ context.Context, action string) JointResponse {
		f, err := root.OpenFile("joint-calls.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return JointResponse{Failure: "tool_failed"}
		}
		_, err = f.WriteString(action + "\n")
		closeErr := f.Close()
		usage := int64(40)
		if err != nil || closeErr != nil {
			return JointResponse{InputUsage: &usage, Failure: "tool_failed"}
		}
		return JointResponse{Text: "fixed:" + action, InputUsage: &usage}
	}
}
func TestJointCrashWorker(t *testing.T) {
	if os.Getenv("JOINT_WORKER") != "1" {
		t.Skip("helper process")
	}
	s := open(t, os.Getenv("JOINT_DIR"))
	f := FeeCLI{Program: os.Getenv("JOINT_PROGRAM"), Digest: os.Getenv("JOINT_DIGEST"), Directory: os.Getenv("JOINT_FEES"), Run: id()}
	_, err := RunJoint(context.Background(), s, id(), actions[:1], 4, false, f, jointRead(s.Root), func(stage string) {
		if stage == os.Getenv("JOINT_STAGE") {
			os.Exit(93)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Fatal("fault did not trigger")
}
func TestJointHardExitRecoversOnlyMissingCommit(t *testing.T) {
	program, digest := buildJointFee(t)
	for _, stage := range []string{"after_intent", "after_fee_prepare", "after_binding", "after_fee_begin", "after_execute", "after_receipt", "after_fee_settle", "after_result_commit"} {
		t.Run(stage, func(t *testing.T) {
			s, f, dir := jointRoots(t, program, digest)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJointCrashWorker$")
			cmd.Env = append(os.Environ(), "JOINT_WORKER=1", "JOINT_DIR="+dir, "JOINT_PROGRAM="+program, "JOINT_DIGEST="+digest, "JOINT_FEES="+f.Directory, "JOINT_STAGE="+stage)
			data, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 93 {
				t.Fatalf("worker %v %s", err, data)
			}
			if err := s.Root.Remove("writer.lock"); err != nil {
				t.Fatal(err)
			}
			state, err := RunJoint(context.Background(), s, id(), actions[:1], 4, false, f, jointRead(s.Root), nil)
			unknown := stage == "after_fee_begin" || stage == "after_execute"
			if unknown {
				if !errors.Is(err, ErrUnknown) || state.Cursor != 0 {
					t.Fatal(state, err)
				}
			} else if err != nil || state.Cursor != 1 || len(state.Results) != 1 || len(state.Calls) != 1 {
				t.Fatal(state, err)
			}
			log, readErr := s.Root.ReadFile("joint-calls.log")
			want := 1
			if stage == "after_fee_begin" {
				want = 0
			}
			if readErr != nil && !(want == 0 && errors.Is(readErr, os.ErrNotExist)) || strings.Count(string(log), actions[0]+"\n") != want {
				t.Fatal(string(log), readErr, want)
			}
			reply, err := f.Invoke(context.Background(), FeeRequest{Action: "snapshot"})
			if err != nil {
				t.Fatal(err)
			}
			if len(reply.Snapshot.Entries) != 1 {
				t.Fatal(reply)
			}
			if unknown {
				if reply.Snapshot.Held != 50 || reply.Snapshot.Used != 0 {
					t.Fatal(reply)
				}
			} else if reply.Snapshot.Held != 0 || reply.Snapshot.Used != 40 {
				t.Fatal(reply)
			}
		})
	}
}
func TestJointReadonlyRetryKeepsUnknownAndOriginalBudget(t *testing.T) {
	program, digest := buildJointFee(t)
	s, f, _ := jointRoots(t, program, digest)
	// Cancel at this cooperative point leaves a confirmed sent intent without a tool result.
	ctx, cancel := context.WithCancel(context.Background())
	_, err := RunJoint(ctx, s, id(), actions[:1], 2, false, f, jointRead(s.Root), func(stage string) {
		if stage == "after_fee_begin" {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := RunJoint(context.Background(), s, id(), actions[:1], 3, true, f, jointRead(s.Root), nil); !errors.Is(err, ErrState) {
		t.Fatal("budget changed", err)
	}
	state, err := RunJoint(context.Background(), s, id(), actions[:1], 2, true, f, jointRead(s.Root), nil)
	if err != nil || state.Cursor != 1 || len(state.Calls) != 2 || state.Calls[0].Phase != "unknown" {
		t.Fatal(state, err)
	}
	r, err := f.Invoke(context.Background(), FeeRequest{Action: "snapshot"})
	if err != nil || r.Snapshot.Held != 50 || r.Snapshot.Used != 40 || len(r.Snapshot.Entries) != 2 {
		t.Fatal(r, err)
	}
}
func TestJointKnownFailureAndCanceledReceiptKeepFees(t *testing.T) {
	program, digest := buildJointFee(t)
	for _, kind := range []string{"tool_failed", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			s, f, _ := jointRoots(t, program, digest)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			usage := int64(20)
			state, err := RunJoint(ctx, s, id(), actions[:1], 3, false, f, func(context.Context, string) JointResponse {
				if kind == "canceled" {
					cancel()
				}
				return JointResponse{Text: "unpublished", InputUsage: &usage, Failure: "tool_failed"}
			}, nil)
			if kind == "canceled" {
				if !errors.Is(err, context.Canceled) || state.Calls[0].Phase != "received" {
					t.Fatal(state, err)
				}
				state, err = RunJoint(context.Background(), s, id(), actions[:1], 3, false, f, func(context.Context, string) JointResponse { t.Fatal("recalled tool"); return JointResponse{} }, nil)
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrJointTool) {
				t.Fatal(err)
			}
			if state.Cursor != 0 || len(state.Results) != 0 {
				t.Fatal("failed observation committed", state)
			}
			r, err := f.Invoke(context.Background(), FeeRequest{Action: "snapshot"})
			if err != nil || r.Snapshot.Used != 20 || r.Snapshot.Held != 0 {
				t.Fatal(r, err)
			}
		})
	}
}

func TestJointUnknownUsageOverrunAndMissingLedgerRemainExplicit(t *testing.T) {
	program, digest := buildJointFee(t)
	for _, kind := range []string{"unknown-usage", "overrun", "missing-ledger"} {
		t.Run(kind, func(t *testing.T) {
			s, f, _ := jointRoots(t, program, digest)
			usage := int64(40)
			if kind == "overrun" {
				usage = 110
			}
			state, err := RunJoint(context.Background(), s, id(), actions[:1], 3, false, f, func(context.Context, string) JointResponse {
				if kind == "unknown-usage" {
					return JointResponse{Text: "fixed"}
				}
				return JointResponse{Text: "fixed", InputUsage: &usage}
			}, nil)
			if kind == "overrun" {
				if !errors.Is(err, ErrBudget) || state.Cursor != 0 || state.Calls[0].Failure != "overrun" {
					t.Fatal(state, err)
				}
			} else if err != nil || state.Cursor != 1 {
				t.Fatal(state, err)
			}
			if kind == "missing-ledger" {
				if err := os.Remove(filepath.Join(f.Directory, "ledger.json")); err != nil {
					t.Fatal(err)
				}
				if _, err := RunJoint(context.Background(), s, id(), actions[:1], 3, false, f, func(context.Context, string) JointResponse {
					t.Fatal("lost ledger recalled tool")
					return JointResponse{}
				}, nil); !errors.Is(err, ErrState) {
					t.Fatal("missing paid record accepted", err)
				}
			} else {
				reply, err := f.Invoke(context.Background(), FeeRequest{Action: "snapshot"})
				if err != nil {
					t.Fatal(err)
				}
				if kind == "unknown-usage" && (reply.Snapshot.Used != 0 || reply.Snapshot.Held != 50 || state.Calls[0].UsageKnown) {
					t.Fatal(reply, state)
				}
				if kind == "overrun" && (reply.Snapshot.Used != 110 || !reply.Snapshot.Frozen) {
					t.Fatal(reply)
				}
			}
		})
	}
}
