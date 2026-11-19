package user_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/config"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/kitex_gen/commerce/userservice"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/order"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/product"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-demo/internal/hertzkitexdemo/user"
)

func TestChain_ProductToOrderToUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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

	userServer, err := user.Start(ctx, logger, cfg, orderServer.Addr())
	if err != nil {
		t.Fatalf("start user: %v", err)
	}
	t.Cleanup(func() { _ = userServer.Shutdown(context.Background()) })

	c, err := userservice.NewClient("user",
		client.WithHostPorts(userServer.Addr()),
		client.WithRPCTimeout(cfg.RPCTimeout),
	)
	if err != nil {
		t.Fatalf("new user client: %v", err)
	}
	resp, err := c.GetUserOrders(ctx, &commerce.UserOrdersRequest{UserId: "u-100"})
	if err != nil {
		t.Fatalf("GetUserOrders: %v", err)
	}
	if got := resp.GetUser().GetName(); got != "Ada" {
		t.Fatalf("user name = %q, want Ada", got)
	}
	if got := len(resp.GetOrders()); got != 2 {
		t.Fatalf("orders len = %d, want 2", got)
	}
	if got := total(resp.GetOrders()); got != 2497 {
		t.Fatalf("total = %d, want 2497", got)
	}
}

func total(orders []*commerce.Order) int64 {
	var sum int64
	for _, order := range orders {
		sum += order.GetTotalCents()
	}
	return sum
}
