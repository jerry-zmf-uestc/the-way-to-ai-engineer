package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

func TestAmbiguousBodyRejectedBeforeAdmission(t *testing.T) {
	var calls atomic.Int32
	s := testService(t, func(context.Context, tasks.Task) (string, error) { calls.Add(1); return "ok", nil })
	for _, body := range []string{
		`{"input":"first","input":"second"}`,
		`{"input":"first","Input":"second"}`,
		`{"input":"first","\u0069nput":"second"}`,
		"{\"input\":\"" + string([]byte{0xff}) + "\"}",
		strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18),
	} {
		if r := call(s, "POST", "/tasks", "demo-alice", "same-key", body); r.Code != http.StatusBadRequest {
			t.Fatal("ambiguous accepted", r.Code, r.Body.String())
		}
	}
	if len(s.manager.List(tasks.Scope{Tenant: "demo", User: "alice"})) != 0 || calls.Load() != 0 {
		t.Fatal("invalid body admitted a task")
	}
	if r := call(s, "POST", "/tasks", "demo-alice", "same-key", `{"input":"valid"}`); r.Code != 202 {
		t.Fatal("bad body reserved idempotency key", r.Code)
	}
}
func TestUniqueNamesAreScopedToEachObject(t *testing.T) {
	raw := `{"left":{"value":1},"right":{"value":2}}`
	if err := uniqueObjectFields(json.NewDecoder(strings.NewReader(raw)), 0); err != nil {
		t.Fatal("different objects conflict", err)
	}
}
