package errcode

import "errors"

var (
	// ErrInvalidInput marks malformed or empty request input.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound marks missing fixture data.
	ErrNotFound = errors.New("not found")
	// ErrUpstream marks a downstream RPC failure.
	ErrUpstream = errors.New("upstream error")
)
