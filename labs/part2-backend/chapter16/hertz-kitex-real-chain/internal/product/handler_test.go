package product

import (
	"context"
	"errors"
	"testing"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce"
)

func TestProductServiceImpl_GetProduct(t *testing.T) {
	t.Parallel()

	svc := NewHandler()
	product, err := svc.GetProduct(context.Background(), &commerce.ProductRequest{ProductId: "p-100"})
	if err != nil {
		t.Fatalf("GetProduct() error = %v", err)
	}
	if product.GetId() != "p-100" || product.GetName() != "Go Mug" || product.GetPriceCents() != 1299 {
		t.Fatalf("product = %+v", product)
	}
}

func TestProductServiceImpl_GetProductErrors(t *testing.T) {
	t.Parallel()

	svc := NewHandler()
	tests := []struct {
		name string
		req  *commerce.ProductRequest
		want error
	}{
		{name: "nil request", req: nil, want: errcode.ErrInvalidInput},
		{name: "empty id", req: &commerce.ProductRequest{}, want: errcode.ErrInvalidInput},
		{name: "missing", req: &commerce.ProductRequest{ProductId: "missing"}, want: errcode.ErrNotFound},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.GetProduct(context.Background(), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("GetProduct() error = %v, want errors.Is(%v)", err, tt.want)
			}
		})
	}
}

func TestProductServiceImpl_Ping(t *testing.T) {
	t.Parallel()

	got, err := NewHandler().Ping(context.Background(), &commerce.PingRequest{})
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if got.GetOk() != "product:ok" {
		t.Fatalf("Ping().ok = %q, want product:ok", got.GetOk())
	}
}
