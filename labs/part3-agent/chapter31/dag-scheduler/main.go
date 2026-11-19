package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"
)

var (
	ErrGraph  = errors.New("dag: invalid graph")
	ErrQueue  = errors.New("dag: ready queue full")
	ErrFailed = errors.New("dag: task failure")
)

type Node struct {
	ID      string
	Depends []string
	Run     func(context.Context, map[string]string) (string, error)
}
type Report struct {
	States, Results map[string]string
	Events          []string
	Peak            int
}

func Validate(nodes []Node) error {
	if len(nodes) == 0 || len(nodes) > 16 {
		return ErrGraph
	}
	byID := map[string]Node{}
	for _, n := range nodes {
		if n.ID == "" || len(n.ID) > 128 || len(n.Depends) > 16 || n.Run == nil {
			return ErrGraph
		}
		if _, exists := byID[n.ID]; exists {
			return ErrGraph
		}
		byID[n.ID] = n
	}
	colors := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if colors[id] == 1 {
			return ErrGraph
		}
		if colors[id] == 2 {
			return nil
		}
		node, ok := byID[id]
		if !ok {
			return ErrGraph
		}
		colors[id] = 1
		seen := map[string]bool{}
		for _, dep := range node.Depends {
			if seen[dep] {
				return ErrGraph
			}
			seen[dep] = true
			if err := visit(dep); err != nil {
				return err
			}
		}
		colors[id] = 2
		return nil
	}
	for id := range byID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

// Run只有调度器写Report。任务取得独立依赖结果，只读任务须协作响应ctx。
func Run(ctx context.Context, nodes []Node, parallel, queueLimit int) (Report, error) {
	report := Report{States: map[string]string{}, Results: map[string]string{}}
	if err := Validate(nodes); err != nil {
		return report, err
	}
	if parallel <= 0 || parallel > 8 || queueLimit <= 0 || queueLimit > 16 {
		return report, ErrGraph
	}
	for _, node := range nodes {
		report.States[node.ID] = "pending"
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	type completion struct {
		id, text string
		err      error
	}
	done := make(chan completion, parallel)
	active, terminal := 0, 0
	queue := []Node{}
	var cause error
	stopping := false
	for terminal < len(nodes) {
		// 外层取消停止派发，并收齐已开始任务，不能遗留发送阻塞的goroutine。
		if ctx.Err() != nil && !stopping {
			cause = errors.Join(cause, ctx.Err())
			stopping = true
			cancel()
		}
		if !stopping {
			changed := true
			for changed {
				changed = false
				for _, node := range nodes {
					if report.States[node.ID] != "pending" {
						continue
					}
					blocked := false
					ready := true
					for _, dep := range node.Depends {
						switch report.States[dep] {
						case "failed", "blocked", "canceled":
							blocked = true
						case "completed":
						default:
							ready = false
						}
					}
					if blocked {
						report.States[node.ID] = "blocked"
						report.Events = append(report.Events, "blocked:"+node.ID)
						terminal++
						changed = true
						continue
					}
					if ready {
						if len(queue) >= queueLimit {
							cause = errors.Join(cause, ErrQueue)
							stopping = true
							cancel()
							break
						}
						copy := node
						copy.Depends = append([]string(nil), node.Depends...)
						queue = append(queue, copy)
						report.States[node.ID] = "queued"
						report.Events = append(report.Events, "queued:"+node.ID)
					}
				}
				if stopping {
					break
				}
			}
		}
		if stopping {
			queue = nil
			for id, state := range report.States {
				if state == "pending" || state == "queued" {
					report.States[id] = "canceled"
					terminal++
				}
			}
		}
		for !stopping && active < parallel && len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			deps := map[string]string{}
			for _, dep := range node.Depends {
				deps[dep] = report.Results[dep]
			}
			report.States[node.ID] = "running"
			report.Events = append(report.Events, "started:"+node.ID)
			active++
			report.Peak = max(report.Peak, active)
			go func(n Node, input map[string]string) {
				text, err := executeWork(child, func(ctx context.Context) (string, error) {
					return n.Run(ctx, input)
				})
				done <- completion{id: n.ID, text: text, err: err}
			}(node, deps)
		}
		if active == 0 {
			if terminal == len(nodes) {
				break
			}
			if stopping {
				break
			}
			return report, ErrGraph
		}
		var result completion
		if stopping {
			result = <-done
		} else {
			select {
			case result = <-done:
			case <-ctx.Done():
				continue
			}
		}
		if !stopping && ctx.Err() != nil {
			cause = errors.Join(cause, ctx.Err())
			stopping = true
			cancel()
		}
		active--
		terminal++
		switch {
		case stopping:
			report.States[result.id] = "canceled"
		case result.err != nil:
			report.States[result.id] = "failed"
			cause = errors.Join(cause, ErrFailed, result.err)
		default:
			report.States[result.id] = "completed"
			report.Results[result.id] = result.text
		}
		report.Events = append(report.Events, "finished:"+result.id+":"+report.States[result.id])
	}
	return report, cause
}
func main() {
	resources := flag.Bool("resources", false, "compare finite ready-job resource admission")
	live := flag.Bool("live", false, "show admission rejection and producer shutdown")
	flag.Parse()
	if *live && *resources {
		panic("choose one demonstration")
	}
	if *live {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		report, err := liveDemo(ctx)
		if !errors.Is(err, ErrQueue) {
			panic("expected queue admission rejection")
		}
		fmt.Printf("cause=queue_full producer_stopped=true free_db=%d results=%d\n", report.Free["db"], len(report.Results))
		return
	}
	if *resources {
		jobs := []ReadyJob{}
		for _, id := range []string{"a1", "a2", "a3", "b1", "b2"} {
			jobs = append(jobs, ReadyJob{ID: id, Tenant: id[:1], Needs: []string{"db"}, Run: func(context.Context) (string, error) { return "read", nil }})
		}
		fifo, err := RunReady(context.Background(), jobs, map[string]int{"db": 1}, 2, 3, false)
		if err != nil {
			panic(err)
		}
		rr, err := RunReady(context.Background(), jobs, map[string]int{"db": 1}, 2, 3, true)
		if err != nil {
			panic(err)
		}
		fmt.Printf("fifo=%v round_robin=%v completed=%d peak=%d free_db=%d\n", fifo.Dispatch, rr.Dispatch, len(rr.Results), rr.Peak, rr.Free["db"])
		return
	}
	nodes := []Node{
		{ID: "timeout", Run: func(context.Context, map[string]string) (string, error) { return "30", nil }},
		{ID: "retry", Run: func(context.Context, map[string]string) (string, error) { return "2", nil }},
		{ID: "summary", Depends: []string{"timeout", "retry"}, Run: func(_ context.Context, deps map[string]string) (string, error) {
			return "timeout=" + deps["timeout"] + " retry=" + deps["retry"], nil
		}},
	}
	report, err := Run(context.Background(), nodes, 2, 4)
	if err != nil {
		panic(err)
	}
	fmt.Printf("completed=%d peak=%d summary=%q\n", len(report.Results), report.Peak, report.Results["summary"])
}
