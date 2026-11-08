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
	"unicode/utf8"
)

var (
	ErrState   = errors.New("checkpoint: invalid state")
	ErrUnknown = errors.New("checkpoint: action outcome unknown")
	ErrBusy    = errors.New("checkpoint: writer already claimed")
	ErrBudget  = errors.New("checkpoint: attempts exhausted")
)

type Identity struct{ Tenant, User, Session, Run string }
type Result struct{ Action, Text string }
type State struct {
	Schema           int
	Identity         Identity
	PlanHash, Phase  string
	Cursor, Attempts int
	MaxAttempts      int
	Results          []Result
}
type Store struct {
	Root         *os.Root
	beforeRename func() // Trusted fault injection only; nil during normal operation.
}

func fingerprint(actions []string) string {
	data, _ := json.Marshal(actions)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func validate(s State, id Identity, actions []string) error {
	if s.Schema != 2 || s.Identity != id || s.PlanHash != fingerprint(actions) || s.Cursor < 0 || s.Cursor > len(actions) || len(s.Results) != s.Cursor || s.Attempts < s.Cursor || s.MaxAttempts < 1 || s.MaxAttempts > 32 || s.Attempts > s.MaxAttempts {
		return ErrState
	}
	if s.Phase != "pending" && s.Phase != "in_flight" && s.Phase != "completed" {
		return ErrState
	}
	if (s.Phase == "completed") != (s.Cursor == len(actions)) || s.Phase == "in_flight" && s.Cursor == len(actions) {
		return ErrState
	}
	for n, result := range s.Results {
		if result.Action != actions[n] || len(result.Text) > 4096 {
			return ErrState
		}
	}
	return nil
}
func (s Store) Load() (State, error) {
	f, err := s.Root.Open("checkpoint.json")
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil {
		return State{}, err
	}
	if len(data) > 64<<10 || !utf8.Valid(data) || uniqueStateJSON(json.NewDecoder(bytes.NewReader(data)), 0) != nil {
		return State{}, ErrState
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var state State
	if err := dec.Decode(&state); err != nil {
		return State{}, fmt.Errorf("%w: %w", ErrState, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return State{}, ErrState
	}
	return state, nil
}

// Save假定当前进程持有writer.lock。临时文件、结果和游标一起替换。
func (s Store) Save(state State) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.saveBytes("checkpoint.json", "checkpoint.tmp", data)
}

func (s Store) saveBytes(target, candidate string, data []byte) error {
	if len(data) > 64<<10 {
		return ErrState
	}
	// 固定文件名位于私有单写者目录，不接受模型提供的路径。
	f, err := s.Root.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer s.Root.Remove(candidate)
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
	if s.beforeRename != nil {
		s.beforeRename()
	}
	if err = s.Root.Rename(candidate, target); err != nil {
		return err
	}
	dir, err := s.Root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s Store) claim() (func() error, error) {
	f, err := s.Root.OpenFile("writer.lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrBusy
		}
		return nil, err
	}
	if _, err = fmt.Fprintf(f, "pid=%d\n", os.Getpid()); err != nil {
		f.Close()
		s.Root.Remove("writer.lock")
		return nil, err
	}
	if err = f.Close(); err != nil {
		s.Root.Remove("writer.lock")
		return nil, err
	}
	return func() error { return s.Root.Remove("writer.lock") }, nil
}

// Run只允许可信应用声明readonlyReplay；持久尝试计数不因重启清零。
func Run(ctx context.Context, store Store, id Identity, actions []string, maxAttempts int, readonlyReplay bool, read func(context.Context, string) (string, error), hook func(string)) (state State, err error) {
	if id.Tenant == "" || id.User == "" || id.Session == "" || id.Run == "" || len(actions) == 0 || len(actions) > 16 || maxAttempts <= 0 || maxAttempts > 32 || read == nil {
		return State{}, ErrState
	}
	seen := map[string]bool{}
	for _, action := range actions {
		if action == "" || len(action) > 128 || seen[action] {
			return State{}, ErrState
		}
		seen[action] = true
	}
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	release, err := store.claim()
	if err != nil {
		return State{}, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			err = errors.Join(err, releaseErr)
		}
	}()
	state, err = store.Load()
	if errors.Is(err, os.ErrNotExist) {
		state = State{Schema: 2, Identity: id, PlanHash: fingerprint(actions), Phase: "pending", MaxAttempts: maxAttempts}
		err = store.Save(state)
	}
	if err != nil {
		return state, err
	}
	if err = validate(state, id, actions); err != nil {
		return state, err
	}
	if state.MaxAttempts != maxAttempts {
		return state, ErrState
	}
	if state.Phase == "in_flight" && !readonlyReplay {
		return state, ErrUnknown
	}
	for state.Cursor < len(actions) {
		if err = ctx.Err(); err != nil {
			return state, err
		}
		if state.Attempts >= state.MaxAttempts {
			return state, ErrBudget
		}
		state.Phase = "in_flight"
		state.Attempts++
		if err = store.Save(state); err != nil {
			return state, err
		}
		if hook != nil {
			hook("after_inflight")
		}
		action := actions[state.Cursor]
		text, callErr := read(ctx, action)
		if hook != nil {
			hook("after_execute")
		}
		if stop := ctx.Err(); stop != nil {
			return state, stop
		}
		if callErr != nil {
			return state, callErr
		}
		if len(text) > 4096 {
			return state, ErrState
		}
		state.Results = append(state.Results, Result{Action: action, Text: text})
		state.Cursor++
		state.Phase = "pending"
		if state.Cursor == len(actions) {
			state.Phase = "completed"
		}
		if err = store.Save(state); err != nil {
			return state, err
		}
		if hook != nil {
			hook("after_commit")
		}
	}
	return state, nil
}
func main() {
	directory := flag.String("dir", "", "private state directory; default temporary")
	flag.Parse()
	if *directory == "" {
		dir, err := os.MkdirTemp("", "checkpoint-demo-")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(dir)
		*directory = dir
	}
	root, err := os.OpenRoot(*directory)
	if err != nil {
		panic(err)
	}
	defer root.Close()
	state, err := Run(context.Background(), Store{Root: root}, Identity{"demo", "reader", "session", "run"}, []string{"read-A", "read-B"}, 8, true, func(_ context.Context, action string) (string, error) { return "fixed:" + action, nil }, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("phase=%s cursor=%d results=%d attempts=%d\n", state.Phase, state.Cursor, len(state.Results), state.Attempts)
}
