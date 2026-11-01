package main

import (
	"errors"
	"math"
	"slices"
	"testing"
)

func closeVec(a, b Vec) bool { return math.Abs(a[0]-b[0]) < 1e-12 && math.Abs(a[1]-b[1]) < 1e-12 }
func TestBytePairsRoundTripRanksAndUTF8(t *testing.T) {
	tokenizer, err := NewTokenizer([]Merge{{int('A'), int('I')}})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"AI学习 AI", " leading spaces\n", "🙂AI", ""} {
		ids, err := tokenizer.Encode(text)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := tokenizer.Decode(ids)
		if err != nil || decoded != text {
			t.Fatal(decoded, err)
		}
	}
	ids, _ := tokenizer.Encode("AI学习 AI")
	if len(ids) != 9 {
		t.Fatal(ids)
	}
	first, _ := NewTokenizer([]Merge{{int('a'), int('b')}, {int('b'), int('c')}})
	second, _ := NewTokenizer([]Merge{{int('b'), int('c')}, {int('a'), int('b')}})
	a, _ := first.Encode("abc")
	b, _ := second.Encode("abc")
	if slices.Equal(a, b) {
		t.Fatal("merge priority ignored")
	}
	if _, err := tokenizer.Decode([]int{0xe5}); !errors.Is(err, ErrInput) {
		t.Fatal("invalid UTF8 accepted")
	}
	if _, err := NewTokenizer([]Merge{{256, 1}}); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
}
func TestCausalPrefixUnaffectedByFuture(t *testing.T) {
	left, _, err := FullCausal([]int{0, 1, 2})
	if err != nil {
		t.Fatal(err)
	}
	right, _, err := FullCausal([]int{0, 1, 3})
	if err != nil {
		t.Fatal(err)
	}
	if !closeVec(left[0], right[0]) || !closeVec(left[1], right[1]) || closeVec(left[2], right[2]) {
		t.Fatal(left, right)
	}
	out, weights, err := Attention(Vec{0, 0}, []Vec{{1, 0}, {0, 1}}, []Vec{{2, 0}, {0, 2}})
	if err != nil || !closeVec(out, Vec{1, 1}) || math.Abs(weights[0]-.5) > 1e-12 {
		t.Fatal(out, weights, err)
	}
}
func TestKVCacheMatchesFullAndRejectsChangedIdentityPrefix(t *testing.T) {
	identity := CacheIdentity{"weights-v1", "tokenizer-v1", "position-v1"}
	cache := NewCache(identity)
	tokens := []int{0, 1, 2, 3}
	for n := 1; n <= len(tokens); n++ {
		full, _, err := FullCausal(tokens[:n])
		if err != nil {
			t.Fatal(err)
		}
		cached, err := cache.Step(identity, tokens[:n])
		if err != nil || !closeVec(full[n-1], cached) {
			t.Fatal(full, cached, err)
		}
	}
	if cache.Projections != 4 || cache.ScorePairs != 10 {
		t.Fatal(cache)
	}
	changed := append([]int(nil), tokens...)
	changed[0] = 3
	changed = append(changed, 1)
	if _, err := cache.Step(identity, changed); !errors.Is(err, ErrCache) {
		t.Fatal(err)
	}
	for _, id := range []CacheIdentity{{"v2", identity.Tokenizer, identity.Position}, {identity.Weights, "v2", identity.Position}, {identity.Weights, identity.Tokenizer, "v2"}} {
		if _, err := cache.Step(id, append(tokens, 1)); !errors.Is(err, ErrCache) {
			t.Fatal(err)
		}
	}
	if cache.Projections != 4 {
		t.Fatal("rejected suffix changed state")
	}
}
func entropy(p []float64) float64 {
	h := 0.0
	for _, v := range p {
		if v > 0 {
			h -= v * math.Log(v)
		}
	}
	return h
}
func TestTemperatureTopPAndExplicitDraw(t *testing.T) {
	greedy, _ := Distribution([]float64{2, 1, 0}, 0, 1)
	if !slices.Equal(greedy, []float64{1, 0, 0}) {
		t.Fatal(greedy)
	}
	cold, _ := Distribution([]float64{2, 1, 0}, .5, 1)
	hot, _ := Distribution([]float64{2, 1, 0}, 2, 1)
	if entropy(hot) <= entropy(cold) {
		t.Fatal(cold, hot)
	}
	nucleus, _ := Distribution([]float64{2, 1, 0}, 1, .8)
	if nucleus[2] != 0 {
		t.Fatal(nucleus)
	}
	for _, u := range []float64{0, .2, .8, .999999} {
		id, err := Sample(nucleus, u)
		if err != nil || id == 2 {
			t.Fatal(id, err)
		}
	}
	if _, err := Distribution([]float64{math.NaN()}, 1, 1); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	if _, _, err := Attention(Vec{math.Inf(1), 0}, []Vec{{1, 1}}, []Vec{{1, 1}}); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	stable, err := Softmax([]float64{10000, 10000})
	if err != nil || stable[0] != .5 {
		t.Fatal(stable, err)
	}
}
