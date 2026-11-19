package user

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce"
)

func TestUserServiceImpl_GetUserOrders(t *testing.T) {
	t.Parallel()

	svc := NewHandler(fakeOrderClient{})
	resp, err := svc.GetUserOrders(context.Background(), &commerce.UserOrdersRequest{UserId: "u-100"})
	if err != nil {
		t.Fatalf("GetUserOrders() error = %v", err)
	}
	if resp.GetUser().GetName() != "Ada" {
		t.Fatalf("user name = %q, want Ada", resp.GetUser().GetName())
	}
	if len(resp.GetOrders()) != 2 {
		t.Fatalf("orders len = %d, want 2", len(resp.GetOrders()))
	}
}

func TestUserServiceImpl_GetUserOrdersErrors(t *testing.T) {
	t.Parallel()

	svc := NewHandler(fakeOrderClient{})
	tests := []struct {
		name string
		req  *commerce.UserOrdersRequest
		want error
	}{
		{name: "nil request", req: nil, want: errcode.ErrInvalidInput},
		{name: "empty user", req: &commerce.UserOrdersRequest{}, want: errcode.ErrInvalidInput},
		{name: "missing user", req: &commerce.UserOrdersRequest{UserId: "missing"}, want: errcode.ErrNotFound},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.GetUserOrders(context.Background(), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("GetUserOrders() error = %v, want errors.Is(%v)", err, tt.want)
			}
		})
	}
}

func TestUserServiceImpl_UpstreamError(t *testing.T) {
	t.Parallel()

	svc := NewHandler(fakeOrderClient{err: errcode.ErrUpstream})
	_, err := svc.GetUserOrders(context.Background(), &commerce.UserOrdersRequest{UserId: "u-100"})
	if !errors.Is(err, errcode.ErrUpstream) {
		t.Fatalf("GetUserOrders() error = %v, want upstream", err)
	}
}

func TestUserServiceImpl_Ping(t *testing.T) {
	t.Parallel()

	got, err := NewHandler(fakeOrderClient{}).Ping(context.Background(), &commerce.PingRequest{})
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if got.GetOk() != "user:ok" {
		t.Fatalf("Ping().ok = %q, want user:ok", got.GetOk())
	}
}

type fakeOrderClient struct {
	err error
}

func (c fakeOrderClient) ListOrders(ctx context.Context, req *commerce.ListOrdersRequest, opts ...callopt.Option) (*commerce.ListOrdersResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	if req.GetUserId() == "missing" {
		return nil, errcode.ErrNotFound
	}
	return &commerce.ListOrdersResponse{
		Orders: []*commerce.Order{
			{Id: "o-100", UserId: req.GetUserId(), TotalCents: 1898},
			{Id: "o-200", UserId: req.GetUserId(), TotalCents: 599},
		},
	}, nil
}
