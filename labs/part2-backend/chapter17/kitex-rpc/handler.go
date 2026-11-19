package main

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter17/kitex-rpc/kitex_gen/api"
)

// ErrNilRequest reports a nil Kitex request pointer.
var ErrNilRequest = errors.New("request is nil")

// ErrEmptyMessage reports an Echo request with no useful message content.
var ErrEmptyMessage = errors.New("message is required")

// EchoServiceImpl implements the generated Kitex EchoService interface.
type EchoServiceImpl struct{}

// Echo trims a message and returns its UTF-8 rune length.
func (EchoServiceImpl) Echo(ctx context.Context, req *api.EchoRequest) (*api.EchoResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrNilRequest
	}

	message := strings.TrimSpace(req.GetMessage())
	if message == "" {
		return nil, ErrEmptyMessage
	}

	return &api.EchoResponse{
		Message: message,
		Length:  int32(utf8.RuneCountInString(message)),
	}, nil
}

// Add returns the sum of two integer inputs.
func (EchoServiceImpl) Add(ctx context.Context, req *api.AddRequest) (*api.AddResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrNilRequest
	}

	return &api.AddResponse{Sum: req.GetFirst() + req.GetSecond()}, nil
}
