package product

import (
	"context"
	"fmt"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/errcode"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce"
)

// ProductServiceImpl serves ProductService from in-memory fixtures.
type ProductServiceImpl struct {
	products map[string]*commerce.Product
}

// NewHandler constructs the ProductService handler.
func NewHandler() *ProductServiceImpl {
	return &ProductServiceImpl{
		products: map[string]*commerce.Product{
			"p-100": {Id: "p-100", Name: "Go Mug", PriceCents: 1299},
			"p-200": {Id: "p-200", Name: "Kitex Sticker", PriceCents: 599},
			"p-300": {Id: "p-300", Name: "Hertz Notebook", PriceCents: 1899},
		},
	}
}

// GetProduct returns one product fixture.
func (h *ProductServiceImpl) GetProduct(ctx context.Context, req *commerce.ProductRequest) (*commerce.Product, error) {
	if req == nil || req.GetProductId() == "" {
		return nil, fmt.Errorf("%w: product_id required", errcode.ErrInvalidInput)
	}
	product, ok := h.products[req.GetProductId()]
	if !ok {
		return nil, fmt.Errorf("%w: product %s", errcode.ErrNotFound, req.GetProductId())
	}
	return &commerce.Product{
		Id:         product.GetId(),
		Name:       product.GetName(),
		PriceCents: product.GetPriceCents(),
	}, nil
}

// Ping returns a bounded liveness response.
func (h *ProductServiceImpl) Ping(ctx context.Context, req *commerce.PingRequest) (*commerce.PingResponse, error) {
	return &commerce.PingResponse{Ok: "product:ok"}, nil
}
