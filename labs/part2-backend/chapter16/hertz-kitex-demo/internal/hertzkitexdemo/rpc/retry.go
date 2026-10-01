package rpc

import "github.com/cloudwego/kitex/pkg/retry"

// FailureRetryPolicy converts total RPC attempts into Kitex retry attempts.
func FailureRetryPolicy(maxAttempts int) *retry.FailurePolicy {
	policy := retry.NewFailurePolicy()
	retryTimes := maxAttempts - 1
	if retryTimes < 0 {
		retryTimes = 0
	}
	policy.WithMaxRetryTimes(retryTimes)
	return policy
}
