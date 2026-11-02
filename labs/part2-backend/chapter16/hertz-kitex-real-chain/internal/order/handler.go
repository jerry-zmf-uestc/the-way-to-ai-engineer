package order

import (
	"context"
	"fmt"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce"
)

// ProductClient is the ProductService subset used by OrderService.
type ProductClient interface {
	GetProduct(context.Context, *commerce.ProductRequest, ...callopt.Option) (*commerce.Product, error)
}

// OrderServiceImpl serves OrderService from in-memory fixtures.
type OrderServiceImpl struct {
	product ProductClient
	items   map[string][]orderSeed
}

type orderSeed struct {
	orderID   string
	productID string
	quantity  int64
}

// NewHandler constructs the OrderService handler.
func NewHandler(product ProductClient) *OrderServiceImpl {
	return &OrderServiceImpl{
		product: product,
		items: map[string][]orderSeed{
			"u-100": {
				{orderID: "o-100", productID: "p-100", quantity: 1},
				{orderID: "o-100", productID: "p-200", quantity: 1},
				{orderID: "o-200", productID: "p-200", quantity: 1},
			},
		},
	}
}

// ListOrders returns hydrated orders for one user.
func (h *OrderServiceImpl) ListOrders(ctx context.Context, req *commerce.ListOrdersRequest) (*commerce.ListOrdersResponse, error) {
	if req == nil || req.GetUserId() == "" {
		return nil, fmt.Errorf("%w: user_id required", errcode.ErrInvalidInput)
	}
	seeds, ok := h.items[req.GetUserId()]
	if !ok {
		return nil, fmt.Errorf("%w: orders for user %s", errcode.ErrNotFound, req.GetUserId())
	}
	if h.product == nil {
		return nil, fmt.Errorf("%w: product client missing", errcode.ErrUpstream)
	}

	byID := make(map[string]*commerce.Order)
	orderIDs := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		product, err := h.product.GetProduct(ctx, &commerce.ProductRequest{ProductId: seed.productID})
		if err != nil {
			return nil, fmt.Errorf("%w: product %s: %w", errcode.ErrUpstream, seed.productID, err)
		}
		order, ok := byID[seed.orderID]
		if !ok {
			order = &commerce.Order{Id: seed.orderID, UserId: req.GetUserId()}
			byID[seed.orderID] = order
			orderIDs = append(orderIDs, seed.orderID)
		}
		lineTotal := product.GetPriceCents() * seed.quantity
		order.Items = append(order.Items, &commerce.OrderItem{
			ProductId:      product.GetId(),
			ProductName:    product.GetName(),
			UnitPriceCents: product.GetPriceCents(),
			Quantity:       seed.quantity,
		})
		order.TotalCents += lineTotal
	}

	orders := make([]*commerce.Order, 0, len(orderIDs))
	for _, id := range orderIDs {
		orders = append(orders, byID[id])
	}
	return &commerce.ListOrdersResponse{Orders: orders}, nil
}

// Ping returns a bounded liveness response.
func (h *OrderServiceImpl) Ping(ctx context.Context, req *commerce.PingRequest) (*commerce.PingResponse, error) {
	return &commerce.PingResponse{Ok: "order:ok"}, nil
}
