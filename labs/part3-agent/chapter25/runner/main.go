// Command runner 隔离演示运行策略：固定动作序列，无模型、网络或持久存储。
package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type State string

const (
	Pending   State = "pending"
	Running   State = "running"
	Completed State = "completed"
	Failed    State = "failed"
	Canceled  State = "canceled"
)

var (
	ErrBudget    = errors.New("runtime: budget exhausted")
	ErrTemporary = errors.New("runtime: temporary failure")
	ErrInvalid   = errors.New("runtime: invalid contract")
)

type Result struct {
	Text   string
	Tokens int
}

// Job是一个受控动作；Kind只允许model或tool。RetrySafe由应用指定。
type Job struct {
	Name      string
	Kind      string
	RetrySafe bool
	Execute   func(context.Context) (Result, error)
}
type Policy struct {
	MaxModelCalls  int
	MaxToolCalls   int
	MaxTokens      int
	MaxOutputBytes int
	MaxAttempts    int
	Backoff        time.Duration
}
type Event struct {
	Seq     int
	Job     string
	Attempt int
	Kind    string
	State   State
}
type Report struct {
	State      State
	ModelCalls int
	ToolCalls  int
	Tokens     int
	// UsageOverflow时Tokens只保留此前可表示的累加；OverflowUsage保存本次回报。
	UsageOverflow bool
	OverflowUsage int
	Outputs       []string
	Events        []Event
}

func (r *Report) event(job string, attempt int, kind string) {
	r.Events = append(r.Events, Event{Seq: len(r.Events) + 1, Job: job, Attempt: attempt, Kind: kind, State: r.State})
}
func (p Policy) validate(jobs []Job) error {
	if p.MaxModelCalls < 0 || p.MaxToolCalls < 0 || p.MaxTokens <= 0 || p.MaxOutputBytes <= 0 || p.MaxAttempts <= 0 || p.Backoff < 0 {
		return fmt.Errorf("%w: invalid policy", ErrInvalid)
	}
	for _, job := range jobs {
		if strings.TrimSpace(job.Name) == "" || job.Execute == nil || (job.Kind != "model" && job.Kind != "tool") {
			return fmt.Errorf("%w: invalid job", ErrInvalid)
		}
	}
	return nil
}

// reserve在调用前扣次数。尝试失败仍消耗次数，重试不能绕过总配额。
func (r *Report) reserve(p Policy, kind string) error {
	if r.Tokens >= p.MaxTokens {
		return fmt.Errorf("%w: tokens", ErrBudget)
	}
	if kind == "model" {
		if r.ModelCalls >= p.MaxModelCalls {
			return fmt.Errorf("%w: model calls", ErrBudget)
		}
		r.ModelCalls++
	} else {
		if r.ToolCalls >= p.MaxToolCalls {
			return fmt.Errorf("%w: tool calls", ErrBudget)
		}
		r.ToolCalls++
	}
	return nil
}
func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

// Run独占局部Report，同步执行jobs。调用方拿到结果时Run已进入终态。
// 这里不实现动态决策；Agent Loop可在相同策略边界逐次提交动作。
func Run(ctx context.Context, p Policy, jobs []Job) (report Report, err error) {
	report.State = Pending
	defer func() {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			report.State = Canceled
		case err != nil:
			report.State = Failed
		default:
			report.State = Completed
		}
		report.event("", 0, "run_finished")
	}()
	if err = p.validate(jobs); err != nil {
		return
	}
	if err = ctx.Err(); err != nil {
		return
	}
	report.State = Running
	report.event("", 0, "run_started")
	for _, job := range jobs {
		for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
			if err = ctx.Err(); err != nil {
				return
			}
			if err = report.reserve(p, job.Kind); err != nil {
				return
			}
			report.event(job.Name, attempt, "attempt_started")
			result, callErr := job.Execute(ctx)
			// 即使调用失败或取消，也先记已回报的消耗；没有usage不能当作零费用事实。
			if result.Tokens < 0 {
				err = fmt.Errorf("%w: negative usage", ErrInvalid)
				return
			}
			if result.Tokens > int(^uint(0)>>1)-report.Tokens {
				report.UsageOverflow = true
				report.OverflowUsage = result.Tokens
				err = fmt.Errorf("%w: token sum overflow", ErrInvalid)
			} else {
				report.Tokens += result.Tokens
				if report.Tokens > p.MaxTokens {
					err = fmt.Errorf("%w: reported tokens", ErrBudget)
				}
			}
			report.event(job.Name, attempt, "attempt_finished")
			if stop := ctx.Err(); stop != nil {
				err = stop
				return
			}
			if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) {
				err = callErr
				return
			}
			if err != nil {
				return
			}
			if len(result.Text) > p.MaxOutputBytes {
				err = fmt.Errorf("%w: output bytes", ErrBudget)
				return
			}
			if callErr == nil {
				report.Outputs = append(report.Outputs, result.Text)
				break
			}
			if !job.RetrySafe || !errors.Is(callErr, ErrTemporary) || attempt == p.MaxAttempts {
				err = fmt.Errorf("runtime: job %s: %w", job.Name, callErr)
				return
			}
			report.event(job.Name, attempt, "retry_wait")
			if err = wait(ctx, p.Backoff); err != nil {
				return
			}
		}
	}
	return
}
func main() {
	attempts := 0
	p := Policy{MaxModelCalls: 3, MaxToolCalls: 1, MaxTokens: 100, MaxOutputBytes: 128, MaxAttempts: 2}
	jobs := []Job{
		{Name: "decide", Kind: "model", RetrySafe: true, Execute: func(context.Context) (Result, error) {
			attempts++
			if attempts == 1 {
				return Result{Tokens: 3}, ErrTemporary
			}
			return Result{Text: "read", Tokens: 5}, nil
		}},
		{Name: "read", Kind: "tool", Execute: func(context.Context) (Result, error) { return Result{Text: "fixed evidence"}, nil }},
	}
	r, err := Run(context.Background(), p, jobs)
	fmt.Printf("state=%s model_calls=%d tool_calls=%d tokens=%d outputs=%v error=%v\n", r.State, r.ModelCalls, r.ToolCalls, r.Tokens, r.Outputs, err)
	for _, event := range r.Events {
		fmt.Printf("seq=%d kind=%s job=%s attempt=%d state=%s\n", event.Seq, event.Kind, event.Job, event.Attempt, event.State)
	}
}
