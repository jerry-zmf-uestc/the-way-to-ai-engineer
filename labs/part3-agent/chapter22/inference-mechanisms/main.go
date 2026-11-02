package main

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"unicode/utf8"
)

var (
	ErrInput = errors.New("invalid mechanism input")
	ErrCache = errors.New("cached prefix or identity changed")
)

type Merge struct{ Left, Right int }
type Tokenizer struct {
	merges []Merge
	vocab  [][]byte
}

func NewTokenizer(merges []Merge) (*Tokenizer, error) {
	if len(merges) > 64 {
		return nil, ErrInput
	}
	t := &Tokenizer{merges: append([]Merge(nil), merges...)}
	for n := 0; n < 256; n++ {
		t.vocab = append(t.vocab, []byte{byte(n)})
	}
	seen := map[Merge]bool{}
	for _, m := range merges {
		if m.Left < 0 || m.Right < 0 || m.Left >= len(t.vocab) || m.Right >= len(t.vocab) || seen[m] {
			return nil, ErrInput
		}
		seen[m] = true
		piece := append(append([]byte(nil), t.vocab[m.Left]...), t.vocab[m.Right]...)
		if len(piece) > 256 {
			return nil, ErrInput
		}
		t.vocab = append(t.vocab, piece)
	}
	return t, nil
}
func (t *Tokenizer) Encode(text string) ([]int, error) {
	if len(text) > 4096 || !utf8.ValidString(text) {
		return nil, ErrInput
	}
	ids := make([]int, len(text))
	for n, b := range []byte(text) {
		ids[n] = int(b)
	}
	for {
		bestRank, bestPosition := -1, -1
		for position := 0; position+1 < len(ids); position++ {
			for rank, m := range t.merges {
				if ids[position] == m.Left && ids[position+1] == m.Right && (bestRank < 0 || rank < bestRank) {
					bestRank = rank
					bestPosition = position
				}
			}
		}
		if bestRank < 0 {
			return ids, nil
		}
		ids[bestPosition] = 256 + bestRank
		ids = append(ids[:bestPosition+1], ids[bestPosition+2:]...)
	}
}
func (t *Tokenizer) Decode(ids []int) (string, error) {
	if len(ids) > 4096 {
		return "", ErrInput
	}
	out := []byte{}
	for _, id := range ids {
		if id < 0 || id >= len(t.vocab) || len(out)+len(t.vocab[id]) > 4096 {
			return "", ErrInput
		}
		out = append(out, t.vocab[id]...)
	}
	if !utf8.Valid(out) {
		return "", ErrInput
	}
	return string(out), nil
}

type Vec [2]float64
type Projection struct{ Q, K, V Vec }

