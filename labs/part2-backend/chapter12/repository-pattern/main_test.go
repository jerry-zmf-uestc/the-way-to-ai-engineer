package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	now := time.Date(2026, 7, 28, 21, 0, 0, 0, time.UTC)
	return func() time.Time { return now }
}

func TestCatalogServiceAddsAndListsActiveProducts(t *testing.T) {
	repo := NewInMemoryProductRepository(fixedClock())
	service := NewCatalogService(repo)

	_, err := service.AddProduct(context.Background(), AddProductRequest{
		SKU:        " sku-200 ",
		Name:       "CloudWeGo Cap",
		PriceCents: 2599,
		Stock:      5,
		Active:     true,
	})
	if err != nil {
		t.Fatalf("AddProduct() error = %v", err)
	}
	_, err = service.AddProduct(context.Background(), AddProductRequest{
		SKU:        "sku-100",
		Name:       "Draft Hoodie",
		PriceCents: 8999,
		Stock:      2,
		Active:     false,
	})
	if err != nil {
		t.Fatalf("AddProduct() inactive error = %v", err)
	}

	products, totalStock, err := service.ActiveCatalog(context.Background())
	if err != nil {
		t.Fatalf("ActiveCatalog() error = %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("active products len = %d, want 1", len(products))
	}
	if products[0].SKU != "SKU-200" {
		t.Fatalf("SKU = %q, want SKU-200", products[0].SKU)
	}
	if totalStock != 5 {
		t.Fatalf("totalStock = %d, want 5", totalStock)
	}
}

func TestRepositoryRejectsDuplicateSKU(t *testing.T) {
	repo := NewInMemoryProductRepository(fixedClock())
	product := Product{SKU: "sku-1", Name: "Mug", PriceCents: 1000, Active: true}

	if _, err := repo.Create(context.Background(), product); err != nil {
		t.Fatalf("Create() first error = %v", err)
	}
	if _, err := repo.Create(context.Background(), product); !errors.Is(err, ErrDuplicateSKU) {
		t.Fatalf("Create() duplicate error = %v, want ErrDuplicateSKU", err)
	}
}

func TestRepositoryReturnsCopies(t *testing.T) {
	repo := NewInMemoryProductRepository(fixedClock())
	created, err := repo.Create(context.Background(), Product{
		SKU:        "sku-copy",
		Name:       "Copy Test",
		PriceCents: 1500,
		Stock:      4,
		Active:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	created.Stock = 0
	found, err := repo.FindBySKU(context.Background(), "sku-copy")
	if err != nil {
		t.Fatalf("FindBySKU() error = %v", err)
	}
	if found.Stock != 4 {
		t.Fatalf("stored stock = %d, want 4", found.Stock)
	}
}

func TestRepositoryStockBoundaries(t *testing.T) {
	repo := NewInMemoryProductRepository(fixedClock())
	if _, err := repo.Create(context.Background(), Product{
		SKU:        "sku-stock",
		Name:       "Stock Test",
		PriceCents: 1,
		Stock:      3,
		Active:     true,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := repo.UpdateStock(context.Background(), "sku-stock", 2)
	if err != nil {
		t.Fatalf("UpdateStock() increase error = %v", err)
	}
	if updated.Stock != 5 {
		t.Fatalf("stock after increase = %d, want 5", updated.Stock)
	}
	if _, err := repo.UpdateStock(context.Background(), "sku-stock", -6); err == nil {
		t.Fatal("UpdateStock() excessive decrease error = nil, want error")
	}
}

func TestServiceValidation(t *testing.T) {
	service := NewCatalogService(NewInMemoryProductRepository(fixedClock()))
	tests := []struct {
		name string
		req  AddProductRequest
	}{
		{name: "missing sku", req: AddProductRequest{Name: "Mug", PriceCents: 1}},
		{name: "missing name", req: AddProductRequest{SKU: "sku", PriceCents: 1}},
		{name: "negative price", req: AddProductRequest{SKU: "sku", Name: "Mug", PriceCents: -1}},
		{name: "negative stock", req: AddProductRequest{SKU: "sku", Name: "Mug", PriceCents: 1, Stock: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.AddProduct(context.Background(), tt.req); err == nil {
				t.Fatal("AddProduct() error = nil, want validation error")
			}
		})
	}
}

func TestRepositoryHonorsCancelledContext(t *testing.T) {
	repo := NewInMemoryProductRepository(fixedClock())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repo.Create(ctx, Product{SKU: "sku", Name: "Mug"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create() error = %v, want context.Canceled", err)
	}
	if _, err := repo.FindBySKU(ctx, "sku"); !errors.Is(err, context.Canceled) {
		t.Fatalf("FindBySKU() error = %v, want context.Canceled", err)
	}
	if _, err := repo.List(ctx, ProductFilter{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("List() error = %v, want context.Canceled", err)
	}
	if _, err := repo.UpdateStock(ctx, "sku", 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateStock() error = %v, want context.Canceled", err)
	}
}

func TestProductSummary(t *testing.T) {
	product := Product{
		SKU:        "SKU-101",
		Name:       "CloudWeGo Mug",
		PriceCents: 1999,
		Active:     true,
		Stock:      12,
	}

	got := product.Summary()
	want := "SKU-101 CloudWeGo Mug active $19.99 stock=12"
	if got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}
