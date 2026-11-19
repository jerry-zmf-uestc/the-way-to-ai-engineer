package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	embedding "github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var ErrNoContext = errors.New("eino rag project: no context found")

type RetrievedContext struct {
	Source string
	Text   string
}

type ProjectAnswer struct {
	Question string
	Contexts []RetrievedContext
	Text     string
}

func (a ProjectAnswer) Render() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Question: %s\n", a.Question)
	builder.WriteString("Retrieved context:\n")
	for i, ctx := range a.Contexts {
		fmt.Fprintf(&builder, "%d. [%s] %s\n", i+1, ctx.Source, ctx.Text)
	}
	fmt.Fprintf(&builder, "Answer: %s", a.Text)
	return builder.String()
}

type FakeEmbedder struct {
	Dimensions int
}

var _ embedding.Embedder = FakeEmbedder{}

func (f FakeEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	dims := f.Dimensions
	if dims <= 0 {
		dims = 8
	}

	out := make([][]float64, 0, len(texts))
	for _, text := range texts {
		vector := make([]float64, dims)
		for _, token := range strings.Fields(normalizeText(text)) {
			vector[stableBucket(token, dims)]++
		}
		out = append(out, unit(vector))
	}
	return out, nil
}

type FakeAnswerModel struct{}

var _ model.BaseChatModel = FakeAnswerModel{}

func (FakeAnswerModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if len(input) == 0 {
		return nil, errors.New("eino rag project: empty prompt")
	}
	return schema.AssistantMessage("Use the retrieved context above as the grounding material.", nil), nil
}

func (m FakeAnswerModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reply, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}

	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Send(reply, nil)
	writer.Close()
	return reader, nil
}

type knowledgeDoc struct {
	ID     string
	Source string
	Text   string
	Vector []float64
}

type Project struct {
	embedder FakeEmbedder
	model    FakeAnswerModel
	docs     []knowledgeDoc
}

func NewProject(embedder FakeEmbedder, model FakeAnswerModel) *Project {
	return &Project{
		embedder: embedder,
		model:    model,
		docs: []knowledgeDoc{
			{
				ID:     "embedding",
				Source: "labs/part3-agent/chapter28/eino-vector-store",
				Text:   "Eino embedding adapters define batch input, context propagation, and component boundaries.",
			},
			{
				ID:     "vector",
				Source: "labs/part3-agent/chapter28/eino-vector-store",
				Text:   "Eino embedding plus a vector store retrieves the most relevant context for a query.",
			},
			{
				ID:     "project",
				Source: "labs/part3-agent/chapter28/eino-rag-project",
				Text:   "A RAG project assembles retrieved context before generating a grounded answer.",
			},
		},
	}
}

func (p *Project) Answer(ctx context.Context, query string) (ProjectAnswer, error) {
	texts := make([]string, 0, len(p.docs))
	for _, doc := range p.docs {
		texts = append(texts, doc.Text)
	}

	vectors, err := p.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return ProjectAnswer{}, err
	}
	indexed := make([]knowledgeDoc, len(p.docs))
	for i, doc := range p.docs {
		indexed[i] = doc
		indexed[i].Vector = vectors[i]
	}

	queryVector, err := p.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return ProjectAnswer{}, err
	}

	contexts, err := retrieveTopContexts(queryVector[0], indexed, 2)
	if err != nil {
		return ProjectAnswer{}, err
	}

	reply, err := p.model.Generate(ctx, buildAnswerMessages(query, contexts))
	if err != nil {
		return ProjectAnswer{}, err
	}

	return ProjectAnswer{
		Question: query,
		Contexts: contexts,
		Text:     reply.Content,
	}, nil
}

func buildAnswerMessages(query string, contexts []RetrievedContext) []*schema.Message {
	var builder strings.Builder
	fmt.Fprintf(&builder, "question=%s\n", query)
	for i, ctx := range contexts {
		fmt.Fprintf(&builder, "context_%d=[%s] %s\n", i+1, ctx.Source, ctx.Text)
	}

	return []*schema.Message{
		schema.SystemMessage("Answer only from retrieved context."),
		schema.UserMessage(builder.String()),
	}
}

func retrieveTopContexts(query []float64, docs []knowledgeDoc, topK int) ([]RetrievedContext, error) {
	type scored struct {
		doc   knowledgeDoc
		score float64
	}

	scoredDocs := make([]scored, 0, len(docs))
	for _, doc := range docs {
		score, err := cosine(query, doc.Vector)
		if err != nil {
			return nil, err
		}
		if score <= 0 {
			continue
		}
		scoredDocs = append(scoredDocs, scored{doc: doc, score: score})
	}
	if len(scoredDocs) == 0 {
		return nil, ErrNoContext
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		if scoredDocs[i].score == scoredDocs[j].score {
			return scoredDocs[i].doc.ID < scoredDocs[j].doc.ID
		}
		return scoredDocs[i].score > scoredDocs[j].score
	})

	if len(scoredDocs) > topK {
		scoredDocs = scoredDocs[:topK]
	}

	contexts := make([]RetrievedContext, 0, len(scoredDocs))
	for _, item := range scoredDocs {
		contexts = append(contexts, RetrievedContext{Source: item.doc.Source, Text: item.doc.Text})
	}
	return contexts, nil
}

func normalizeText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

func stableBucket(token string, dimensions int) int {
	var sum int
	for _, r := range token {
		sum += int(r)
	}
	return sum % dimensions
}

func unit(vector []float64) []float64 {
	var squared float64
	for _, v := range vector {
		squared += v * v
	}
	if squared == 0 {
		return vector
	}

	length := math.Sqrt(squared)
	out := make([]float64, len(vector))
	for i, v := range vector {
		out[i] = v / length
	}
	return out
}

func cosine(a, b []float64) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, errors.New("eino rag project: empty vector")
	}
	if len(a) != len(b) {
		return 0, errors.New("eino rag project: vector dimensions differ")
	}

	var dot, lenA, lenB float64
	for i := range a {
		dot += a[i] * b[i]
		lenA += a[i] * a[i]
		lenB += b[i] * b[i]
	}
	if lenA == 0 || lenB == 0 {
		return 0, errors.New("eino rag project: zero vector")
	}
	return dot / (math.Sqrt(lenA) * math.Sqrt(lenB)), nil
}

func runDemo(ctx context.Context) (string, error) {
	app := NewProject(FakeEmbedder{Dimensions: 8}, FakeAnswerModel{})
	answer, err := app.Answer(ctx, "How does rag use context?")
	if err != nil {
		return "", err
	}
	return answer.Render(), nil
}

func main() {
	out, err := runDemo(context.Background())
	if err != nil {
		fmt.Println("run demo:", err)
		return
	}

	fmt.Println(out)
}
