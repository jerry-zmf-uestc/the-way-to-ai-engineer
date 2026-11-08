package gateway

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce"
)

func TestStatusFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid input", err: errcode.ErrInvalidInput, want: http.StatusBadRequest},
		{name: "not found", err: errcode.ErrNotFound, want: http.StatusNotFound},
		{name: "upstream", err: errcode.ErrUpstream, want: http.StatusBadGateway},
		{name: "unknown", err: errors.New("boom"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := statusFromError(tt.err); got != tt.want {
				t.Fatalf("statusFromError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestTotal(t *testing.T) {
	t.Parallel()

	orders := []*commerce.Order{
		{TotalCents: 1898},
		{TotalCents: 599},
	}
	if got := total(orders); got != 2497 {
		t.Fatalf("total() = %d, want 2497", got)
	}
}
