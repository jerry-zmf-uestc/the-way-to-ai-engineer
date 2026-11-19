package main

import "math"

type PairLogProb struct{ Chosen, Rejected float64 }

// PreferenceLoss evaluates one DPO pair's objective, without training any model.
func PreferenceLoss(policy, reference PairLogProb, beta float64) (float64, error) {
	if !finite(beta) || beta <= 0 || beta > 1e6 {
		return 0, ErrInput
	}
	for _, value := range []float64{policy.Chosen, policy.Rejected, reference.Chosen, reference.Rejected} {
		if !finite(value) || value > 0 || value < -1e6 {
			return 0, ErrInput
		}
	}
	margin := (policy.Chosen - reference.Chosen) - (policy.Rejected - reference.Rejected)
	z := beta * margin
	// Stable -log(sigmoid(z)), including large positive and negative margins.
	if z >= 0 {
		return math.Log1p(math.Exp(-z)), nil
	}
	return -z + math.Log1p(math.Exp(z)), nil
}
