package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrInvalid = errors.New("retrieval: invalid input")

type Chunk struct {
	ID, Source, Version, Tenant, Text, Space string
	Vector                                   []float64
}
type Hit struct {
	Chunk                   Chunk
	Score                   float64
	LexicalRank, VectorRank int
}
type Index struct {
	Space      string
	Dimensions int
	Chunks     []Chunk
}

// Split保留文档版本与分段顺序；教学实现按码点切分，不假称语义切分。
func Split(source, version, tenant, text string, maxRunes int) ([]Chunk, error) {
	if source == "" || version == "" || tenant == "" || maxRunes <= 0 || len(text) > 64<<10 || !utf8.ValidString(text) {
		return nil, ErrInvalid
	}
	out := []Chunk{}
	for _, paragraph := range strings.Split(text, "\n\n") {
		runes := []rune(strings.TrimSpace(paragraph))
		for len(runes) > 0 {
			n := min(maxRunes, len(runes))
			id := fmt.Sprintf("%s@%s:%d", url.QueryEscape(source), url.QueryEscape(version), len(out))
			out = append(out, Chunk{ID: id, Source: source, Version: version, Tenant: tenant, Text: string(runes[:n])})
			runes = runes[n:]
			if len(out) > 128 {
				return nil, ErrInvalid
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrInvalid
	}
	return out, nil
}
func cosine(a, b []float64) (float64, error) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, ErrInvalid
	}
	dot, aa, bb := 0.0, 0.0, 0.0
	for i, v := range a {
		w := b[i]
		if math.IsNaN(v) || math.IsNaN(w) || math.IsInf(v, 0) || math.IsInf(w, 0) || math.Abs(v) > 1e6 || math.Abs(w) > 1e6 {
			return 0, ErrInvalid
		}
		dot += v * w
		aa += v * v
		bb += w * w
	}
	if aa == 0 || bb == 0 {
		return 0, ErrInvalid
	}
	return dot / (math.Sqrt(aa) * math.Sqrt(bb)), nil
}

// NewIndex在验证整批后才返回索引，向量与原始输入不共享。
func NewIndex(space string, dimensions int, chunks []Chunk) (Index, error) {
	if space == "" || dimensions <= 0 || dimensions > 4096 || len(chunks) == 0 || len(chunks) > 128 {
		return Index{}, ErrInvalid
	}
	index := Index{Space: space, Dimensions: dimensions}
	seen := map[string]bool{}
	bytes := 0
	for _, c := range chunks {
		bytes += len(c.ID) + len(c.Text) + len(c.Source) + len(c.Version) + len(c.Tenant)
		key := c.Tenant + "\x00" + c.ID
		if bytes > 64<<10 || c.ID == "" || c.Source == "" || c.Version == "" || c.Tenant == "" || c.Text == "" || strings.Contains(c.Tenant, "\x00") || strings.Contains(c.ID, "\x00") || seen[key] || c.Space != space || len(c.Vector) != dimensions {
			return Index{}, ErrInvalid
		}
		if _, err := cosine(c.Vector, c.Vector); err != nil {
			return Index{}, err
		}
		seen[key] = true
		c.Vector = append([]float64(nil), c.Vector...)
		index.Chunks = append(index.Chunks, c)
	}
	return index, nil
}
func rank(hits []Hit) []Hit {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Chunk.ID < hits[j].Chunk.ID
		}
		return hits[i].Score > hits[j].Score
	})
	return hits
}
func lexical(query, text string) float64 {
	tokens := map[string]bool{}
	for _, word := range strings.Fields(strings.ToLower(query)) {
		tokens[word] = true
	}
	words := map[string]bool{}
	for _, word := range strings.Fields(strings.ToLower(text)) {
		words[word] = true
	}
	count := 0
	for word := range tokens {
		if words[word] {
			count++
		}
	}
	return float64(count)
}

