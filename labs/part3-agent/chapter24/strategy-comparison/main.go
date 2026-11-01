package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

var (
	ErrInput   = errors.New("invalid comparison input")
	ErrBudget  = errors.New("comparison budget exhausted")
	ErrMissing = errors.New("document missing")
)

type Input struct {
	ID, Root, Version string
	Required          []string
}
type Fact struct{ Value, Source, Version string }
type Document struct {
	ID, Version string
	Values      map[string]string
	Links       map[string][]string
}
type Labels struct {
	Values map[string]string
	Refuse bool
}
type Case struct {
	Input  Input
	Labels Labels
}
type Event struct{ Kind, Goal, Document, Code string }
type Report struct {
	Facts                     map[string]Fact
	Events                    []Event
	Reads, Decisions, Repairs int
	Refused                   bool
	UsageKnown                bool
	Conflicts                 []string
}
type Limits struct{ Reads, Decisions, Repairs, Candidates int }
type Question struct{ ID, Goal, Depends string }
type Node struct{ ID, Goal, Document string }
type Critique struct{ Missing []string }
type runner struct {
	ctx      context.Context
	input    Input
	docs     map[string]Document
	limits   Limits
	report   Report
	observed map[string]Document
}

func cloneDocument(d Document) Document {
	values := map[string]string{}
	for k, v := range d.Values {
		values[k] = v
	}
	links := map[string][]string{}
	for k, v := range d.Links {
		links[k] = append([]string(nil), v...)
	}
	d.Values = values
	d.Links = links
	return d
}
func (r *runner) decision(kind, goal, document string) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if r.report.Decisions >= r.limits.Decisions {
		return ErrBudget
	}
	r.report.Decisions++
	r.report.Events = append(r.report.Events, Event{Kind: kind, Goal: goal, Document: document})
	return nil
}
func (r *runner) read(id string) (Document, error) {
	if err := r.ctx.Err(); err != nil {
		return Document{}, err
	}
	if id == "" || len(id) > 64 {
		return Document{}, ErrInput
	}
	if r.report.Reads >= r.limits.Reads {
		return Document{}, ErrBudget
	}
	r.report.Reads++
	doc, ok := r.docs[id]
	if !ok {
		r.report.Events = append(r.report.Events, Event{Kind: "read", Document: id, Code: "missing"})
		return Document{}, ErrMissing
	}
	r.report.Events = append(r.report.Events, Event{Kind: "read", Document: id})
	r.observed[id] = cloneDocument(doc)
	return cloneDocument(doc), nil
}
func (r *runner) collect(doc Document) {
	if doc.Version != r.input.Version {
		r.report.Events = append(r.report.Events, Event{Kind: "reject_version", Document: doc.ID})
		return
	}
	for _, goal := range r.input.Required {
		if value, ok := doc.Values[goal]; ok && value != "" {
			if old, exists := r.report.Facts[goal]; exists {
				if old.Value != value && !slices.Contains(r.report.Conflicts, goal) {
					r.report.Conflicts = append(r.report.Conflicts, goal)
					r.report.Events = append(r.report.Events, Event{Kind: "fact_conflict", Goal: goal, Document: doc.ID})
				}
				continue
			}
			r.report.Facts[goal] = Fact{Value: value, Source: doc.ID, Version: doc.Version}
		}
	}
}
func Missing(input Input, facts map[string]Fact) Critique {
	c := Critique{}
	for _, goal := range input.Required {
		f, ok := facts[goal]
		if !ok || f.Source == "" || f.Version != input.Version {
			c.Missing = append(c.Missing, goal)
		}
	}
	return c
}
func Questions(input Input) []Question {
	questions := []Question{}
	for n, goal := range input.Required {
		questions = append(questions, Question{ID: fmt.Sprintf("q%d", n+1), Goal: goal, Depends: "root"})
	}
	return questions
}
func Plan(root Document, input Input) []Node {
	nodes := []Node{}
	for n, goal := range input.Required {
		if _, ok := root.Values[goal]; ok && root.Version == input.Version {
			continue
		}
		if ids := root.Links[goal]; len(ids) > 0 {
			nodes = append(nodes, Node{ID: fmt.Sprintf("n%d", n+1), Goal: goal, Document: ids[0]})
		}
	}
	return nodes
}
func (r *runner) attempt(goal, id string) error {
	if err := r.decision("choose", goal, id); err != nil {
		return err
	}
	doc, err := r.read(id)
	if errors.Is(err, ErrMissing) {
		return nil
	}
	if err != nil {
		return err
	}
	r.collect(doc)
	return nil
}
func (r *runner) feedback(root Document) error {
	for _, goal := range r.input.Required {
		if !slices.Contains(Missing(r.input, r.report.Facts).Missing, goal) {
			continue
		}
		ids := root.Links[goal]
		for n, id := range ids {
			if n >= r.limits.Candidates {
				break
			}
			if err := r.attempt(goal, id); err != nil {
				return err
			}
			if !slices.Contains(Missing(r.input, r.report.Facts).Missing, goal) {
				break
			}
		}
	}
	return nil
}

