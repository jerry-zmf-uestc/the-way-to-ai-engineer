package main

import (
	"context"
	"errors"
)

func RunReadyStream(ctx context.Context, input <-chan ReadyJob, capacity map[string]int, parallel, perTenant, maxTasks int) (ResourceReport, error) {
	out := ResourceReport{Report: Report{States: map[string]string{}, Results: map[string]string{}}, Free: map[string]int{}, Dispatch: []string{}}
	if input == nil || len(capacity) < 1 || len(capacity) > 8 || parallel < 1 || parallel > 8 || perTenant < 1 || perTenant > 16 || maxTasks < 1 || maxTasks > 128 {
		return out, ErrGraph
	}
	limits := map[string]int{}
	pool := resourcePool{free: map[string]int{}, held: map[string][]string{}}
	for name, n := range capacity {
		if name == "" || len(name) > 128 || n < 1 || n > 8 {
			return out, ErrGraph
		}
		limits[name] = n
		pool.free[name] = n
	}
	defer func() {
		for name, n := range pool.free {
			out.Free[name] = n
		}
	}()
	queues := map[string][]ReadyJob{}
	tenants := []string{}
	last := -1
	accepted, active := 0, 0
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	type completion struct {
		id, text string
		err      error
	}
	done := make(chan completion, parallel)
	stopping := false
	var cause error
	stop := func(err error) {
		if !stopping {
			stopping = true
			cause = errors.Join(cause, err)
			cancel()
		}
		for id, state := range out.States {
			if state == "queued" {
				out.States[id] = "canceled"
			}
		}
	}
	admit := func(job ReadyJob) error {
		if job.ID == "" || len(job.ID) > 128 || job.Tenant == "" || len(job.Tenant) > 128 || job.Run == nil || len(job.Needs) < 1 || len(job.Needs) > 8 {
			return ErrGraph
		}
		if _, exists := out.States[job.ID]; exists {
			return ErrGraph
		}
		seen := map[string]bool{}
		for _, name := range job.Needs {
			if limits[name] < 1 || seen[name] {
				return ErrGraph
			}
			seen[name] = true
		}
		if accepted >= maxTasks || len(queues[job.Tenant]) >= perTenant {
			return ErrQueue
		}
		if _, exists := queues[job.Tenant]; !exists {
			tenants = append(tenants, job.Tenant)
		}
		job.Needs = append([]string(nil), job.Needs...)
		queues[job.Tenant] = append(queues[job.Tenant], job)
		out.States[job.ID] = "queued"
		out.Events = append(out.Events, "accepted:"+job.ID)
		accepted++
		return nil
	}
	pick := func() (ReadyJob, bool) {
		for offset := 1; offset <= len(tenants); offset++ {
			n := (last + offset) % len(tenants)
			queue := queues[tenants[n]]
			if len(queue) > 0 && pool.take(queue[0]) {
				queues[tenants[n]] = queue[1:]
				last = n
				return queue[0], true
			}
		}
		return ReadyJob{}, false
	}
	finish := func(result completion) {
		pool.release(result.id)
		active--
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
		out.Events = append(out.Events, "finished:"+result.id+":"+out.States[result.id])
	}
	for {
		if err := ctx.Err(); err != nil {
			stop(err)
		}
		if stopping {
			if active == 0 {
				break
			}
			finish(<-done)
			continue
		}
		for active < parallel {
			job, ok := pick()
			if !ok {
				break
			}
			out.States[job.ID] = "running"
			out.Dispatch = append(out.Dispatch, job.ID)
			active++
			out.Peak = max(out.Peak, active)
			go func(job ReadyJob) { text, err := executeWork(child, job.Run); done <- completion{job.ID, text, err} }(job)
		}
		pending := false
		for _, queue := range queues {
			pending = pending || len(queue) > 0
		}
		if input == nil && active == 0 {
			if pending {
				stop(ErrGraph)
			}
			break
		}
		select {
		case <-ctx.Done():
			stop(ctx.Err())
		case result := <-done:
			if err := ctx.Err(); err != nil {
				stop(err)
			}
			finish(result)
		case job, ok := <-input:
			if !ok {
				input = nil
				continue
			}
			if err := ctx.Err(); err != nil {
				stop(err)
				continue
			}
			if err := admit(job); err != nil {
				stop(err)
			}
		}
	}
	return out, cause
}
