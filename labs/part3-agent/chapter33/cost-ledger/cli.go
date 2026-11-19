package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"unicode/utf8"
)

type LedgerCLIResult struct {
	Schema      int
	Action      string
	Identity    RunIdentity
	EntryID     int
	AttemptKey  string
	RequestHash string
	Snapshot    Snapshot
	Error       string
}

var errLedgerUnconfirmed = errors.New("unconfirmed_ledger_state")

func ledgerCLIError(err error) (string, int) {
	switch {
	case errors.Is(err, errLedgerUnconfirmed):
		return "unconfirmed", 2
	case err == nil:
		return "", 0
	case errors.Is(err, ErrBusy):
		return "busy", 1
	case errors.Is(err, ErrOverrun):
		return "overrun", 1
	case errors.Is(err, ErrBudget):
		return "budget", 1
	case errors.Is(err, ErrInvalid):
		return "invalid", 2
	case errors.Is(err, ErrState):
		return "state", 1
	default:
		return "storage", 2
	}
}

// RunLedgerCLI uses a fixed synthetic price; application code owns every flag.
func RunLedgerCLI(args []string, output io.Writer) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	result := LedgerCLIResult{Schema: 1}
	emit := func(err error) (bool, int) {
		var code int
		result.Error, code = ledgerCLIError(err)
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil || len(data)+1 > 64<<10 || output == nil {
			return true, 2
		}
		data = append(data, '\n')
		if n, writeErr := output.Write(data); writeErr != nil || n != len(data) {
			return true, 2
		}
		return true, code
	}
	fs := flag.NewFlagSet("cost-ledger", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	directory := fs.String("ledger-dir", "", "existing private directory")
	action := fs.String("ledger-action", "snapshot", "snapshot, prepare, lookup, begin, unknown, settle")
	tenant := fs.String("tenant", "demo", "application-bound tenant")
	user := fs.String("user", "reader", "application-bound user")
	run := fs.String("run", "run", "application-bound run")
	key := fs.String("attempt-key", "", "persisted physical attempt")
	requestHash := fs.String("request-hash", "", "approved request SHA256")
	id := fs.Int("call-id", 0, "local entry ID")
	usage := fs.Int64("usage-input", 0, "synthetic known input tokens")
	if fs.Parse(args) != nil || fs.NArg() != 0 || *directory == "" {
		return emit(ErrInvalid)
	}
	for _, value := range []string{*tenant, *user, *run} {
		if value == "" || len(value) > 128 || !utf8.ValidString(value) {
			return emit(ErrInvalid)
		}
	}
	result.Identity = RunIdentity{*tenant, *user, *run}
	supplied := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { supplied[f.Name] = true })
	keyAction := *action == "prepare" || *action == "lookup"
	if (!keyAction && (supplied["attempt-key"] || supplied["request-hash"])) || ((keyAction || *action == "snapshot") && supplied["call-id"]) || (*action != "settle" && supplied["usage-input"]) || (*action == "settle" && !supplied["usage-input"]) {
		return emit(ErrInvalid)
	}
	if keyAction {
		if !validAttempt(*key, *requestHash) || *id != 0 || *usage != 0 {
			return emit(ErrInvalid)
		}
	} else if *key != "" || *requestHash != "" {
		return emit(ErrInvalid)
	}
	switch *action {
	case "snapshot":
		if *id != 0 || *usage != 0 {
			return emit(ErrInvalid)
		}
	case "prepare", "lookup":
	case "begin", "unknown", "settle":
		if *id < 1 || *id > 64 || *usage < 0 || *usage > 1_000_000 || (*action != "settle" && *usage != 0) {
			return emit(ErrInvalid)
		}
	default:
		return emit(ErrInvalid)
	}
	result.Action = *action
	r, err := os.OpenRoot(*directory)
	if err != nil {
		return emit(err)
	}
	defer r.Close()
	d, err := OpenDurableLedger(r, result.Identity, "microcredit", 100)
	if err != nil {
		return emit(err)
	}
	result.EntryID = *id
	switch *action {
	case "prepare":
		result.AttemptKey, result.RequestHash = *key, *requestHash
		result.EntryID, err = d.ReserveAttempt(*key, *requestHash, Price{Model: "fixture", Version: "v1", Unit: "microcredit", Input: 1}, Bound{Input: 50})
	case "lookup":
		result.AttemptKey, result.RequestHash = *key, *requestHash
		result.EntryID, err = d.LookupAttempt(*key, *requestHash, Price{Model: "fixture", Version: "v1", Unit: "microcredit", Input: 1}, Bound{Input: 50})
	case "begin":
		err = d.Begin(context.Background(), *id)
	case "unknown":
		err = d.Unknown(*id)
	case "settle":
		err = d.Settle(*id, Usage{Input: *usage})
	}
	var snapshotErr error
	result.Snapshot, snapshotErr = d.Snapshot()
	if closeErr := d.Close(); closeErr != nil {
		return emit(errors.Join(errLedgerUnconfirmed, closeErr))
	}
	if snapshotErr != nil {
		result.Snapshot = Snapshot{}
		return emit(errors.Join(errLedgerUnconfirmed, err, snapshotErr))
	}
	return emit(err)
}
