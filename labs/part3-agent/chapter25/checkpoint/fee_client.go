package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrFeeProtocol = errors.New("checkpoint: invalid fee protocol")
var ErrFeeOperation = errors.New("checkpoint: fee operation rejected")

type FeeIdentity struct{ Tenant, User, Run string }
type FeePrice struct {
	Model, Version, Unit                 string
	Input, CacheRead, CacheWrite, Output int64
}
type FeeUsage struct{ Input, CacheRead, CacheWrite, Output int64 }
type FeeBound struct{ Input, Output int64 }
type FeeEntry struct {
	ID               int
	State            string
	Price            FeePrice
	Bound            FeeBound
	Reserved, Actual int64
	Usage            FeeUsage
}
type FeeSnapshot struct {
	Unit              string
	Limit, Used, Held int64
	Frozen            bool
	Entries           []FeeEntry
}
type FeeReply struct {
	Schema                  int
	Action                  string
	Identity                FeeIdentity
	EntryID                 int
	AttemptKey, RequestHash string
	Snapshot                FeeSnapshot
	Error                   string
}
type FeeRequest struct {
	Action     string
	EntryID    int
	Key, Hash  string
	InputUsage *int64
}
type FeeCLI struct {
	Program, Digest, Directory string
	Run                        Identity
}

func (f FeeCLI) identity() FeeIdentity {
	data, _ := json.Marshal(f.Run)
	h := sha256.Sum256(data)
	return FeeIdentity{f.Run.Tenant, f.Run.User, hex.EncodeToString(h[:])}
}
func feeDigestValid(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == 32 && hex.EncodeToString(data) == value
}
func (r FeeRequest) valid() bool {
	switch r.Action {
	case "prepare", "lookup":
		return strings.TrimSpace(r.Key) != "" && len(r.Key) <= 128 && utf8.ValidString(r.Key) && feeDigestValid(r.Hash) && r.EntryID == 0 && r.InputUsage == nil
	case "snapshot":
		return r.Key == "" && r.Hash == "" && r.EntryID == 0 && r.InputUsage == nil
	case "begin", "unknown":
		return r.Key == "" && r.Hash == "" && r.EntryID > 0 && r.EntryID <= 64 && r.InputUsage == nil
	case "settle":
		return r.Key == "" && r.Hash == "" && r.EntryID > 0 && r.EntryID <= 64 && r.InputUsage != nil && *r.InputUsage >= 0 && *r.InputUsage <= 1_000_000
	}
	return false
}

type feeOutput struct {
	bytes.Buffer
	limit int
}

func (w *feeOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.Len() {
		return 0, ErrFeeProtocol
	}
	return w.Buffer.Write(p)
}

