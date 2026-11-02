package user

import (
	"context"
	"fmt"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce"
)

// OrderClient is the OrderService subset used by UserService.
type OrderClient interface {
	ListOrders(context.Context, *commerce.ListOrdersRequest, ...callopt.Option) (*commerce.ListOrdersResponse, error)
}

// UserServiceImpl serves UserService from in-memory fixtures.
type UserServiceImpl struct {
	order OrderClient
	users map[string]*commerce.User
}

// NewHandler constructs the UserService handler.
func NewHandler(order OrderClient) *UserServiceImpl {
	return &UserServiceImpl{
		order: order,
		users: map[string]*commerce.User{
			"u-100": {Id: "u-100", Name: "Ada"},
		},
	}
}

// GetUserOrders returns one user and that user's hydrated orders.
func (h *UserServiceImpl) GetUserOrders(ctx context.Context, req *commerce.UserOrdersRequest) (*commerce.UserOrdersResponse, error) {
	if req == nil || req.GetUserId() == "" {
		return nil, fmt.Errorf("%w: user_id required", errcode.ErrInvalidInput)
	}
	found, ok := h.users[req.GetUserId()]
	if !ok {
		return nil, fmt.Errorf("%w: user %s", errcode.ErrNotFound, req.GetUserId())
	}
	if h.order == nil {
		return nil, fmt.Errorf("%w: order client missing", errcode.ErrUpstream)
	}
	orders, err := h.order.ListOrders(ctx, &commerce.ListOrdersRequest{UserId: req.GetUserId()})
	if err != nil {
		return nil, fmt.Errorf("%w: list orders: %w", errcode.ErrUpstream, err)
	}
	return &commerce.UserOrdersResponse{
		User:   &commerce.User{Id: found.GetId(), Name: found.GetName()},
		Orders: orders.GetOrders(),
	}, nil
}

// Ping returns a bounded liveness response.
func (h *UserServiceImpl) Ping(ctx context.Context, req *commerce.PingRequest) (*commerce.PingResponse, error) {
	return &commerce.PingResponse{Ok: "user:ok"}, nil
}
