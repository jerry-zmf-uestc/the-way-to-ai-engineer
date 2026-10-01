package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildPromptMessages(t *testing.T) {
	t.Parallel()

	messages := buildPromptMessages("retrieval", "backend engineer")
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if got := messages[1].Content; !strings.Contains(got, "retrieval") {
		t.Fatalf("user message = %q, want topic included", got)
	}
}

func TestFakeChatModelGenerate(t *testing.T) {
	t.Parallel()

	model := FakeChatModel{}
	reply, err := model.Generate(context.Background(), buildPromptMessages("rag", "reader"))
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(reply.Content, "rag") {
		t.Fatalf("reply = %q, want topic echoed", reply.Content)
	}
}

func TestRunDemoUsesContextLabel(t *testing.T) {
	t.Parallel()

	out, err := runDemo(context.Background(), DemoInput{
		Topic:    "vector store",
		Audience: "new learner",
	})
	if err != nil {
		t.Fatalf("runDemo() error = %v", err)
	}
	for _, want := range []string{"topic=vector store", "audience=new learner"} {
		if !strings.Contains(out, want) {
			t.Fatalf("runDemo() missing %q in %q", want, out)
		}
	}
}

func TestRunDemoRejectsMissingFields(t *testing.T) {
	t.Parallel()

	_, err := runDemo(context.Background(), DemoInput{Topic: "rag"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("runDemo() error = %v, want %v", err, ErrInvalidInput)
	}
}
