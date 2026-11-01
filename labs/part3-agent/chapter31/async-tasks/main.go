package main

import (
	"context"
	"fmt"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
	"time"
)

func main() {
	m, _ := tasks.NewManager(2, time.Second, func(ctx context.Context, t tasks.Task) (string, error) { return "evidence: " + t.Input, nil })
	defer m.Close()
	scope := tasks.Scope{Tenant: "demo", User: "alice"}
	task, _ := m.Start(scope, "request-1", "timeout")
	goal := ParentGoal{ID: "report", Scope: scope, Revision: 1}
	binding, err := BindChild(goal, task)
	if err != nil {
		panic(err)
	}
	result, err := m.Wait(context.Background(), scope, task.ID)
	if err != nil {
		panic(err)
	}
	output, err := AdmitChild(goal, binding, result)
	if err != nil {
		panic(err)
	}
	fmt.Printf("task=%s state=%s revision=%d output=%q\n", result.ID, result.State, result.Revision, output)
}
