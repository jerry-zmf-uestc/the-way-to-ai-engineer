package main

import (
	"errors"
	"math"
	"testing"
)

func TestPreferenceLossIsReferenceRelative(t *testing.T) {
	reference := PairLogProb{math.Log(.5), math.Log(.5)}
	for _, prob := range []float64{.2, .5, .8} {
		policy := PairLogProb{math.Log(prob), math.Log(1 - prob)}
		loss, err := PreferenceLoss(policy, reference, 1)
		if err != nil || math.Abs(loss+math.Log(prob)) > 1e-12 {
			t.Fatal(prob, loss, err)
		}
	}
	policy := PairLogProb{math.Log(.9), math.Log(.1)}
	loss, err := PreferenceLoss(policy, policy, 1)
	if err != nil || math.Abs(loss-math.Log(2)) > 1e-12 {
		t.Fatal("reference ignored", loss, err)
	}
	for _, policy := range []PairLogProb{{0, -1e6}, {-1e6, 0}} {
		loss, err := PreferenceLoss(policy, reference, 1e6)
		if err != nil || !finite(loss) || loss < 0 {
			t.Fatal("unstable loss", loss, err)
		}
	}
}
func TestPreferenceInvalidInputDoesNotBecomeProbability(t *testing.T) {
	for _, beta := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := PreferenceLoss(PairLogProb{}, PairLogProb{}, beta); !errors.Is(err, ErrInput) {
			t.Fatal(beta, err)
		}
	}
	if _, err := PreferenceLoss(PairLogProb{Chosen: .1}, PairLogProb{}, 1); !errors.Is(err, ErrInput) {
		t.Fatal("positive log probability", err)
	}
}
