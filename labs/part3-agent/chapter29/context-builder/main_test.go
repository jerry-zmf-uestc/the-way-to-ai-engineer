package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func history() []Message {
	return []Message{{Role: "user", Content: "old"}, {Role: "assistant", Content: "old answer"}, {Role: "user", Content: "new"}, {Role: "assistant", Calls: []Call{{ID: "c", Name: "read", Args: json.RawMessage(`{"q":"x"}`)}}}, {Role: "tool", CallID: "c", Content: "evidence"}}
}
func TestWholeTurnSelectionAndOwnership(t *testing.T) {
	h := history()
	original := clone(h)
	required := append([]Message{{Role: "system", Content: "rules"}}, h[2:]...)
	p := Policy{Window: estimate(required) + 10, OutputReserve: 10}
	view, err := Build("rules", h, p, estimate)
	if err != nil || len(view.Messages) != 4 || view.DroppedTurns != 1 || view.Used != estimate(required) {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	if !reflect.DeepEqual(h, original) {
		t.Fatal("history changed")
	}
	view.Messages[2].Calls[0].Args[6] = 'Y'
	if !reflect.DeepEqual(h, original) {
		t.Fatal("shared argument bytes")
	}
	p.Window--
	if _, err := Build("rules", h, p, estimate); !errors.Is(err, ErrBudget) {
		t.Fatal("mandatory turn silently truncated")
	}
}
func TestAllHistoryAndFramingCounter(t *testing.T) {
	h := history()
	view, err := Build("rules", h, Policy{Window: 500, OutputReserve: 20}, estimate)
	if err != nil || view.DroppedTurns != 0 || len(view.Messages) != len(h)+1 || !reflect.DeepEqual(view.Messages[1:], h) {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	// 请求整体计数可包含额外的 framing；不能把字符数或逐条加和冒充准确Token。
	counter := func(m []Message) int { return len(m)*10 + 7 }
	view, err = Build("rules", h, Policy{Window: 57, OutputReserve: 10}, counter)
	if err != nil || view.Used != 47 || view.DroppedTurns != 1 {
		t.Fatalf("framing lost: %+v %v", view, err)
	}
}
func TestRejectsIncompleteAndInvalidPairings(t *testing.T) {
	base := history()
	for _, h := range [][]Message{
		{}, base[:4], {{Role: "user", Content: "q"}, {Role: "tool", CallID: "unknown"}},
		{{Role: "user", Content: "q"}, {Role: "assistant", Calls: []Call{{ID: "c", Name: "read", Args: json.RawMessage(`{}`)}}}, {Role: "user", Content: "early"}},
		{{Role: "system", Content: "external system"}},
		{{Role: "user", Calls: []Call{{ID: "x", Name: "read", Args: json.RawMessage(`{}`)}}}},
	} {
		if _, err := Build("rules", h, Policy{Window: 500, OutputReserve: 20}, estimate); !errors.Is(err, ErrProtocol) {
			t.Fatalf("invalid history accepted: %+v %v", h, err)
		}
	}
}
func TestInvalidBudgetAndCounter(t *testing.T) {
	for _, p := range []Policy{{Window: 0, OutputReserve: 1}, {Window: 20, OutputReserve: 20}, {Window: 20, OutputReserve: 0}} {
		if _, err := Build("rules", history(), p, estimate); !errors.Is(err, ErrBudget) {
			t.Fatal("invalid budget accepted")
		}
	}
	if _, err := Build("rules", history(), Policy{500, 20}, func([]Message) int { return -1 }); !errors.Is(err, ErrBudget) {
		t.Fatal("negative count accepted")
	}
}
