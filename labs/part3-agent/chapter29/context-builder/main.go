package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrBudget   = errors.New("context: mandatory input does not fit")
	ErrProtocol = errors.New("context: invalid message pairing")
)

type Call struct {
	ID, Name string
	Args     json.RawMessage
}
type Message struct {
	Role, Content, CallID string
	Calls                 []Call
}
type Policy struct{ Window, OutputReserve int }
type View struct {
	Messages           []Message
	Used, DroppedTurns int
}

// Counter统计完整候选请求；fake Counter与真实tokenizer必须分别标明。
type Counter func([]Message) int

func estimate(messages []Message) int {
	cost := 0
	for _, m := range messages {
		cost += 4 + utf8.RuneCountInString(m.Role+m.Content+m.CallID)
		for _, c := range m.Calls {
			cost += 4 + utf8.RuneCountInString(c.ID+c.Name+string(c.Args))
		}
	}
	return cost
}
func clone(messages []Message) []Message {
	out := append([]Message(nil), messages...)
	for i, m := range out {
		out[i].Calls = append([]Call(nil), m.Calls...)
		for j, c := range out[i].Calls {
			out[i].Calls[j].Args = append(json.RawMessage(nil), c.Args...)
		}
	}
	return out
}

// turns仅接受完整的调用-观察组。正在等待工具的历史不应送入下一次模型调用。
func turns(history []Message) ([][]Message, error) {
	if len(history) == 0 || len(history) > 128 || history[0].Role != "user" {
		return nil, ErrProtocol
	}
	groups := [][]Message{}
	pending := map[string]bool{}
	seen := map[string]bool{}
	size := 0
	for _, m := range history {
		size += len(m.Content) + len(m.CallID)
		if size > 64<<10 || len(m.Calls) > 64 {
			return nil, ErrProtocol
		}
		if len(m.Calls) > 0 && m.Role != "assistant" {
			return nil, ErrProtocol
		}
		if m.CallID != "" && m.Role != "tool" {
			return nil, ErrProtocol
		}
		switch m.Role {
		case "user":
			if len(pending) > 0 || strings.TrimSpace(m.Content) == "" {
				return nil, ErrProtocol
			}
			groups = append(groups, []Message{})
		case "assistant":
			if len(pending) > 0 {
				return nil, ErrProtocol
			}
			for _, c := range m.Calls {
				size += len(c.ID) + len(c.Name) + len(c.Args)
				if size > 64<<10 {
					return nil, ErrProtocol
				}
				if c.ID == "" || c.Name == "" || seen[c.ID] || !json.Valid(c.Args) {
					return nil, ErrProtocol
				}
				pending[c.ID] = true
				seen[c.ID] = true
			}
		case "tool":
			if !pending[m.CallID] {
				return nil, ErrProtocol
			}
			delete(pending, m.CallID)
		default:
			return nil, ErrProtocol
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], m)
	}
	if len(pending) > 0 || size > 64<<10 {
		return nil, ErrProtocol
	}
	return groups, nil
}

// Build保护system和最新用户轮；旧轮整体选择，原始历史不修改。
func Build(system string, history []Message, p Policy, count Counter) (View, error) {
	if strings.TrimSpace(system) == "" || len(system) > 64<<10 || p.Window <= 0 || p.OutputReserve <= 0 || p.OutputReserve >= p.Window || count == nil {
		return View{}, ErrBudget
	}
	groups, err := turns(history)
	if err != nil {
		return View{}, err
	}
	limit := p.Window - p.OutputReserve
	selected := []Message{{Role: "system", Content: system}}
	selected = append(selected, clone(groups[len(groups)-1])...)
	used := count(selected)
	if used <= 0 || used > limit {
		return View{}, ErrBudget
	}
	dropped := 0
	// ponytail: bounded 128-message history; count each full candidate, optimize only for measured tokenizer cost.
	for i := len(groups) - 2; i >= 0; i-- {
		candidate := []Message{selected[0]}
		candidate = append(candidate, clone(groups[i])...)
		candidate = append(candidate, selected[1:]...)
		candidateCost := count(candidate)
		if candidateCost <= 0 {
			return View{}, ErrBudget
		}
		if candidateCost <= limit {
			selected = candidate
			used = candidateCost
		} else {
			dropped++
		}
	}
	return View{Messages: selected, Used: used, DroppedTurns: dropped}, nil
}
func main() {
	summary := flag.Bool("summary", false, "check declared facts in candidate summaries")
	candidate := flag.String("summary-candidate", "", "validate one raw summary against the declared example snapshot")
	flag.Parse()
	if *candidate != "" {
		if err := inspectCandidate(*candidate); err != nil {
			panic(err)
		}
		return
	}
	if *summary {
		summaryDemo()
		return
	}
	history := []Message{{Role: "user", Content: "old"}, {Role: "assistant", Content: "long historical answer"}, {Role: "user", Content: "query"}, {Role: "assistant", Calls: []Call{{ID: "read-1", Name: "read", Args: json.RawMessage(`{}`)}}}, {Role: "tool", CallID: "read-1", Content: "evidence"}}
	base := append([]Message{{Role: "system", Content: "rules"}}, history[2:]...)
	view, err := Build("rules", history, Policy{Window: estimate(base) + 20, OutputReserve: 20}, estimate)
	if err != nil {
		panic(err)
	}
	fmt.Printf("messages=%d dropped_turns=%d used=%d\n", len(view.Messages), view.DroppedTurns, view.Used)
	for _, m := range view.Messages {
		fmt.Printf("role=%s call_id=%s content=%s\n", m.Role, m.CallID, m.Content)
	}
}
