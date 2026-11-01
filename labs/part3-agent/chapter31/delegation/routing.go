package main

import "errors"

var (
	ErrRoute   = errors.New("delegation: invalid routing configuration or task kind")
	ErrClarify = errors.New("delegation: role selection needs clarification")
)

// SelectRole consumes an application-bound task kind and an optional untrusted role proposal.
// Configuration is read-only during a run; selection does not execute or authorize tools.
func SelectRole(kind, proposed string, routes map[string][]string, roles map[string]Role) (string, error) {
	validName := func(s string) bool { return s != "" && len(s) <= 128 }
	if !validName(kind) || len(routes) == 0 || len(routes) > 8 || len(roles) == 0 || len(roles) > 8 {
		return "", ErrRoute
	}
	for taskKind, candidates := range routes {
		if !validName(taskKind) || len(candidates) == 0 || len(candidates) > 8 {
			return "", ErrRoute
		}
		seen := map[string]bool{}
		for _, name := range candidates {
			role, ok := roles[name]
			if !validName(name) || seen[name] || !ok || role.Name != name || role.Run == nil {
				return "", ErrRoute
			}
			seen[name] = true
		}
	}
	candidates, ok := routes[kind]
	if !ok {
		return "", ErrRoute
	}
	if proposed != "" {
		if !validName(proposed) {
			return "", ErrDenied
		}
		for _, name := range candidates {
			if proposed == name {
				return name, nil
			}
		}
		return "", ErrDenied
	}
	if len(candidates) != 1 {
		return "", ErrClarify
	}
	return candidates[0], nil
}
