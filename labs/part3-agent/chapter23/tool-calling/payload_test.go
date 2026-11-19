package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestToolPayloadRejectedBeforeHandler(t *testing.T) {
	calls := 0
	r := NewRegistry()
	if err := r.Register(Tool{Name: "echo", Parameters: Schema{Properties: map[string]ArgType{"text": ArgString}, Required: []string{"text"}}, Handler: func(context.Context, map[string]any) (string, error) { calls++; return "ok", nil }}); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{strings.Repeat("x", 4097), strings.Repeat("\x00", 4096), string([]byte{0xff})} {
		got, err := r.Call(context.Background(), "echo", map[string]any{"text": text})
		if !errors.Is(err, ErrToolInputLimit) || got != "" || calls != 0 {
			t.Fatal("invalid payload executed", len(text), got, err, calls)
		}
	}
	if _, err := r.Call(context.Background(), strings.Repeat("x", 129), nil); !errors.Is(err, ErrToolInputLimit) {
		t.Fatal("unbounded tool name echoed", err)
	}
	if got, err := r.Call(context.Background(), "echo", map[string]any{"text": strings.Repeat("x", 4096)}); err != nil || got != "ok" || calls != 1 {
		t.Fatal("valid string bound rejected", got, err, calls)
	}
}

func TestToolSuccessAndFailureHaveDistinctOutputBoundaries(t *testing.T) {
	cause := errors.New("controlled failure")
	for _, tc := range []struct {
		text             string
		handlerErr, want error
	}{
		{strings.Repeat("x", 4096), nil, nil},
		{strings.Repeat("x", 4097), nil, ErrToolResult},
		{string([]byte{0xff}), nil, ErrToolResult},
		{"partial text", cause, cause},
	} {
		calls := 0
		r := NewRegistry()
		if err := r.Register(Tool{Name: "read", Handler: func(context.Context, map[string]any) (string, error) { calls++; return tc.text, tc.handlerErr }}); err != nil {
			t.Fatal(err)
		}
		got, err := r.Call(context.Background(), "read", nil)
		if !errors.Is(err, tc.want) || calls != 1 || (tc.want != nil && got != "") || (tc.want == nil && got != tc.text) {
			t.Fatal(got, err, calls)
		}
	}
}

func TestToolDefinitionsAndArgumentCollectionsAreBounded(t *testing.T) {
	noop := func(context.Context, map[string]any) (string, error) { return "ok", nil }
	for _, tool := range []Tool{
		{Name: strings.Repeat("x", 129), Handler: noop},
		{Name: "ok", Description: strings.Repeat("x", 1025), Handler: noop},
		{Name: "ok", Parameters: Schema{Properties: map[string]ArgType{strings.Repeat("x", 129): ArgString}}, Handler: noop},
	} {
		if err := NewRegistry().Register(tool); !errors.Is(err, ErrToolInputLimit) {
			t.Fatal("unusable definition accepted", err)
		}
	}
	r := NewRegistry()
	calls := 0
	if err := r.Register(Tool{Name: "echo", Handler: func(context.Context, map[string]any) (string, error) { calls++; return "ok", nil }}); err != nil {
		t.Fatal(err)
	}
	many := map[string]any{}
	for n := 0; n < 33; n++ {
		many[strings.Repeat("x", n)] = "value"
	}
	for _, args := range []map[string]any{
		many, {strings.Repeat("x", 129): "value"}, {string([]byte{0xff}): "value"},
		{"a": strings.Repeat("x", 4096), "b": strings.Repeat("x", 4096), "c": strings.Repeat("x", 4096), "d": strings.Repeat("x", 4096)},
	} {
		if got, err := r.Call(context.Background(), "echo", args); !errors.Is(err, ErrToolInputLimit) || got != "" || calls != 0 {
			t.Fatal("argument collection reached handler", got, err, calls)
		}
	}
}
