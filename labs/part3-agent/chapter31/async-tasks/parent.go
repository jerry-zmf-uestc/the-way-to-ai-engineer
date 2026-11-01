package main

import (
	"context"
	"errors"
	"math"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/internal/tasks"
)

var ErrParentResult = errors.New("async: stale goal or invalid child result")

type ParentGoal struct {
	ID       string
	Scope    tasks.Scope
	Revision uint64
}

type ChildBinding struct {
	parentID, taskID             string
	scope                        tasks.Scope
	parentRevision, taskRevision uint64
}

// BindChild consumes a trusted goal and the task returned by Manager.Start/Update.
func BindChild(goal ParentGoal, task tasks.Task) (ChildBinding, error) {
	if goal.ID == "" || len(goal.ID) > 128 || goal.Scope.Tenant == "" || goal.Scope.User == "" || goal.Revision == 0 || task.ID == "" || task.Scope != goal.Scope || task.Revision == 0 {
		return ChildBinding{}, ErrParentResult
	}
	return ChildBinding{goal.ID, task.ID, goal.Scope, goal.Revision, task.Revision}, nil
}

func AdmitChild(goal ParentGoal, binding ChildBinding, result tasks.Task) (string, error) {
	if goal.ID == "" || len(goal.ID) > 128 || goal.Scope.Tenant == "" || goal.Scope.User == "" || goal.Revision == 0 || binding.parentID != goal.ID || binding.parentRevision != goal.Revision || binding.scope != goal.Scope || result.Scope != goal.Scope || binding.taskID == "" || result.ID != binding.taskID || binding.taskRevision == 0 || result.Revision != binding.taskRevision || result.State != "completed" || result.Failure != "" || result.Output == "" || len(result.Output) > 8192 {
		return "", ErrParentResult
	}
	return result.Output, nil
}

// The caller stops parent-result admission before canceling this old goal's children.
func CancelBoundChildren(ctx context.Context, manager *tasks.Manager, goal ParentGoal, bindings []ChildBinding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if manager == nil || goal.ID == "" || goal.Revision == 0 || goal.Scope.Tenant == "" || goal.Scope.User == "" || len(bindings) == 0 || len(bindings) > 8 {
		return ErrParentResult
	}
	seen := map[string]bool{}
	for _, binding := range bindings {
		if binding.parentID != goal.ID || binding.parentRevision != goal.Revision || binding.scope != goal.Scope || binding.taskID == "" || seen[binding.taskID] || binding.taskRevision == 0 || binding.taskRevision == math.MaxUint64 {
			return ErrParentResult
		}
		seen[binding.taskID] = true
	}
	var issues []error
	var waiting []string
	for _, binding := range bindings {
		child, err := manager.Check(goal.Scope, binding.taskID)
		if err != nil {
			issues = append(issues, err)
			continue
		}
		alreadyCanceled := (child.State == "canceling" || child.State == "canceled") && child.Revision == binding.taskRevision+1
		if child.Revision != binding.taskRevision && !alreadyCanceled {
			issues = append(issues, tasks.ErrConflict)
			continue
		}
		if child.State == "running" || child.State == "updating" {
			if _, err := manager.Cancel(goal.Scope, binding.taskID, binding.taskRevision); err != nil {
				issues = append(issues, err)
				continue
			}
		}
		waiting = append(waiting, child.ID)
	}
	for _, id := range waiting {
		if _, err := manager.Wait(ctx, goal.Scope, id); err != nil {
			issues = append(issues, err)
		}
	}
	return errors.Join(issues...)
}
