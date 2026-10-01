package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var ErrInvalidInput = errors.New("eino basics: topic and audience are required")

type DemoInput struct {
	Topic    string
	Audience string
}

type FakeChatModel struct{}

var _ model.BaseChatModel = FakeChatModel{}

func buildPromptMessages(topic, audience string) []*schema.Message {
	return []*schema.Message{
		schema.SystemMessage("You explain Eino component boundaries with concise Go examples."),
		schema.UserMessage(fmt.Sprintf("topic=%s audience=%s", topic, audience)),
	}
}

func (FakeChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if len(input) == 0 {
		return nil, errors.New("eino basics: empty prompt")
	}

	return schema.AssistantMessage("Eino basics demo: "+input[len(input)-1].Content, nil), nil
}

func (m FakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reply, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}

	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Send(reply, nil)
	writer.Close()
	return reader, nil
}

func runDemo(ctx context.Context, input DemoInput) (string, error) {
	if strings.TrimSpace(input.Topic) == "" || strings.TrimSpace(input.Audience) == "" {
		return "", ErrInvalidInput
	}

	reply, err := (FakeChatModel{}).Generate(ctx, buildPromptMessages(input.Topic, input.Audience))
	if err != nil {
		return "", err
	}
	return reply.Content, nil
}

func main() {
	out, err := runDemo(context.Background(), DemoInput{
		Topic:    "retrieval pipeline",
		Audience: "Go reader",
	})
	if err != nil {
		fmt.Println("run demo:", err)
		return
	}

	fmt.Println(out)
}
