package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrPlan     = errors.New("planning: invalid plan")
	ErrRevision = errors.New("planning: invalid revision")
	ErrFailed   = errors.New("planning: step failed")
	ErrBudget   = errors.New("planning: budget exceeded")
)

type Node struct {
	ID, Goal, Query string
	Depends         []string
}
type Plan struct {
	Version uint64
	Nodes   []Node
}
type Fact struct{ Key, Value, Source string }
type Progress struct {
	State, ErrorCode string
	Fact             Fact
}
type Replanner func(context.Context, Plan, map[string]Progress) (Plan, error)
type Executor func(context.Context, Node) (Fact, error)
type Limits struct{ Attempts, Replans int }
type Report struct {
	Plan              Plan
	Progress          map[string]Progress
	Attempts, Replans int
}

func clonePlan(p Plan) Plan {
	p.Nodes = append([]Node(nil), p.Nodes...)
	for n, node := range p.Nodes {
		p.Nodes[n].Depends = append([]string(nil), node.Depends...)
	}
	return p
}
func cloneProgress(m map[string]Progress) map[string]Progress {
	out := map[string]Progress{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
func Validate(p Plan, goals map[string]string) error {
	if p.Version == 0 || len(p.Nodes) == 0 || len(p.Nodes) > 16 {
		return ErrPlan
	}
	nodes := map[string]Node{}
	for _, node := range p.Nodes {
		if node.ID == "" || len(node.ID) > 128 || node.Query == "" || len(node.Query) > 256 || len(node.Depends) > 16 {
			return ErrPlan
		}
		if _, ok := goals[node.Goal]; !ok {
			return ErrPlan
		}
		if _, ok := nodes[node.ID]; ok {
			return ErrPlan
		}
		nodes[node.ID] = node
	}
	colors := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if colors[id] == 1 {
			return ErrPlan
		}
		if colors[id] == 2 {
			return nil
		}
		node, ok := nodes[id]
		if !ok {
			return ErrPlan
		}
		colors[id] = 1
		seen := map[string]bool{}
		for _, dep := range node.Depends {
			if seen[dep] {
				return ErrPlan
			}
			seen[dep] = true
			if err := visit(dep); err != nil {
				return err
			}
		}
		colors[id] = 2
		return nil
	}
	for id := range nodes {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

// Revise保留完成项的定义与结果；整个候选通过后才切换计划。
func Revise(old, next Plan, progress map[string]Progress, goals map[string]string) (map[string]Progress, error) {
	if old.Version == ^uint64(0) || next.Version != old.Version+1 {
		return nil, ErrRevision
	}
	if err := Validate(next, goals); err != nil {
		return nil, err
	}
	previous := map[string]Node{}
	candidate := map[string]Node{}
	for _, node := range old.Nodes {
		previous[node.ID] = node
	}
	for _, node := range next.Nodes {
		candidate[node.ID] = node
	}
	out := map[string]Progress{}
	for id, state := range progress {
		if state.State == "completed" {
			node, exists := candidate[id]
			if !exists || !reflect.DeepEqual(previous[id], node) {
				return nil, ErrRevision
			}
			out[id] = state
		}
	}
	for _, node := range next.Nodes {
		if _, ok := out[node.ID]; !ok {
			out[node.ID] = Progress{State: "pending"}
		}
	}
	return out, nil
}
func Ready(plan Plan, progress map[string]Progress) (Node, bool) {
	for _, node := range plan.Nodes {
		if progress[node.ID].State != "pending" {
			continue
		}
		ready := true
		for _, dep := range node.Depends {
			if progress[dep].State != "completed" {
				ready = false
			}
		}
		if ready {
			return node, true
		}
	}
	return Node{}, false
}
func Run(ctx context.Context, plan Plan, goals map[string]string, limits Limits, execute Executor, replan Replanner) (Report, error) {
	report := Report{Plan: clonePlan(plan), Progress: map[string]Progress{}}
	if limits.Attempts <= 0 || limits.Attempts > 32 || limits.Replans < 0 || limits.Replans > 8 || execute == nil {
		return report, ErrPlan
	}
	if err := Validate(plan, goals); err != nil {
		return report, err
	}
	for _, node := range report.Plan.Nodes {
		report.Progress[node.ID] = Progress{State: "pending"}
	}
	for {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		node, ok := Ready(report.Plan, report.Progress)
		if !ok {
			for _, state := range report.Progress {
				if state.State != "completed" {
					return report, ErrFailed
				}
			}
			return report, nil
		}
		if report.Attempts >= limits.Attempts {
			return report, ErrBudget
		}
		report.Progress[node.ID] = Progress{State: "running"}
		report.Attempts++
		node.Depends = append([]string(nil), node.Depends...)
		fact, err := execute(ctx, node)
		if stop := ctx.Err(); stop != nil {
			return report, stop
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return report, err
		}
		if len(fact.Key) > 128 || len(fact.Value) > 4096 || len(fact.Source) > 256 {
			return report, ErrPlan
		}
		if err == nil && fact.Key == node.Goal && fact.Source != "" {
			report.Progress[node.ID] = Progress{State: "completed", Fact: fact}
			continue
		}
		report.Progress[node.ID] = Progress{State: "failed", ErrorCode: "missing_or_failed_evidence"}
		if replan == nil || report.Replans >= limits.Replans {
			if err != nil {
				return report, fmt.Errorf("%w: %w", ErrFailed, err)
			}
			return report, ErrFailed
		}
		report.Replans++
		next, err := replan(ctx, clonePlan(report.Plan), cloneProgress(report.Progress))
		if err != nil {
			return report, fmt.Errorf("planning: replanner: %w", err)
		}
		if stop := ctx.Err(); stop != nil {
			return report, stop
		}
		state, err := Revise(report.Plan, next, report.Progress, goals)
		if err != nil {
			return report, err
		}
		report.Plan = clonePlan(next)
		report.Progress = state
	}
}

// 计划完成与任务完成分开；这里仅评分固定结构化事实，不判断任意自然语言目标。
func Complete(goals map[string]string, r Report, err error) bool {
	if err != nil {
		return false
	}
	facts := map[string]string{}
	for _, state := range r.Progress {
		if state.State != "completed" || state.Fact.Source == "" {
			return false
		}
		if old, ok := facts[state.Fact.Key]; ok && old != state.Fact.Value {
			return false
		}
		facts[state.Fact.Key] = state.Fact.Value
	}
	for key, want := range goals {
		if got, ok := facts[key]; !ok || got != want {
			return false
		}
	}
	return true
}
func read(ctx context.Context, node Node) (Fact, error) {
	if err := ctx.Err(); err != nil {
		return Fact{}, err
	}
	switch node.Query {
	case "retry":
		return Fact{Key: "retry", Value: "2", Source: "guide@v1:retry"}, nil
	case "timeout":
		return Fact{Key: "timeout", Value: "30", Source: "guide@v1:timeout"}, nil
	}
	return Fact{}, nil
}
func repair(ctx context.Context, p Plan, progress map[string]Progress) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	p.Version++
	for n, node := range p.Nodes {
		if progress[node.ID].State == "failed" && node.Goal == "timeout" {
			p.Nodes[n].Query = "timeout"
		}
	}
	return p, nil
}
func main() {
	plan := Plan{Version: 1, Nodes: []Node{{ID: "retry", Goal: "retry", Query: "retry"}, {ID: "timeout", Goal: "timeout", Query: "E42", Depends: []string{"retry"}}}}
	goals := map[string]string{"retry": "2", "timeout": "30"}
	for _, enable := range []bool{false, true} {
		var replanner Replanner
		if enable {
			replanner = repair
		}
		r, err := Run(context.Background(), plan, goals, Limits{Attempts: 5, Replans: 1}, read, replanner)
		fmt.Printf("replan=%t complete=%t attempts=%d replans=%d version=%d\n", enable, Complete(goals, r, err), r.Attempts, r.Replans, r.Plan.Version)
	}
}
