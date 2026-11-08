package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrBudget   = errors.New("react: budget exceeded")
	ErrProtocol = errors.New("react: invalid decision")
	ErrStalled  = errors.New("react: repeated action without progress")
)

type Task struct{ ID, Query, Fallback, Want string }
type Action struct{ Tool, Query string }
type Observation struct {
	Action                  Action
	Source, Text, ErrorCode string
}
type Decision struct {
	Action                *Action
	Final, Source, Reason string
	Refuse                bool
}
type Policy func(context.Context, Task, []Observation) (Decision, error)
type Executor func(context.Context, Action) (Observation, error)
type Limits struct{ Rounds, Tools int }
type Report struct {
	Final, Source string
	Refused       bool
	Rounds, Tools int
	History       []Observation
	Decisions     []Decision
}

func Run(ctx context.Context, task Task, limits Limits, next Policy, execute Executor) (Report, error) {
	var report Report
	if task.ID == "" || task.Query == "" || len(task.Query) > 256 || len(task.Fallback) > 256 || limits.Rounds <= 0 || limits.Rounds > 32 || limits.Tools < 0 || limits.Tools > 32 || next == nil || execute == nil {
		return report, ErrProtocol
	}
	input := task
	input.Want = "" // 评分标签不能进入决策输入。
	repeats := map[Action]int{}
	for report.Rounds < limits.Rounds {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		report.Rounds++
		decision, err := next(ctx, input, append([]Observation(nil), report.History...))
		if err != nil {
			return report, fmt.Errorf("react: decision: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if len(decision.Final) > 4096 || len(decision.Source) > 256 || len(decision.Reason) > 128 {
			return report, ErrProtocol
		}
		// 分支互斥。决策记录复制Action，调用方不能修改已保存提议。
		saved := decision
		if decision.Action != nil {
			action := *decision.Action
			saved.Action = &action
		}
		report.Decisions = append(report.Decisions, saved)
		if decision.Action == nil {
			if decision.Refuse {
				if decision.Final != "" || decision.Source != "" {
					return report, ErrProtocol
				}
				report.Refused = true
				return report, nil
			}
			if strings.TrimSpace(decision.Final) == "" {
				return report, ErrProtocol
			}
			found := false
			for _, observation := range report.History {
				if observation.ErrorCode == "" && observation.Source != "" && observation.Source == decision.Source && observation.Text == decision.Final {
					found = true
				}
			}
			if !found {
				return report, ErrProtocol
			}
			report.Final = decision.Final
			report.Source = decision.Source
			return report, nil
		}
		if decision.Final != "" || decision.Source != "" || decision.Refuse {
			return report, ErrProtocol
		}
		action := *decision.Action
		if action.Tool != "search" || strings.TrimSpace(action.Query) == "" || len(action.Query) > 256 {
			return report, ErrProtocol
		}
		if report.Tools >= limits.Tools {
			return report, ErrBudget
		}
		if repeats[action] >= 2 {
			return report, ErrStalled
		}
		repeats[action]++
		report.Tools++
		observation, err := execute(ctx, action)
		if stop := ctx.Err(); stop != nil {
			return report, stop
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return report, err
		}
		observation.Action = action
		if err != nil {
			observation.Source = ""
			observation.Text = ""
			observation.ErrorCode = "tool_failed"
		}
		if len(observation.Text) > 4096 || len(observation.Source) > 256 || len(observation.ErrorCode) > 128 {
			return report, ErrProtocol
		}
		if observation.ErrorCode != "" {
			observation.Source = ""
			observation.Text = ""
		}
		report.History = append(report.History, observation)
	}
	return report, ErrBudget
}
func Choose(adaptive bool) Policy {
	return func(ctx context.Context, task Task, history []Observation) (Decision, error) {
		if err := ctx.Err(); err != nil {
			return Decision{}, err
		}
		if len(history) == 0 {
			return Decision{Action: &Action{Tool: "search", Query: task.Query}, Reason: "initial_query"}, nil
		}
		last := history[len(history)-1]
		if last.ErrorCode == "" && last.Source != "" && last.Text != "" {
			return Decision{Final: last.Text, Source: last.Source, Reason: "evidence_received"}, nil
		}
		if adaptive && len(history) == 1 && task.Fallback != "" {
			return Decision{Action: &Action{Tool: "search", Query: task.Fallback}, Reason: "need_other_query"}, nil
		}
		return Decision{Refuse: true, Reason: "insufficient_evidence"}, nil
	}
}
func search(ctx context.Context, action Action) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	if action.Query == "timeout" {
		return Observation{Source: "guide@v1:timeout", Text: "Timeout is 30 seconds."}, nil
	}
	return Observation{}, nil
}
func Complete(task Task, report Report, err error) bool {
	if err != nil {
		return false
	}
	if task.Want == "" {
		return report.Refused
	}
	return !report.Refused && report.Final == task.Want
}
func main() {
	tasks := []Task{{ID: "direct", Query: "timeout", Want: "Timeout is 30 seconds."}, {ID: "alias", Query: "E42", Fallback: "timeout", Want: "Timeout is 30 seconds."}, {ID: "missing", Query: "unknown"}}
	for _, adaptive := range []bool{false, true} {
		passed, rounds, tools := 0, 0, 0
		for _, task := range tasks {
			report, err := Run(context.Background(), task, Limits{Rounds: 4, Tools: 3}, Choose(adaptive), search)
			if Complete(task, report, err) {
				passed++
			}
			rounds += report.Rounds
			tools += report.Tools
		}
		fmt.Printf("adaptive=%t passed=%d/%d rounds=%d tools=%d\n", adaptive, passed, len(tasks), rounds, tools)
	}
}
