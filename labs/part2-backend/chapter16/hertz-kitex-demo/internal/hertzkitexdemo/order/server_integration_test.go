package order_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce/orderservice"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/order"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/product"
)

func TestChain_ProductToOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()

	productServer, err := product.Start(ctx, logger)
	if err != nil {
		t.Fatalf("start product: %v", err)
	}
	t.Cleanup(func() { _ = productServer.Shutdown(context.Background()) })

	orderServer, err := order.Start(ctx, logger, cfg, productServer.Addr())
	if err != nil {
		t.Fatalf("start order: %v", err)
	}
	t.Cleanup(func() { _ = orderServer.Shutdown(context.Background()) })

	orderClient, err := orderservice.NewClient("order",
		client.WithHostPorts(orderServer.Addr()),
		client.WithRPCTimeout(cfg.RPCTimeout),
	)
	if err != nil {
		t.Fatalf("new order client: %v", err)
	}
	resp, err := orderClient.ListOrders(ctx, &commerce.ListOrdersRequest{UserId: "u-100"})
	if err != nil {
		t.Fatalf("ListOrders() error = %v", err)
	}
	if len(resp.GetOrders()) != 2 {
		t.Fatalf("orders len = %d, want 2", len(resp.GetOrders()))
	}
	var total int64
	for _, order := range resp.GetOrders() {
		total += order.GetTotalCents()
	}
	if total != 2497 {
		t.Fatalf("total = %d, want 2497", total)
	}
}
