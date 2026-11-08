package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

var ErrEvidence = errors.New("dag: required report input missing or invalid")

// CheckReportInputs validates this report's numeric inputs; it does not authenticate sources.
func CheckReportInputs(ctx context.Context, deps map[string]string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(deps) != 2 || len(deps["timeout"]) == 0 || len(deps["timeout"]) > 32 || len(deps["retry"]) == 0 || len(deps["retry"]) > 32 {
		return "", ErrEvidence
	}
	timeout, err := strconv.Atoi(deps["timeout"])
	if err != nil || timeout < 1 || timeout > 300 {
		return "", ErrEvidence
	}
	retry, err := strconv.Atoi(deps["retry"])
	if err != nil || retry < 0 || retry > 10 {
		return "", ErrEvidence
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("timeout_seconds=%d retry_count=%d", timeout, retry), nil
}