// selfAsk resolves follow-up references using returned observations.
func (r *runner) selfAsk(root Document) error {
	for _, q := range Questions(r.input) {
		current := root
		for depth := 0; depth < 2; depth++ {
			if !slices.Contains(Missing(r.input, r.report.Facts).Missing, q.Goal) {
				break
			}
			ids := current.Links[q.Goal]
			if len(ids) == 0 {
				break
			}
			if err := r.decision("followup_question", q.Goal, current.ID); err != nil {
				return err
			}
			doc, err := r.read(ids[0])
			if errors.Is(err, ErrMissing) {
				break
			}
			if err != nil {
				return err
			}
			r.collect(doc)
			current = doc
		}
	}
	return nil
}
func (r *runner) candidates(goal string) []string {
	keys := make([]string, 0, len(r.observed))
	for id := range r.observed {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	out := []string{}
	seen := map[string]bool{}
	for _, id := range keys {
		for _, candidate := range r.observed[id].Links[goal] {
			if !seen[candidate] {
				out = append(out, candidate)
				seen[candidate] = true
			}
		}
	}
	return out
}

// repair enumerates bounded action candidates from observations, not evaluator labels.
func (r *runner) repair(failedPlan []Node) error {
	critique := Missing(r.input, r.report.Facts)
	if len(critique.Missing) == 0 {
		return nil
	}
	if r.report.Repairs >= r.limits.Repairs {
		return ErrBudget
	}
	r.report.Repairs++
	if err := r.decision("critique", "", ""); err != nil {
		return err
	}
	tried := map[string]bool{}
	for _, node := range failedPlan {
		tried[node.Goal+"\x00"+node.Document] = true
	}
	for _, goal := range critique.Missing {
		explored := 0
		for _, id := range r.candidates(goal) {
			if tried[goal+"\x00"+id] {
				continue
			}
			if explored >= r.limits.Candidates {
				break
			}
			explored++
			if err := r.decision("candidate", goal, id); err != nil {
				return err
			}
			if err := r.attempt(goal, id); err != nil {
				return err
			}
			if !slices.Contains(Missing(r.input, r.report.Facts).Missing, goal) {
				break
			}
		}
	}
	return nil
}
func Run(ctx context.Context, input Input, docs map[string]Document, limits Limits, strategy string) (Report, error) {
	r := runner{ctx: ctx, input: input, docs: docs, observed: map[string]Document{}, limits: limits, report: Report{Facts: map[string]Fact{}}}
	if input.ID == "" || input.Root == "" || input.Version == "" || len(input.Required) == 0 || len(input.Required) > 3 || limits.Reads < 1 || limits.Reads > 16 || limits.Decisions < 1 || limits.Decisions > 32 || limits.Repairs < 0 || limits.Repairs > 1 || limits.Candidates < 1 || limits.Candidates > 2 {
		return r.report, ErrInput
	}
	if !slices.Contains([]string{"direct", "react-control", "self-ask-control", "plan-control", "reflection-control"}, strategy) {
		return r.report, ErrInput
	}
	seen := map[string]bool{}
	for _, goal := range input.Required {
		if goal == "" || len(goal) > 64 || seen[goal] {
			return r.report, ErrInput
		}
		seen[goal] = true
	}
	r.input.Required = append([]string(nil), input.Required...)
	if strategy == "self-ask-control" {
		for _, q := range Questions(r.input) {
			if err := r.decision("subquestion", q.Goal, q.Depends); err != nil {
				return r.report, err
			}
		}
	}
	root, err := r.read(input.Root)
	if err != nil {
		return r.report, err
	}
	r.collect(root)
	switch strategy {
	case "direct":
	case "react-control":
		err = r.feedback(root)
	case "self-ask-control":
		err = r.selfAsk(root)
	default:
		plan := Plan(root, r.input)
		err = r.decision("plan", "", "")
		if err == nil {
			for _, node := range plan {
				if err = r.attempt(node.Goal, node.Document); err != nil {
					break
				}
			}
		}
		if err == nil && strategy == "reflection-control" {
			err = r.repair(plan)
		}
	}
	if stop := ctx.Err(); stop != nil {
		return r.report, stop
	}
	if err != nil {
		return r.report, err
	}
	r.report.Refused = len(Missing(r.input, r.report.Facts).Missing) > 0 || len(r.report.Conflicts) > 0
	return r.report, nil
}
func Complete(input Input, labels Labels, r Report, err error) bool {
	if err != nil {
		return false
	}
	if labels.Refuse {
		return r.Refused
	}
	if r.Refused {
		return false
	}
	for _, goal := range input.Required {
		want, expected := labels.Values[goal]
		f, found := r.Facts[goal]
		if !expected || !found || f.Value != want || f.Version != input.Version || f.Source == "" {
			return false
		}
	}
	return true
}
func fixtures() ([]Case, map[string]Document) {
	docs := map[string]Document{
		"direct":      {ID: "direct", Version: "v2", Values: map[string]string{"timeout": "30"}},
		"multi":       {ID: "multi", Version: "v2", Links: map[string][]string{"timeout": {"timeout"}, "retry": {"retry"}}},
		"alias":       {ID: "alias", Version: "v2", Links: map[string][]string{"timeout": {"missing-id", "timeout"}}},
		"stale":       {ID: "stale", Version: "v2", Links: map[string][]string{"timeout": {"old-timeout", "timeout"}}},
		"nested":      {ID: "nested", Version: "v2", Links: map[string][]string{"timeout": {"router"}}},
		"router":      {ID: "router", Version: "v2", Links: map[string][]string{"timeout": {"timeout"}}},
		"empty":       {ID: "empty", Version: "v2", Links: map[string][]string{"timeout": {"absent"}}},
		"timeout":     {ID: "timeout", Version: "v2", Values: map[string]string{"timeout": "30"}},
		"retry":       {ID: "retry", Version: "v2", Values: map[string]string{"retry": "2"}},
		"old-timeout": {ID: "old-timeout", Version: "v1", Values: map[string]string{"timeout": "25"}},
	}
	cases := []Case{
		{Input: Input{ID: "nested", Root: "nested", Version: "v2", Required: []string{"timeout"}}, Labels: Labels{Values: map[string]string{"timeout": "30"}}},
		{Input: Input{ID: "direct", Root: "direct", Version: "v2", Required: []string{"timeout"}}, Labels: Labels{Values: map[string]string{"timeout": "30"}}},
		{Input: Input{ID: "multi", Root: "multi", Version: "v2", Required: []string{"timeout", "retry"}}, Labels: Labels{Values: map[string]string{"timeout": "30", "retry": "2"}}},
		{Input: Input{ID: "alias", Root: "alias", Version: "v2", Required: []string{"timeout"}}, Labels: Labels{Values: map[string]string{"timeout": "30"}}},
		{Input: Input{ID: "stale", Root: "stale", Version: "v2", Required: []string{"timeout"}}, Labels: Labels{Values: map[string]string{"timeout": "30"}}},
		{Input: Input{ID: "empty", Root: "empty", Version: "v2", Required: []string{"timeout"}}, Labels: Labels{Refuse: true}},
	}
	return cases, docs
}
func main() {
	cases, docs := fixtures()
	limits := Limits{Reads: 8, Decisions: 16, Repairs: 1, Candidates: 2}
	for _, strategy := range []string{"direct", "react-control", "self-ask-control", "plan-control", "reflection-control"} {
		passed, reads, decisions, repairs := 0, 0, 0, 0
		for _, c := range cases {
			report, err := Run(context.Background(), c.Input, docs, limits, strategy)
			if Complete(c.Input, c.Labels, report, err) {
				passed++
			}
			reads += report.Reads
			decisions += report.Decisions
			repairs += report.Repairs
		}
		fmt.Printf("strategy=%s fixture_passed=%d/%d reads=%d decisions=%d repairs=%d usage=unknown\n", strings.TrimSpace(strategy), passed, len(cases), reads, decisions, repairs)
	}
}