// Search先按可信tenant过滤，两路各取candidateK，最终topK独立限制。
func (i Index) Search(ctx context.Context, tenant, query, space string, vector []float64, candidateK, topK int) ([]Hit, error) {
	if tenant == "" || strings.TrimSpace(query) == "" || len(query) > 4096 || space != i.Space || len(vector) != i.Dimensions || candidateK <= 0 || candidateK > 128 || topK <= 0 || topK > 128 {
		return nil, ErrInvalid
	}
	if _, err := cosine(vector, vector); err != nil {
		return nil, err
	}
	lex, vec := []Hit{}, []Hit{}
	for _, c := range i.Chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.Tenant != tenant {
			continue
		}
		// 返回数据复制，调用方不能修改索引向量。
		c.Vector = append([]float64(nil), c.Vector...)
		if score := lexical(query, c.Text); score > 0 {
			lex = append(lex, Hit{Chunk: c, Score: score})
		}
		score, err := cosine(vector, c.Vector)
		if err != nil {
			return nil, err
		}
		if score > 0 {
			vec = append(vec, Hit{Chunk: c, Score: score})
		}
	}
	rank(lex)
	rank(vec)
	lex = lex[:min(candidateK, len(lex))]
	vec = vec[:min(candidateK, len(vec))]
	fused := Fuse(lex, vec, 60)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return fused[:min(topK, len(fused))], nil
}

// Fuse用排名融合，不直接相加不同尺度的原始相似度。
func Fuse(lex, vec []Hit, k float64) []Hit {
	combined := map[string]Hit{}
	for route, list := range [][]Hit{lex, vec} {
		seen := map[string]bool{}
		for n, item := range list {
			id := item.Chunk.ID
			if seen[id] {
				continue
			}
			seen[id] = true
			hit := combined[id]
			hit.Chunk = item.Chunk
			hit.Score += 1 / (k + float64(n+1))
			if route == 0 {
				hit.LexicalRank = n + 1
			} else {
				hit.VectorRank = n + 1
			}
			combined[id] = hit
		}
	}
	out := make([]Hit, 0, len(combined))
	for _, hit := range combined {
		out = append(out, hit)
	}
	return rank(out)
}

// Rerank只对已有候选评分，不接收新ID；非有限分数与服务失败都拒绝。
func Rerank(ctx context.Context, hits []Hit, score func(context.Context, Chunk) (float64, error)) ([]Hit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if score == nil || len(hits) > 128 {
		return nil, ErrInvalid
	}
	out := append([]Hit(nil), hits...)
	for n, hit := range out {
		out[n].Chunk.Vector = append([]float64(nil), hit.Chunk.Vector...)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		copy := hit.Chunk
		copy.Vector = append([]float64(nil), copy.Vector...)
		value, err := score(ctx, copy)
		if err != nil {
			return nil, err
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, ErrInvalid
		}
		out[n].Score = value
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return rank(out), nil
}
func main() {
	metrics := flag.Bool("ranking", false, "compare retrieval ranking metrics")
	flag.Parse()
	if *metrics {
		rankingDemo()
		return
	}
	chunks, _ := Split("guide", "v1", "demo", "timeout E42 documented\n\nrequest deadline reference", 64)
	// 手工给定教学向量，只验证排序管道，不宣称学得语义。
	for n := range chunks {
		chunks[n].Space = "teaching-v1"
		chunks[n].Vector = [][]float64{{0.8, 0.2}, {1, 0}}[n]
	}
	index, err := NewIndex("teaching-v1", 2, chunks)
	if err != nil {
		panic(err)
	}
	hits, err := index.Search(context.Background(), "demo", "E42", "teaching-v1", []float64{1, 0}, 2, 2)
	if err != nil {
		panic(err)
	}
	for _, hit := range hits {
		fmt.Printf("id=%s lexical_rank=%d vector_rank=%d fused=%.5f\n", hit.Chunk.ID, hit.LexicalRank, hit.VectorRank, hit.Score)
	}
}
