package tasks

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWorkerChildContextCategoriesAndCapacity(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			m, err := NewManager(1, time.Second, func(context.Context, Task) (string, error) {
				return "late-private-payload", fmt.Errorf("private-wrapper: %w", cause)
			})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			scope := Scope{Tenant: "t", User: "u"}
			task, err := m.Start(scope, "k", "read")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			final, err := m.Wait(ctx, scope, task.ID)
			state, code := "canceled", "worker_canceled"
			if cause == context.DeadlineExceeded {
				state, code = "failed", "worker_deadline_exceeded"
			}
			if err != nil || final.State != state || final.Failure != code || final.Output != "" || final.Attempt != 1 || strings.Contains(final.Failure, "private-wrapper") {
				t.Fatal(final, err)
			}
			if _, err = m.Start(scope, "next", "read"); err != nil {
				t.Fatal("slot not released after actual exit", err)
			}
		})
	}
}