func project(token, position int) (Projection, error) {
	if token < 0 || token >= 4 || position < 0 || position >= 64 {
		return Projection{}, ErrInput
	}
	embedding := [4]Vec{{1, 0}, {0, 1}, {1, 1}, {-1, .5}}
	x := embedding[token]
	x[1] += float64(position) * .01 // Fixed positional fixture, not RoPE or learned production weights.
	return Projection{Q: x, K: Vec{.5*x[0] + x[1], x[0] - .5*x[1]}, V: Vec{x[0] - x[1], .25*x[0] + x[1]}}, nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func Softmax(logits []float64) ([]float64, error) {
	if len(logits) == 0 || len(logits) > 256 {
		return nil, ErrInput
	}
	highest := math.Inf(-1)
	for _, v := range logits {
		if !finite(v) {
			return nil, ErrInput
		}
		highest = max(highest, v)
	}
	probs := make([]float64, len(logits))
	total := 0.0
	for n, v := range logits {
		probs[n] = math.Exp(v - highest)
		total += probs[n]
	}
	for n := range probs {
		probs[n] /= total
	}
	return probs, nil
}
func Attention(q Vec, keys, values []Vec) (Vec, []float64, error) {
	if len(keys) == 0 || len(keys) > 64 || len(keys) != len(values) {
		return Vec{}, nil, ErrInput
	}
	for _, vectors := range [][]Vec{{q}, keys, values} {
		for _, v := range vectors {
			for _, x := range v {
				if !finite(x) || math.Abs(x) > 1_000_000 {
					return Vec{}, nil, ErrInput
				}
			}
		}
	}
	scores := make([]float64, len(keys))
	for n, k := range keys {
		scores[n] = (q[0]*k[0] + q[1]*k[1]) / math.Sqrt(2)
	}
	weights, err := Softmax(scores)
	if err != nil {
		return Vec{}, nil, err
	}
	out := Vec{}
	for n, v := range values {
		out[0] += weights[n] * v[0]
		out[1] += weights[n] * v[1]
	}
	return out, weights, nil
}
func FullCausal(tokens []int) ([]Vec, int, error) {
	if len(tokens) == 0 || len(tokens) > 64 {
		return nil, 0, ErrInput
	}
	projected := make([]Projection, len(tokens))
	keys := []Vec{}
	values := []Vec{}
	for n, t := range tokens {
		p, err := project(t, n)
		if err != nil {
			return nil, 0, err
		}
		projected[n] = p
		keys = append(keys, p.K)
		values = append(values, p.V)
	}
	outputs := []Vec{}
	for n, p := range projected {
		v, _, err := Attention(p.Q, keys[:n+1], values[:n+1])
		if err != nil {
			return nil, 0, err
		}
		outputs = append(outputs, v)
	}
	return outputs, len(tokens), nil
}

type CacheIdentity struct{ Weights, Tokenizer, Position string }
type KVCache struct {
	identity                CacheIdentity
	tokens                  []int
	keys, values            []Vec
	Projections, ScorePairs int
}

func NewCache(identity CacheIdentity) *KVCache { return &KVCache{identity: identity} }
func (c *KVCache) Step(identity CacheIdentity, prefix []int) (Vec, error) {
	if identity != c.identity || len(prefix) != len(c.tokens)+1 || len(prefix) > 64 || !slices.Equal(prefix[:len(c.tokens)], c.tokens) {
		return Vec{}, ErrCache
	}
	p, err := project(prefix[len(prefix)-1], len(c.tokens))
	if err != nil {
		return Vec{}, err
	}
	// Previous K/V remain unchanged; the new query still attends over all accepted history.
	keys := append(append([]Vec(nil), c.keys...), p.K)
	values := append(append([]Vec(nil), c.values...), p.V)
	out, _, err := Attention(p.Q, keys, values)
	if err != nil {
		return Vec{}, err
	}
	c.tokens = append([]int(nil), prefix...)
	c.keys = keys
	c.values = values
	c.Projections++
	c.ScorePairs += len(keys)
	return out, nil
}
func Distribution(logits []float64, temperature, topP float64) ([]float64, error) {
	if len(logits) == 0 || len(logits) > 256 || !finite(temperature) || temperature < 0 || temperature > 100 || !finite(topP) || topP <= 0 || topP > 1 {
		return nil, ErrInput
	}
	for _, v := range logits {
		if !finite(v) || math.Abs(v) > 1_000_000 {
			return nil, ErrInput
		}
	}
	if temperature == 0 {
		winner := 0
		for n, v := range logits {
			if v > logits[winner] {
				winner = n
			}
		}
		probs := make([]float64, len(logits))
		probs[winner] = 1
		return probs, nil
	}
	if temperature < 1e-6 {
		return nil, ErrInput
	}
	scaled := make([]float64, len(logits))
	for n, v := range logits {
		scaled[n] = v / temperature
	}
	probs, err := Softmax(scaled)
	if err != nil {
		return nil, err
	}
	order := make([]int, len(probs))
	for n := range order {
		order[n] = n
	}
	sort.SliceStable(order, func(i, j int) bool { return probs[order[i]] > probs[order[j]] })
	kept := make([]float64, len(probs))
	mass := 0.0
	for _, n := range order {
		kept[n] = probs[n]
		mass += probs[n]
		if mass >= topP {
			break
		}
	}
	for n := range kept {
		kept[n] /= mass
	}
	return kept, nil
}
func Sample(probs []float64, u float64) (int, error) {
	if len(probs) == 0 || len(probs) > 256 || !finite(u) || u < 0 || u >= 1 {
		return 0, ErrInput
	}
	total := 0.0
	last := -1
	for n, p := range probs {
		if !finite(p) || p < 0 {
			return 0, ErrInput
		}
		total += p
		if p > 0 {
			last = n
		}
	}
	if math.Abs(total-1) > 1e-9 || last < 0 {
		return 0, ErrInput
	}
	cumulative := 0.0
	for n, p := range probs {
		cumulative += p
		if u < cumulative {
			return n, nil
		}
	}
	return last, nil // Floating rounding at the final CDF boundary.
}
func main() {
	tokenizer, _ := NewTokenizer([]Merge{{Left: int('A'), Right: int('I')}})
	text := "AI学习 AI"
	ids, _ := tokenizer.Encode(text)
	decoded, _ := tokenizer.Decode(ids)
	fmt.Printf("bytes=%d runes=%d toy_tokens=%d roundtrip=%t\n", len(text), utf8.RuneCountInString(text), len(ids), decoded == text)
	identity := CacheIdentity{Weights: "fixture-v1", Tokenizer: "four-ids-v1", Position: "fixed-v1"}
	cache := NewCache(identity)
	tokens := []int{0, 1, 2, 3}
	fullProjections := 0
	equal := true
	for n := 1; n <= len(tokens); n++ {
		out, count, _ := FullCausal(tokens[:n])
		fullProjections += count
		cached, _ := cache.Step(identity, tokens[:n])
		for d := range cached {
			equal = equal && math.Abs(cached[d]-out[n-1][d]) < 1e-12
		}
	}
	fmt.Printf("cache_equivalent=%t full_projections=%d cached_projections=%d cached_score_pairs=%d\n", equal, fullProjections, cache.Projections, cache.ScorePairs)
	probs, _ := Distribution([]float64{2, 1, 0}, 1, .8)
	sample, _ := Sample(probs, .9)
	fmt.Printf("top_p_probabilities=[%.3f %.3f %.3f] fixed_draw=%d\n", probs[0], probs[1], probs[2], sample)
	ref := PairLogProb{Chosen: math.Log(.5), Rejected: math.Log(.5)}
	for _, prob := range []float64{.2, .5, .8} {
		loss, err := PreferenceLoss(PairLogProb{Chosen: math.Log(prob), Rejected: math.Log(1 - prob)}, ref, 1)
		if err != nil {
			panic(err)
		}
		fmt.Printf("preferred_probability=%.1f reference_probability=0.5 preference_loss=%.6f\n", prob, loss)
	}
}
