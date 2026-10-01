package product_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/internal/product"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce"
	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter16/hertz-kitex-real-chain/kitex_gen/commerce/productservice"
)

func TestProductServer_RealKitex(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	productServer, err := product.Start(ctx, logger)
	if err != nil {
		t.Fatalf("start product: %v", err)
	}
	t.Cleanup(func() { _ = productServer.Shutdown(context.Background()) })

	c, err := productservice.NewClient("product", client.WithHostPorts(productServer.Addr()))
	if err != nil {
		t.Fatalf("new product client: %v", err)
	}
	pong, err := c.Ping(ctx, &commerce.PingRequest{})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if pong.GetOk() != "product:ok" {
		t.Fatalf("Ping().ok = %q, want product:ok", pong.GetOk())
	}
	got, err := c.GetProduct(ctx, &commerce.ProductRequest{ProductId: "p-200"})
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got.GetName() != "Kitex Sticker" || got.GetPriceCents() != 599 {
		t.Fatalf("product = %+v", got)
	}
}