func (f FeeCLI) Invoke(ctx context.Context, request FeeRequest) (FeeReply, error) {
	if !request.valid() || !filepath.IsAbs(f.Program) || !filepath.IsAbs(f.Directory) || !feeDigestValid(f.Digest) {
		return FeeReply{}, ErrFeeProtocol
	}
	for _, value := range []string{f.Run.Tenant, f.Run.User, f.Run.Session, f.Run.Run} {
		if value == "" || len(value) > 128 || !utf8.ValidString(value) {
			return FeeReply{}, ErrFeeProtocol
		}
	}
	if err := ctx.Err(); err != nil {
		return FeeReply{}, err
	}
	program, err := os.Open(f.Program)
	if err != nil {
		return FeeReply{}, ErrFeeProtocol
	}
	info, statErr := program.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 || info.Mode().Perm()&0111 == 0 {
		program.Close()
		return FeeReply{}, ErrFeeProtocol
	}
	h := sha256.New()
	_, readErr := io.Copy(h, io.LimitReader(program, (32<<20)+1))
	closeErr := program.Close()
	if readErr != nil || closeErr != nil || hex.EncodeToString(h.Sum(nil)) != f.Digest {
		return FeeReply{}, ErrFeeProtocol
	}
	id := f.identity()
	args := []string{"-ledger-dir", f.Directory, "-ledger-action", request.Action, "-tenant", id.Tenant, "-user", id.User, "-run", id.Run}
	if request.Action == "prepare" || request.Action == "lookup" {
		args = append(args, "-attempt-key", request.Key, "-request-hash", request.Hash)
	}
	if request.EntryID != 0 {
		args = append(args, "-call-id", strconv.Itoa(request.EntryID))
	}
	if request.InputUsage != nil {
		args = append(args, "-usage-input", strconv.FormatInt(*request.InputUsage, 10))
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(bounded, f.Program, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	stdout, stderr := &feeOutput{limit: 64 << 10}, &feeOutput{limit: 8 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	if err := bounded.Err(); err != nil {
		return FeeReply{}, err
	}
	code := 0
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return FeeReply{}, ErrFeeProtocol
		}
		code = exit.ExitCode()
	}
	return decodeFeeReply(stdout.Bytes(), code, id, request)
}

func validFeeSnapshot(s FeeSnapshot) bool {
	if s.Unit != "microcredit" || s.Limit != 100 || len(s.Entries) > 64 {
		return false
	}
	var used, held int64
	frozen := false
	for n, e := range s.Entries {
		if e.ID != n+1 || e.Price != (FeePrice{"fixture", "v1", "microcredit", 1, 0, 0, 0}) || e.Bound != (FeeBound{50, 0}) || e.Reserved != 50 {
			return false
		}
		switch e.State {
		case "reserved", "sent", "unknown", "not_sent":
			if e.Actual != 0 || e.Usage != (FeeUsage{}) {
				return false
			}
			if e.State != "not_sent" {
				held += 50
			}
		case "settled", "overrun":
			if e.Usage.Input < 0 || e.Usage.Input > 1_000_000 || e.Usage.CacheRead != 0 || e.Usage.CacheWrite != 0 || e.Usage.Output != 0 || e.Actual != e.Usage.Input || (e.State == "overrun") != (e.Usage.Input > 50) {
				return false
			}
			used += e.Actual
			frozen = frozen || e.State == "overrun"
		default:
			return false
		}
	}
	return s.Used == used && s.Held == held && s.Frozen == frozen && (frozen || used+held <= 100)
}

func decodeFeeReply(data []byte, code int, id FeeIdentity, request FeeRequest) (FeeReply, error) {
	if !request.valid() || len(data) > 64<<10 || !utf8.Valid(data) || uniqueStateJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return FeeReply{}, ErrFeeProtocol
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r FeeReply
	if dec.Decode(&r) != nil || dec.Decode(new(any)) != io.EOF || r.Schema != 1 || r.Action != request.Action || r.Identity != id {
		return FeeReply{}, ErrFeeProtocol
	}
	expectedCode := 0
	switch r.Error {
	case "":
	case "busy", "budget", "state", "overrun":
		expectedCode = 1
	case "invalid", "storage", "unconfirmed":
		expectedCode = 2
	default:
		return FeeReply{}, ErrFeeProtocol
	}
	if code != expectedCode {
		return FeeReply{}, ErrFeeProtocol
	}
	if r.EntryID < 0 || r.EntryID > 64 {
		return FeeReply{}, ErrFeeProtocol
	}
	if r.Error != "" {
		if r.Snapshot.Unit == "" {
			if r.Snapshot.Limit != 0 || r.Snapshot.Used != 0 || r.Snapshot.Held != 0 || r.Snapshot.Frozen || len(r.Snapshot.Entries) != 0 {
				return FeeReply{}, ErrFeeProtocol
			}
		} else if !validFeeSnapshot(r.Snapshot) {
			return FeeReply{}, ErrFeeProtocol
		}
		if request.Action == "prepare" || request.Action == "lookup" {
			if r.EntryID != 0 || !((r.AttemptKey == "" && r.RequestHash == "") || (r.AttemptKey == request.Key && r.RequestHash == request.Hash)) {
				return FeeReply{}, ErrFeeProtocol
			}
		} else if (r.EntryID != 0 && r.EntryID != request.EntryID) || r.AttemptKey != "" || r.RequestHash != "" {
			return FeeReply{}, ErrFeeProtocol
		}
		return r, ErrFeeOperation
	}
	if !validFeeSnapshot(r.Snapshot) {
		return FeeReply{}, ErrFeeProtocol
	}
	if request.Action == "snapshot" {
		if r.EntryID != 0 || r.AttemptKey != "" || r.RequestHash != "" {
			return FeeReply{}, ErrFeeProtocol
		}
		return r, nil
	}
	if r.EntryID < 1 || r.EntryID > len(r.Snapshot.Entries) {
		return FeeReply{}, ErrFeeProtocol
	}
	entry := r.Snapshot.Entries[r.EntryID-1]
	if request.Action == "prepare" || request.Action == "lookup" {
		if r.AttemptKey != request.Key || r.RequestHash != request.Hash {
			return FeeReply{}, ErrFeeProtocol
		}
	} else {
		if r.EntryID != request.EntryID || r.AttemptKey != "" || r.RequestHash != "" {
			return FeeReply{}, ErrFeeProtocol
		}
		if request.Action == "begin" && entry.State != "sent" || request.Action == "unknown" && entry.State != "unknown" || request.Action == "settle" && (entry.State != "settled" || entry.Usage.Input != *request.InputUsage) {
			return FeeReply{}, ErrFeeProtocol
		}
	}
	return r, nil
}
