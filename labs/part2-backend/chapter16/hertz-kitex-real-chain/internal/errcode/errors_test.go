package errcode

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []error{ErrInvalidInput, ErrNotFound, ErrUpstream}
	for _, err := range tests {
		if err == nil {
			t.Fatal("sentinel error is nil")
		}
		if !errors.Is(err, err) {
			t.Fatalf("errors.Is(%v, itself) = false", err)
		}
	}
}
