package order

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce"
)

func TestOrderServiceImpl_ListOrders(t *testing.T) {
	t.Parallel()

	svc := NewHandler(fakeProductClient{})
	resp, err := svc.ListOrders(context.Background(), &commerce.ListOrdersRequest{UserId: "u-100"})
	if err != nil {
		t.Fatalf("ListOrders() error = %v", err)
	}
	if len(resp.GetOrders()) != 2 {
		t.Fatalf("orders len = %d, want 2", len(resp.GetOrders()))
	}
	if got := total(resp.GetOrders()); got != 2497 {
		t.Fatalf("total = %d, want 2497", got)
	}
	if resp.GetOrders()[0].GetItems()[0].GetProductName() != "Go Mug" {
		t.Fatalf("first product name = %q", resp.GetOrders()[0].GetItems()[0].GetProductName())
	}
}

func TestOrderServiceImpl_ListOrdersErrors(t *testing.T) {
	t.Parallel()

	svc := NewHandler(fakeProductClient{})
	tests := []struct {
		name string
		req  *commerce.ListOrdersRequest
		want error
	}{
		{name: "nil request", req: nil, want: errcode.ErrInvalidInput},
		{name: "empty user", req: &commerce.ListOrdersRequest{}, want: errcode.ErrInvalidInput},
		{name: "missing user", req: &commerce.ListOrdersRequest{UserId: "missing"}, want: errcode.ErrNotFound},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.ListOrders(context.Background(), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ListOrders() error = %v, want errors.Is(%v)", err, tt.want)
			}
		})
	}
}

func TestOrderServiceImpl_UpstreamError(t *testing.T) {
	t.Parallel()

	svc := NewHandler(fakeProductClient{err: errcode.ErrUpstream})
	_, err := svc.ListOrders(context.Background(), &commerce.ListOrdersRequest{UserId: "u-100"})
	if !errors.Is(err, errcode.ErrUpstream) {
		t.Fatalf("ListOrders() error = %v, want upstream", err)
	}
}

func TestOrderServiceImpl_Ping(t *testing.T) {
	t.Parallel()

	got, err := NewHandler(fakeProductClient{}).Ping(context.Background(), &commerce.PingRequest{})
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if got.GetOk() != "order:ok" {
		t.Fatalf("Ping().ok = %q, want order:ok", got.GetOk())
	}
}

type fakeProductClient struct {
	err error
}

func (c fakeProductClient) GetProduct(ctx context.Context, req *commerce.ProductRequest, opts ...callopt.Option) (*commerce.Product, error) {
	if c.err != nil {
		return nil, c.err
	}
	switch req.GetProductId() {
	case "p-100":
		return &commerce.Product{Id: "p-100", Name: "Go Mug", PriceCents: 1299}, nil
	case "p-200":
		return &commerce.Product{Id: "p-200", Name: "Kitex Sticker", PriceCents: 599}, nil
	default:
		return nil, errcode.ErrNotFound
	}
}

func total(orders []*commerce.Order) int64 {
	var sum int64
	for _, order := range orders {
		sum += order.GetTotalCents()
	}
	return sum
}
