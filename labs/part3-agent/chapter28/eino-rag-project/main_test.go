package main

import (
	"context"
	"strings"
	"testing"
)

func TestProjectRetrieveContext(t *testing.T) {
	t.Parallel()

	app := NewProject(FakeEmbedder{Dimensions: 8}, FakeAnswerModel{})
	answer, err := app.Answer(context.Background(), "How does rag use context?")
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	if len(answer.Contexts) == 0 {
		t.Fatalf("Contexts = 0, want retrieved material")
	}
}

func TestProjectAnswerIncludesGrounding(t *testing.T) {
	t.Parallel()

	app := NewProject(FakeEmbedder{Dimensions: 8}, FakeAnswerModel{})
	answer, err := app.Answer(context.Background(), "What is eino rag?")
	if err != nil {
		t.Fatalf("Answer() error = %v", err)
	}
	for _, want := range []string{"Question:", "Retrieved context:", "Answer:"} {
		if !strings.Contains(answer.Render(), want) {
			t.Fatalf("Render() missing %q in %q", want, answer.Render())
		}
	}
}

func TestRunDemoReturnsStableGoldenShape(t *testing.T) {
	t.Parallel()

	out, err := runDemo(context.Background())
	if err != nil {
		t.Fatalf("runDemo() error = %v", err)
	}
	if !strings.Contains(out, "[labs/part3-agent/chapter28/eino-rag-project]") {
		t.Fatalf("runDemo() = %q, want project source label", out)
	}
}
