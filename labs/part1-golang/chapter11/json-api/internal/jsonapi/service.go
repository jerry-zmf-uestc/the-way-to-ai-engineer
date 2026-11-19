package jsonapi

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrEmptyMessage reports an echo request that contains no useful content.
var ErrEmptyMessage = errors.New("message is required")

// EchoRequest is the JSON payload accepted by POST /api/echo.
type EchoRequest struct {
	Message string `json:"message"`
}

// EchoResponse is the stable JSON payload returned by POST /api/echo.
type EchoResponse struct {
	Echo   string `json:"echo"`
	Length int    `json:"length"`
}

// EchoService owns the private learning logic behind the HTTP endpoint.
type EchoService struct{}

// NewEchoService constructs the service without exposing mutable state.
func NewEchoService() EchoService {
	return EchoService{}
}

// Echo validates and normalizes the message before building the response.
func (EchoService) Echo(message string) (EchoResponse, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return EchoResponse{}, ErrEmptyMessage
	}

	return EchoResponse{
		Echo:   message,
		Length: utf8.RuneCountInString(message),
	}, nil
}
