package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFeeCLIActualProgramAndSessionBinding(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "cost-cli")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", program, "../../chapter33/cost-ledger")
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v %s", err, data)
	}
	data, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	feeDir := filepath.Join(dir, "fees")
	if err := os.Mkdir(feeDir, 0700); err != nil {
		t.Fatal(err)
	}
	f := FeeCLI{Program: program, Digest: hex.EncodeToString(digest[:]), Directory: feeDir, Run: id()}
	prepare := FeeRequest{Action: "prepare", Key: "attempt-1", Hash: strings.Repeat("a", 64)}
	for range 2 {
		r, err := f.Invoke(context.Background(), prepare)
		if err != nil || r.EntryID != 1 || r.Snapshot.Held != 50 {
			t.Fatal(r, err)
		}
	}
	r, err := f.Invoke(context.Background(), FeeRequest{Action: "begin", EntryID: 1})
	if err != nil || r.Snapshot.Entries[0].State != "sent" {
		t.Fatal(r, err)
	}
	r, err = f.Invoke(context.Background(), FeeRequest{Action: "snapshot"})
	if err != nil || r.Snapshot.Entries[0].State != "unknown" || r.Snapshot.Held != 50 {
		t.Fatal(r, err)
	}
	if _, err := f.Invoke(context.Background(), FeeRequest{Action: "begin", EntryID: 1}); !errors.Is(err, ErrFeeOperation) {
		t.Fatal("repeat dispatch allowed", err)
	}
	usage := int64(40)
	for range 2 {
		r, err = f.Invoke(context.Background(), FeeRequest{Action: "settle", EntryID: 1, InputUsage: &usage})
		if err != nil || r.Snapshot.Used != 40 || r.Snapshot.Held != 0 {
			t.Fatal(r, err)
		}
	}
	foreign := f
	foreign.Run.Session = "other"
	if foreign.identity() == f.identity() {
		t.Fatal("session lost in fee identity")
	}
	if _, err := foreign.Invoke(context.Background(), FeeRequest{Action: "snapshot"}); !errors.Is(err, ErrFeeOperation) {
		t.Fatal("cross-session ledger accepted", err)
	}
	bad := f
	bad.Digest = strings.Repeat("0", 64)
	if _, err := bad.Invoke(context.Background(), prepare); !errors.Is(err, ErrFeeProtocol) {
		t.Fatal("unverified program executed", err)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := f.Invoke(canceled, prepare); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func feeSuccess() (FeeReply, FeeRequest) {
	request := FeeRequest{Action: "prepare", Key: "attempt-1", Hash: strings.Repeat("a", 64)}
	r := FeeReply{Schema: 1, Action: "prepare", Identity: FeeIdentity{"t", "u", "fee-run"}, EntryID: 1, AttemptKey: request.Key, RequestHash: request.Hash,
		Snapshot: FeeSnapshot{Unit: "microcredit", Limit: 100, Held: 50, Entries: []FeeEntry{{ID: 1, State: "reserved", Price: FeePrice{"fixture", "v1", "microcredit", 1, 0, 0, 0}, Bound: FeeBound{50, 0}, Reserved: 50}}}}
	return r, request
}

func TestFeeReplyRejectsWrongIdentityRequestTotalsAndProtocol(t *testing.T) {
	original, request := feeSuccess()
	for _, mutate := range []func(*FeeReply){
		func(r *FeeReply) { r.Schema = 2 }, func(r *FeeReply) { r.Action = "snapshot" },
		func(r *FeeReply) { r.Identity.User = "other" }, func(r *FeeReply) { r.EntryID = 2 },
		func(r *FeeReply) { r.AttemptKey = "different" }, func(r *FeeReply) { r.RequestHash = strings.Repeat("b", 64) },
		func(r *FeeReply) { r.Snapshot.Held = 0 }, func(r *FeeReply) { r.Snapshot.Frozen = true },
		func(r *FeeReply) { r.Snapshot.Entries[0].State = "completed" },
		func(r *FeeReply) { r.Snapshot.Entries[0].Price.Input = 2 },
	} {
		data, _ := json.Marshal(original)
		var copy FeeReply
		if err := json.Unmarshal(data, &copy); err != nil {
			t.Fatal(err)
		}
		mutate(&copy)
		data, _ = json.Marshal(copy)
		if _, err := decodeFeeReply(data, 0, original.Identity, request); !errors.Is(err, ErrFeeProtocol) {
			t.Fatal(copy, err)
		}
	}
	data, _ := json.Marshal(original)
	for _, bad := range [][]byte{
		append(append([]byte(nil), data...), []byte(`{}`)...),
		[]byte(strings.TrimSuffix(string(data), "}") + `,"schema":1}`),
		bytes.Repeat([]byte("x"), (64<<10)+1), []byte{0xff},
	} {
		if _, err := decodeFeeReply(bad, 0, original.Identity, request); !errors.Is(err, ErrFeeProtocol) {
			t.Fatal(err)
		}
	}
	if _, err := decodeFeeReply(data, 1, original.Identity, request); !errors.Is(err, ErrFeeProtocol) {
		t.Fatal("exit contradiction", err)
	}
	var output feeOutput
	output.limit = 3
	if _, err := output.Write([]byte("xxxx")); !errors.Is(err, ErrFeeProtocol) || output.Len() != 0 {
		t.Fatal(err)
	}
}

func TestFeeReplyChecksErrorAndSuccessTargetStates(t *testing.T) {
	r, request := feeSuccess()
	r.Error = "budget"
	r.EntryID = 0
	data, _ := json.Marshal(r)
	if _, err := decodeFeeReply(data, 1, r.Identity, request); !errors.Is(err, ErrFeeOperation) {
		t.Fatal(err)
	}
	r.Snapshot = FeeSnapshot{Held: 999}
	data, _ = json.Marshal(r)
	if _, err := decodeFeeReply(data, 1, r.Identity, request); !errors.Is(err, ErrFeeProtocol) {
		t.Fatal("bad error snapshot", err)
	}
	r, _ = feeSuccess()
	r.Action = "begin"
	r.AttemptKey, r.RequestHash = "", ""
	data, _ = json.Marshal(r)
	if _, err := decodeFeeReply(data, 0, r.Identity, FeeRequest{Action: "begin", EntryID: 1}); !errors.Is(err, ErrFeeProtocol) {
		t.Fatal("success but not sent", err)
	}
}
