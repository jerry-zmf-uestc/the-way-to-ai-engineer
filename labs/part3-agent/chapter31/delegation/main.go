package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
)

var (
	ErrDenied = errors.New("delegation: capability denied")
	ErrBudget = errors.New("delegation: shared budget exceeded")
	ErrResult = errors.New("delegation: invalid result")
)

type Scope struct{ Tenant, User string }
type Fact struct{ Key, Value, Source string }

type Task struct {
	ID, Parent string
	Scope      Scope
	Goal       string
	Context    []Fact
	Depth      int
}
type Result struct {
	TaskID, Parent, Actor string
	Scope                 Scope
	Facts                 []Fact
	Missing               []string
}

func validFact(f Fact) bool {
	return f.Key != "" && len(f.Key) <= 128 && f.Source != "" && len(f.Source) <= 256 && len(f.Value) <= 4096
}

type Tool func(context.Context, Scope) (Fact, error)
type Ledger struct {
	mu          sync.Mutex
	Limit, Used int
	beforeTake  func(string, int) error
	afterReturn func(int, string, Fact, error) error
}

func (l *Ledger) take(tool string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Used >= l.Limit {
		return 0, ErrBudget
	}
	if l.beforeTake != nil {
		if err := l.beforeTake(tool, l.Used+1); err != nil {
			return 0, err
		}
	}
	l.Used++
	return l.Used, nil
}

func (l *Ledger) returned(sequence int, tool string, fact Fact, cause error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.afterReturn != nil {
		return l.afterReturn(sequence, tool, fact, cause)
	}
	return nil
}

type Toolbox struct {
	scope    Scope
	tools    map[string]Tool
	ledger   *Ledger
	observed []Fact
}

func (t *Toolbox) Read(ctx context.Context, name string) (Fact, error) {
	if err := ctx.Err(); err != nil {
		return Fact{}, err
	}
	tool, ok := t.tools[name]
	if !ok {
		return Fact{}, ErrDenied
	}
	sequence, err := t.ledger.take(name)
	if err != nil {
		return Fact{}, err
	}
	fact, err := tool(ctx, t.scope)
	if commitErr := t.ledger.returned(sequence, name, fact, err); commitErr != nil {
		return Fact{}, errors.Join(err, commitErr)
	}
	if err != nil {
		return Fact{}, err
	}
	if err := ctx.Err(); err != nil {
		return Fact{}, err
	}
	if !validFact(fact) {
		return Fact{}, ErrResult
	}
	t.observed = append(t.observed, fact)
	return fact, nil
}

type Worker func(context.Context, Task, *Toolbox) (Result, error)
type Role struct {
	Name  string
	Tools []string
	Run   Worker
}
type Dispatcher struct {
	Roles       map[string]Role
	ParentTools map[string]Tool
	Ledger      *Ledger
	MaxDepth    int
}

// Delegate给子任务选定上下文与能力，不传递父级完整历史或可变政策map。
func (d Dispatcher) Delegate(ctx context.Context, task Task, roleName string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	role, ok := d.Roles[roleName]
	if !ok || role.Name != roleName || role.Run == nil || task.ID == "" || task.Parent == "" || task.Scope.Tenant == "" || task.Scope.User == "" || task.Depth < 0 || task.Depth >= d.MaxDepth || len(task.Context) > 8 || len(task.Goal) > 256 || d.Ledger == nil {
		return Result{}, ErrDenied
	}
	for _, fact := range task.Context {
		if !validFact(fact) {
			return Result{}, ErrResult
		}
	}
	child := task
	child.Depth++
	child.Context = append([]Fact(nil), task.Context...)
	box := &Toolbox{scope: task.Scope, tools: map[string]Tool{}, ledger: d.Ledger}
	for _, name := range role.Tools {
		if tool, approved := d.ParentTools[name]; approved {
			box.tools[name] = tool
		}
	}
	output, err := role.Run(ctx, child, box)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if output.TaskID != task.ID || output.Parent != task.Parent || output.Actor != role.Name || output.Scope != task.Scope || len(output.Facts) > 8 || len(output.Missing) > 8 {
		return Result{}, ErrResult
	}
	for _, missing := range output.Missing {
		if missing == "" || len(missing) > 256 {
			return Result{}, ErrResult
		}
	}
	// 只接受本次受控工具或父级显式选入的事实，不接受子Agent自报来源。
	allowed := append(append([]Fact(nil), task.Context...), box.observed...)
	for _, fact := range output.Facts {
		found := false
		for _, known := range allowed {
			if fact == known {
				found = true
			}
		}
		if !found {
			return Result{}, ErrResult
		}
	}
	output.Facts = append([]Fact(nil), output.Facts...)
	output.Missing = append([]string(nil), output.Missing...)
	return output, nil
}

