package main

import (
	"context"
	"errors"
)

type ReadyJob struct {
	ID, Tenant string
	Needs      []string
	Run        func(context.Context) (string, error)
}
type resourcePool struct {
	free map[string]int
	held map[string][]string
}

// The scheduler owns the pool; workers never receive it or hold partial reservations.
func (p *resourcePool) take(job ReadyJob) bool {
	for _, name := range job.Needs {
		if p.free[name] < 1 {
			return false
		}
	}
	for _, name := range job.Needs {
		p.free[name]--
	}
	p.held[job.ID] = append([]string(nil), job.Needs...)
	return true
}
func (p *resourcePool) release(id string) {
	for _, name := range p.held[id] {
		p.free[name]++
	}
	delete(p.held, id)
}

type ResourceReport struct {
	Report
	Dispatch []string
	Free     map[string]int
}

func RunReady(ctx context.Context, jobs []ReadyJob, capacity map[string]int, parallel, perTenant int, roundRobin bool) (ResourceReport, error) {
	out := ResourceReport{Report: Report{States: map[string]string{}, Results: map[string]string{}}, Free: map[string]int{}, Dispatch: []string{}}
	if len(jobs) < 1 || len(jobs) > 16 || len(capacity) < 1 || len(capacity) > 8 || parallel < 1 || parallel > 8 || perTenant < 1 || perTenant > 16 {
		return out, ErrGraph
	}
	pool := resourcePool{free: map[string]int{}, held: map[string][]string{}}
	for name, n := range capacity {
		if name == "" || n < 1 || n > 8 {
			return out, ErrGraph
		}
		pool.free[name] = n
	}
	queues := map[string][]ReadyJob{}
	tenants := []string{}
	pending := []ReadyJob{}
	ids := map[string]bool{}
	for _, job := range jobs {
		if job.ID == "" || job.Tenant == "" || len(job.ID) > 128 || len(job.Tenant) > 128 || len(job.Needs) < 1 || len(job.Needs) > 8 || job.Run == nil {
			return out, ErrGraph
		}
		if ids[job.ID] {
			return out, ErrGraph
		}
		ids[job.ID] = true
		seen := map[string]bool{}
		for _, name := range job.Needs {
			if pool.free[name] < 1 || seen[name] {
				return out, ErrGraph
			}
			seen[name] = true
		}
		if len(queues[job.Tenant]) >= perTenant {
			return out, ErrQueue
		}
		if _, ok := queues[job.Tenant]; !ok {
			tenants = append(tenants, job.Tenant)
		}
		job.Needs = append([]string(nil), job.Needs...)
		queues[job.Tenant] = append(queues[job.Tenant], job)
		pending = append(pending, job)
	}
	for _, job := range pending {
		out.States[job.ID] = "queued"
	}
	cursor := 0
	pick := func() (ReadyJob, bool) {
		if !roundRobin {
			for n, job := range pending {
				if pool.take(job) {
					pending = append(pending[:n], pending[n+1:]...)
					return job, true
				}
			}
			return ReadyJob{}, false
		}
		for offset := 0; offset < len(tenants); offset++ {
			n := (cursor + offset) % len(tenants)
			tenant := tenants[n]
			queue := queues[tenant]
			if len(queue) > 0 && pool.take(queue[0]) {
				queues[tenant] = queue[1:]
				cursor = (n + 1) % len(tenants)
				return queue[0], true
			}
		}
		return ReadyJob{}, false
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	type completion struct {
		id, text string
		err      error
	}
	done := make(chan completion, parallel)
	active, finished := 0, 0
	stopping := false
	var cause error
	for finished < len(jobs) {
		if ctx.Err() != nil && !stopping {
			stopping = true
			cause = errors.Join(cause, ctx.Err())
			cancel()
		}
		if stopping {
			for id, state := range out.States {
				if state == "queued" {
					out.States[id] = "canceled"
					finished++
				}
			}
		}
		for !stopping && active < parallel {
			job, ok := pick()
			if !ok {
				break
			}
			out.States[job.ID] = "running"
			out.Dispatch = append(out.Dispatch, job.ID)
			active++
			out.Peak = max(out.Peak, active)
			go func(job ReadyJob) {
				text, err := executeWork(child, job.Run)
				done <- completion{job.ID, text, err}
			}(job)
		}
		if active == 0 {
			if finished == len(jobs) {
				break
			}
			return out, ErrGraph
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
		if ctx.Err() != nil && !stopping {
			stopping = true
			cause = errors.Join(cause, ctx.Err())
			cancel()
		}
		pool.release(result.id)
		active--
		finished++
		switch {
		case stopping:
			out.States[result.id] = "canceled"
		case result.err != nil:
			out.States[result.id] = "failed"
			cause = errors.Join(cause, ErrFailed, result.err)
		default:
			out.States[result.id] = "completed"
			out.Results[result.id] = result.text
		}
	}
	for name, n := range pool.free {
		out.Free[name] = n
	}
	return out, cause
}
