package main

import (
	"context"
	"errors"
	"testing"
)

func newTestStore() *InMemoryStore {
	return NewInMemoryStore([]InventoryItem{
		{SKU: "sku-101", Stock: 5},
		{SKU: "sku-102", Stock: 1},
	})
}

func TestPlaceOrderCommitsStockAndOrder(t *testing.T) {
	store := newTestStore()
	service := NewCheckoutService(store)

	order, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
		OrderID:    "ORD-1",
		CustomerID: "CUST-1",
		Lines: []OrderLine{
			{SKU: "sku-101", Quantity: 2, PriceCents: 1500},
			{SKU: "sku-102", Quantity: 1, PriceCents: 500},
		},
	})
	if err != nil {
		t.Fatalf("PlaceOrder() error = %v", err)
	}
	if order.Status != "created" {
		t.Fatalf("order status = %q, want created", order.Status)
	}
	if order.TotalCents != 3500 {
		t.Fatalf("total = %d, want 3500", order.TotalCents)
	}

	stored, err := store.Order(context.Background(), "ORD-1")
	if err != nil {
		t.Fatalf("Order() error = %v", err)
	}
	if stored.Summary() != "ORD-1 customer=CUST-1 status=created total=$35.00 lines=2" {
		t.Fatalf("Summary() = %q", stored.Summary())
	}

	items, err := store.Inventory(context.Background())
	if err != nil {
		t.Fatalf("Inventory() error = %v", err)
	}
	assertStock(t, items, "SKU-101", 3)
	assertStock(t, items, "SKU-102", 0)
}

func TestPlaceOrderRollsBackAllReservations(t *testing.T) {
	store := newTestStore()
	service := NewCheckoutService(store)

	_, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
		OrderID:    "ORD-2",
		CustomerID: "CUST-1",
		Lines: []OrderLine{
			{SKU: "sku-101", Quantity: 2, PriceCents: 1500},
			{SKU: "sku-102", Quantity: 2, PriceCents: 500},
		},
	})
	if !errors.Is(err, ErrOutOfStock) {
		t.Fatalf("PlaceOrder() error = %v, want ErrOutOfStock", err)
	}
	if _, err := store.Order(context.Background(), "ORD-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Order() error = %v, want ErrNotFound", err)
	}

	items, err := store.Inventory(context.Background())
	if err != nil {
		t.Fatalf("Inventory() error = %v", err)
	}
	assertStock(t, items, "SKU-101", 5)
	assertStock(t, items, "SKU-102", 1)
}

func TestPlaceOrderRollsBackWhenOrderCreateFails(t *testing.T) {
	store := newTestStore()
	service := NewCheckoutService(store)
	req := PlaceOrderRequest{
		OrderID:    "ORD-3",
		CustomerID: "CUST-1",
		Lines:      []OrderLine{{SKU: "sku-101", Quantity: 1, PriceCents: 100}},
	}

	if _, err := service.PlaceOrder(context.Background(), req); err != nil {
		t.Fatalf("PlaceOrder() first error = %v", err)
	}
	if _, err := service.PlaceOrder(context.Background(), req); !errors.Is(err, ErrDuplicateOrder) {
		t.Fatalf("PlaceOrder() duplicate error = %v, want ErrDuplicateOrder", err)
	}

	items, err := store.Inventory(context.Background())
	if err != nil {
		t.Fatalf("Inventory() error = %v", err)
	}
	assertStock(t, items, "SKU-101", 4)
}

func TestPlaceOrderValidation(t *testing.T) {
	service := NewCheckoutService(newTestStore())
	tests := []struct {
		name string
		req  PlaceOrderRequest
	}{
		{name: "missing order id", req: PlaceOrderRequest{CustomerID: "cust", Lines: []OrderLine{{SKU: "sku", Quantity: 1}}}},
		{name: "missing customer", req: PlaceOrderRequest{OrderID: "ord", Lines: []OrderLine{{SKU: "sku", Quantity: 1}}}},
		{name: "no lines", req: PlaceOrderRequest{OrderID: "ord", CustomerID: "cust"}},
		{name: "missing sku", req: PlaceOrderRequest{OrderID: "ord", CustomerID: "cust", Lines: []OrderLine{{Quantity: 1}}}},
		{name: "bad quantity", req: PlaceOrderRequest{OrderID: "ord", CustomerID: "cust", Lines: []OrderLine{{SKU: "sku", Quantity: 0}}}},
		{name: "bad price", req: PlaceOrderRequest{OrderID: "ord", CustomerID: "cust", Lines: []OrderLine{{SKU: "sku", Quantity: 1, PriceCents: -1}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.PlaceOrder(context.Background(), tt.req); !errors.Is(err, ErrInvalidOrder) {
				t.Fatalf("PlaceOrder() error = %v, want ErrInvalidOrder", err)
			}
		})
	}
}

func TestPlaceOrderHonorsCancelledContext(t *testing.T) {
	store := newTestStore()
	service := NewCheckoutService(store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.PlaceOrder(ctx, PlaceOrderRequest{
		OrderID:    "ORD-CANCELLED",
		CustomerID: "CUST-1",
		Lines:      []OrderLine{{SKU: "sku-101", Quantity: 1, PriceCents: 100}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PlaceOrder() error = %v, want context.Canceled", err)
	}

	items, err := store.Inventory(context.Background())
	if err != nil {
		t.Fatalf("Inventory() error = %v", err)
	}
	assertStock(t, items, "SKU-101", 5)
}

func assertStock(t *testing.T, items []InventoryItem, sku string, want int) {
	t.Helper()

	for _, item := range items {
		if item.SKU == sku {
			if item.Stock != want {
				t.Fatalf("%s stock = %d, want %d", sku, item.Stock, want)
			}
			return
		}
	}
	t.Fatalf("%s not found in inventory: %#v", sku, items)
}