// Aggregate consumes Dispatcher-validated results; parent comes from the trusted caller.
func Aggregate(scope Scope, parent string, expected map[string]string, results []Result) (map[string]Fact, error) {
	if err := validateResultSet(scope, parent, expected, results); err != nil {
		return nil, err
	}
	facts := map[string]Fact{}
	for _, result := range results {
		for _, fact := range result.Facts {
			if old, ok := facts[fact.Key]; ok && old.Value != fact.Value {
				return nil, ErrResult
			}
			facts[fact.Key] = fact
		}
	}
	return facts, nil
}
func roles() map[string]Role {
	out := map[string]Role{}
	for _, key := range []string{"timeout", "retry"} {
		name := key + "-reader"
		toolName := "read-" + key
		out[name] = Role{Name: name, Tools: []string{toolName}, Run: func(ctx context.Context, task Task, box *Toolbox) (Result, error) {
			fact, err := box.Read(ctx, toolName)
			if err != nil {
				return Result{}, err
			}
			return Result{TaskID: task.ID, Parent: task.Parent, Actor: name, Scope: task.Scope, Facts: []Fact{fact}}, nil
		}}
	}
	return out
}
func tools() map[string]Tool {
	return map[string]Tool{"read-timeout": func(context.Context, Scope) (Fact, error) { return Fact{"timeout", "30", "guide@v1:timeout"}, nil }, "read-retry": func(context.Context, Scope) (Fact, error) { return Fact{"retry", "2", "guide@v1:retry"}, nil }}
}
func main() {
	showHandoff := flag.Bool("handoff", false, "run control ownership transfer")
	storeDir := flag.String("store", "", "private absolute handoff state directory")
	storeOp := flag.String("store-op", "show", "init/show/execute/transfer/complete/recover-readonly")
	storeGeneration := flag.Int("store-generation", 0, "expected file-store recovery generation")
	confirmStopped := flag.Bool("store-confirm-stopped", false, "application confirmed previous worker stopped")
	storeOwner := flag.String("store-owner", "", "expected current demo role")
	storeRevision := flag.Uint64("store-revision", 0, "expected current demo revision")
	storeTo := flag.String("store-to", "", "approved destination role")
	flag.Parse()
	storeRequested := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "store", "store-op", "store-owner", "store-revision", "store-to", "store-generation", "store-confirm-stopped":
			storeRequested = true
		}
	})
	if storeRequested && *storeDir == "" {
		panic(ErrHandoff)
	}
	if *storeDir != "" {
		if *showHandoff || flag.NArg() != 0 {
			panic(ErrHandoff)
		}
		store, err := OpenHandoffStore(*storeDir, readonlyHandoffFactory, "readonly-demo-v1")
		if err != nil {
			panic(err)
		}
		defer store.Close()
		request := HandoffRequest{ID: "report", Scope: Scope{"demo", "reader"}, From: *storeOwner, To: *storeTo, Revision: *storeRevision}
		var record HandoffFileRecord
		if *storeOp == "recover-readonly" {
			record, err = store.RecoverReadonly(context.Background(), request, *storeGeneration, HandoffRecoveryApproval{Version: "readonly-demo-v1", Actor: request.From, Readonly: true, PreviousStopped: *confirmStopped})
		} else {
			if *confirmStopped {
				panic(ErrHandoff)
			}
			record, err = store.ApplyVersioned(context.Background(), *storeOp, request, *storeGeneration)
		}
		if err != nil {
			panic(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(record); err != nil {
			panic(err)
		}
		return
	}
	if *showHandoff {
		state, err := handoffDemo()
		if err != nil {
			panic(err)
		}
		fmt.Printf("run=%s owner=%s status=%s hops=%d turns=%d facts=%d revision=%d\n", state.ID, state.Owner, state.Status, state.Hops, state.Turns, len(state.Facts), state.Revision)
		return
	}
	ctx := context.Background()
	scope := Scope{"demo", "reader"}
	catalog := tools()
	single := []Fact{}
	for _, name := range []string{"read-timeout", "read-retry"} {
		fact, err := catalog[name](ctx, scope)
		if err != nil {
			panic(err)
		}
		single = append(single, fact)
	}
	ledger := &Ledger{Limit: 2}
	d := Dispatcher{Roles: roles(), ParentTools: catalog, Ledger: ledger, MaxDepth: 2}
	expected := map[string]string{"timeout": "timeout-reader", "retry": "retry-reader"}
	results := []Result{}
	for _, key := range []string{"timeout", "retry"} {
		result, err := d.Delegate(ctx, Task{ID: key, Parent: "parent-run", Scope: scope, Goal: key}, expected[key])
		if err != nil {
			panic(err)
		}
		results = append(results, result)
	}
	combined, err := Aggregate(scope, "parent-run", expected, results)
	if err != nil {
		panic(err)
	}
	equal := len(combined) == len(single)
	for _, fact := range single {
		equal = equal && combined[fact.Key] == fact
	}
	fmt.Printf("equivalent=%t delegated_roles=%d shared_tool_calls=%d facts=%d\n", equal, len(results), ledger.Used, len(combined))
}
