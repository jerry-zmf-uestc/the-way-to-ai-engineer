package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	errMissingAuthor   = errors.New("author is required")
	errMissingChangeID = errors.New("change ID is required")
	errMissingChecks   = errors.New("at least one review check is required")
)

type reviewInput struct {
	Author       string
	ChangeID     string
	LinesChanged int
	Checks       []string
}

func normalizeChecks(checks []string) map[string]bool {
	seen := make(map[string]bool, len(checks))
	for _, check := range checks {
		name := strings.ToLower(strings.TrimSpace(check))
		if name == "" {
			continue
		}
		seen[name] = true
	}
	return seen
}

func missingRequiredChecks(checks []string) []string {
	seen := normalizeChecks(checks)
	required := []string{"gofmt", "tests", "error-handling"}

	var missing []string
	for _, check := range required {
		if !seen[check] {
			missing = append(missing, check)
		}
	}
	return missing
}

func summarizeReview(ctx context.Context, in reviewInput) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("review canceled: %w", err)
	}

	if strings.TrimSpace(in.Author) == "" {
		return "", errMissingAuthor
	}
	if strings.TrimSpace(in.ChangeID) == "" {
		return "", errMissingChangeID
	}
	if len(normalizeChecks(in.Checks)) == 0 {
		return "", errMissingChecks
	}

	missing := missingRequiredChecks(in.Checks)
	if len(missing) > 0 {
		sort.Strings(missing)
		return "", fmt.Errorf("missing required checks: %s", strings.Join(missing, ", "))
	}

	return fmt.Sprintf("%s reviewed %s: %d lines checked", in.Author, in.ChangeID, in.LinesChanged), nil
}

func main() {
	input := reviewInput{
		Author:       "Lin",
		ChangeID:     "CL-2048",
		LinesChanged: 42,
		Checks:       []string{"gofmt", "tests", "error-handling"},
	}

	summary, err := summarizeReview(context.Background(), input)
	if err != nil {
		fmt.Println("review failed:", err)
		return
	}
	fmt.Println(summary)
}
