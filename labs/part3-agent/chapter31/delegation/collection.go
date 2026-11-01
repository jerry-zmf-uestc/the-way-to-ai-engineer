package main

import "sort"

func validateResultSet(scope Scope, parent string, expected map[string]string, results []Result) error {
	if scope.Tenant == "" || scope.User == "" || parent == "" || len(expected) == 0 || len(expected) > 8 || len(results) != len(expected) {
		return ErrResult
	}
	for id, actor := range expected {
		if id == "" || len(id) > 128 || actor == "" || len(actor) > 128 {
			return ErrResult
		}
	}
	seen := map[string]bool{}
	for _, result := range results {
		actor, requested := expected[result.TaskID]
		if !requested || result.Actor != actor || result.Scope != scope || result.Parent != parent || seen[result.TaskID] || len(result.Facts) > 8 || len(result.Missing) > 8 {
			return ErrResult
		}
		seen[result.TaskID] = true
		for _, fact := range result.Facts {
			if !validFact(fact) {
				return ErrResult
			}
		}
		for _, missing := range result.Missing {
			if missing == "" || len(missing) > 256 {
				return ErrResult
			}
		}
	}
	return nil
}

// CollectResults consumes Dispatcher-validated results, preserving evidence and missing goals.
func CollectResults(scope Scope, parent string, expected map[string]string, results []Result) ([]Result, error) {
	if err := validateResultSet(scope, parent, expected, results); err != nil {
		return nil, err
	}
	collected := make([]Result, len(results))
	for n, result := range results {
		collected[n] = result
		collected[n].Facts = append([]Fact(nil), result.Facts...)
		collected[n].Missing = append([]string(nil), result.Missing...)
	}
	sort.Slice(collected, func(i, j int) bool { return collected[i].TaskID < collected[j].TaskID })
	return collected, nil
}
