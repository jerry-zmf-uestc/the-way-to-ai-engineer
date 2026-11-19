package main

import (
	"context"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
	"testing"
	"time"
)

func TestIndependentTaskRoundTrip(t *testing.T) {
	m, err := tasks.NewManager(1, time.Second, func(ctx context.Context, t tasks.Task) (string, error) { return "evidence: " + t.Input, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	scope := tasks.Scope{Tenant: "demo", User: "alice"}
	task, err := m.Start(scope, "request-1", "timeout")
	if err != nil {
		t.Fatal(err)
	}
	result, err := m.Wait(context.Background(), scope, task.ID)
	if err != nil || result.State != "completed" || result.Output != "evidence: timeout" {
		t.Fatal(result, err)
	}
}
