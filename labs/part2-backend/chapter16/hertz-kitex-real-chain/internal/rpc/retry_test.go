package rpc

import "testing"

func TestFailureRetryPolicyUsesAttemptsMinusFirstCall(t *testing.T) {
	t.Parallel()

	got := FailureRetryPolicy(3)
	if got.StopPolicy.MaxRetryTimes != 2 {
		t.Fatalf("MaxRetryTimes = %d, want 2", got.StopPolicy.MaxRetryTimes)
	}
}

func TestFailureRetryPolicyClampsInvalidAttempts(t *testing.T) {
	t.Parallel()

	for _, attempts := range []int{0, -1} {
		got := FailureRetryPolicy(attempts)
		if got.StopPolicy.MaxRetryTimes != 0 {
			t.Fatalf("attempts %d -> MaxRetryTimes = %d, want 0", attempts, got.StopPolicy.MaxRetryTimes)
		}
	}
}
