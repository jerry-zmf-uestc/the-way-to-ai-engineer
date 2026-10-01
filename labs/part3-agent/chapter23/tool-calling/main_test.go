package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	fixed := func() time.Time { return time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC) }
	for _, tool := range []Tool{calculatorTool(), getTimeTool(fixed)} {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Register(%q) error = %v", tool.Name, err)
		}
	}
	return registry
}

func TestCallRegisteredTools(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)

	product, err := registry.Call(context.Background(), "calculator", map[string]any{"op": "multiply", "a": 6, "b": 7})
	if err != nil {
		t.Fatalf("Call(calculator) error = %v", err)
	}
	if product != "42" {
		t.Fatalf("Call(calculator) = %q, want %q", product, "42")
	}

	clock, err := registry.Call(context.Background(), "get_time", nil)
	if err != nil {
		t.Fatalf("Call(get_time) error = %v", err)
	}
	if clock != "2026-01-01T08:00:00Z" {
		t.Fatalf("Call(get_time) = %q, want fixed time", clock)
	}
}

func TestUnknownToolRejected(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	_, err := registry.Call(context.Background(), "delete_database", nil)
	if !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("Call(unknown) error = %v, want ErrUnknownTool", err)
	}
	if !strings.Contains(err.Error(), `"delete_database"`) {
		t.Fatalf("Call(unknown) error = %q, want tool name in message", err.Error())
	}
}

func TestInvalidArgumentsRejectedWithoutRunningHandler(t *testing.T) {
	t.Parallel()

	calls := 0
	registry := NewRegistry()
	tool := Tool{
		Name: "probe",
		Parameters: Schema{
			Properties: map[string]ArgType{"a": ArgNumber},
			Required:   []string{"a"},
		},
		Handler: func(_ context.Context, _ map[string]any) (string, error) {
			calls++
			return "ran", nil
		},
	}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	cases := map[string]map[string]any{
		"missing required":    {},
		"wrong type":          {"a": "not-a-number"},
		"unexpected argument": {"a": 1, "b": 2},
	}
	for name, args := range cases {
		if _, err := registry.Call(context.Background(), "probe", args); !errors.Is(err, ErrInvalidArguments) {
			t.Fatalf("%s: Call() error = %v, want ErrInvalidArguments", name, err)
		}
	}
	if calls != 0 {
		t.Fatalf("handler ran %d times, want 0: invalid arguments must not reach the handler", calls)
	}
}

func TestValidationErrorMessageIsReadable(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	_, err := registry.Call(context.Background(), "calculator", map[string]any{"op": "add", "a": "1", "b": 2})
	if err == nil {
		t.Fatal("Call() error = nil, want validation error")
	}
	for _, want := range []string{`"calculator"`, `"a"`, "expected number", "got string"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

func TestHandlerSemanticErrorsStillSurface(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	if _, err := registry.Call(context.Background(), "calculator", map[string]any{"op": "divide", "a": 1, "b": 0}); err == nil {
		t.Fatal("Call(divide by zero) error = nil, want handler error")
	}
	if _, err := registry.Call(context.Background(), "calculator", map[string]any{"op": "pow", "a": 2, "b": 3}); err == nil {
		t.Fatal("Call(unknown op) error = nil, want handler error")
	}
}

func TestDuplicateRegistrationRejected(t *testing.T) {
	t.Parallel()

	registry := newTestRegistry(t)
	if err := registry.Register(calculatorTool()); !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("Register(duplicate) error = %v, want ErrDuplicateTool", err)
	}
}

func TestRunDemoOutput(t *testing.T) {
	t.Parallel()

	out, err := runDemo(context.Background())
	if err != nil {
		t.Fatalf("runDemo() error = %v", err)
	}
	for _, want := range []string{"registered=[calculator get_time]", "calculator=42", "get_time=2026-01-01T08:00:00Z", "rejected="} {
		if !strings.Contains(out, want) {
			t.Fatalf("runDemo() missing %q in %q", want, out)
		}
	}
}
