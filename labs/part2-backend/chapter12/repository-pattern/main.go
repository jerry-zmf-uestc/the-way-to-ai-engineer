package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrDuplicateSKU = errors.New("product sku already exists")
	ErrNotFound     = errors.New("product not found")
)

type Product struct {
	ID         int64
	SKU        string
	Name       string
	PriceCents int64
	Active     bool
	Stock      int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (p Product) Summary() string {
	state := "inactive"
	if p.Active {
		state = "active"
	}
	return fmt.Sprintf("%s %s %s $%.2f stock=%d", p.SKU, p.Name, state, float64(p.PriceCents)/100, p.Stock)
}

type ProductFilter struct {
	ActiveOnly bool
}

type ProductRepository interface {
	Create(ctx context.Context, product Product) (Product, error)
	FindBySKU(ctx context.Context, sku string) (Product, error)
	List(ctx context.Context, filter ProductFilter) ([]Product, error)
	UpdateStock(ctx context.Context, sku string, delta int) (Product, error)
}

type InMemoryProductRepository struct {
	mu     sync.RWMutex
	nextID int64
	bySKU  map[string]Product
	clock  func() time.Time
}

func NewInMemoryProductRepository(clock func() time.Time) *InMemoryProductRepository {
	if clock == nil {
		clock = time.Now
	}
	return &InMemoryProductRepository{
		nextID: 1,
		bySKU:  make(map[string]Product),
		clock:  clock,
	}
}

func (r *InMemoryProductRepository) Create(ctx context.Context, product Product) (Product, error) {
	if err := ctx.Err(); err != nil {
		return Product{}, err
	}

	sku := normalizeSKU(product.SKU)
	if sku == "" {
		return Product{}, fmt.Errorf("sku is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.bySKU[sku]; exists {
		return Product{}, fmt.Errorf("%w: %s", ErrDuplicateSKU, sku)
	}

	now := r.clock()
	product.ID = r.nextID
	product.SKU = sku
	product.CreatedAt = now
	product.UpdatedAt = now
	r.nextID++
	r.bySKU[sku] = product

	return product, nil
}

func (r *InMemoryProductRepository) FindBySKU(ctx context.Context, sku string) (Product, error) {
	if err := ctx.Err(); err != nil {
		return Product{}, err
	}

	key := normalizeSKU(sku)
	r.mu.RLock()
	defer r.mu.RUnlock()

	product, ok := r.bySKU[key]
	if !ok {
		return Product{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return product, nil
}

func (r *InMemoryProductRepository) List(ctx context.Context, filter ProductFilter) ([]Product, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	products := make([]Product, 0, len(r.bySKU))
	for _, product := range r.bySKU {
		if filter.ActiveOnly && !product.Active {
			continue
		}
		products = append(products, product)
	}
	sort.Slice(products, func(i, j int) bool {
		return products[i].SKU < products[j].SKU
	})
	return products, nil
}

func (r *InMemoryProductRepository) UpdateStock(ctx context.Context, sku string, delta int) (Product, error) {
	if err := ctx.Err(); err != nil {
		return Product{}, err
	}

	key := normalizeSKU(sku)
	r.mu.Lock()
	defer r.mu.Unlock()

	product, ok := r.bySKU[key]
	if !ok {
		return Product{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if product.Stock+delta < 0 {
		return Product{}, fmt.Errorf("stock cannot become negative")
	}
	product.Stock += delta
	product.UpdatedAt = r.clock()
	r.bySKU[key] = product

	return product, nil
}

type CatalogService struct {
	repo ProductRepository
}

func NewCatalogService(repo ProductRepository) CatalogService {
	return CatalogService{repo: repo}
}

type AddProductRequest struct {
	SKU        string
	Name       string
	PriceCents int64
	Stock      int
	Active     bool
}

func (s CatalogService) AddProduct(ctx context.Context, req AddProductRequest) (Product, error) {
	product := Product{
		SKU:        req.SKU,
		Name:       strings.TrimSpace(req.Name),
		PriceCents: req.PriceCents,
		Stock:      req.Stock,
		Active:     req.Active,
	}
	if err := validateProduct(product); err != nil {
		return Product{}, err
	}
	return s.repo.Create(ctx, product)
}

func (s CatalogService) Restock(ctx context.Context, sku string, delta int) (Product, error) {
	if delta == 0 {
		return Product{}, fmt.Errorf("stock delta cannot be zero")
	}
	return s.repo.UpdateStock(ctx, sku, delta)
}

func (s CatalogService) ActiveCatalog(ctx context.Context) ([]Product, int, error) {
	products, err := s.repo.List(ctx, ProductFilter{ActiveOnly: true})
	if err != nil {
		return nil, 0, err
	}

	totalStock := 0
	for _, product := range products {
		totalStock += product.Stock
	}
	return products, totalStock, nil
}

func validateProduct(product Product) error {
	if normalizeSKU(product.SKU) == "" {
		return fmt.Errorf("sku is required")
	}
	if strings.TrimSpace(product.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if product.PriceCents < 0 {
		return fmt.Errorf("price cents cannot be negative")
	}
	if product.Stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}
	return nil
}

func normalizeSKU(sku string) string {
	return strings.ToUpper(strings.TrimSpace(sku))
}

func main() {
	ctx := context.Background()
	now := time.Date(2026, 7, 28, 21, 0, 0, 0, time.UTC)
	repo := NewInMemoryProductRepository(func() time.Time { return now })
	service := NewCatalogService(repo)

	_, _ = service.AddProduct(ctx, AddProductRequest{
		SKU:        "sku-101",
		Name:       "CloudWeGo Mug",
		PriceCents: 1999,
		Stock:      12,
		Active:     true,
	})
	_, _ = service.AddProduct(ctx, AddProductRequest{
		SKU:        "sku-102",
		Name:       "CloudWeGo Sticker",
		PriceCents: 299,
		Stock:      3,
		Active:     true,
	})

	products, totalStock, err := service.ActiveCatalog(ctx)
	if err != nil {
		fmt.Printf("catalog error: %v\n", err)
		return
	}

	fmt.Printf("repository first=%s listed=%d total_stock=%d\n", products[0].Summary(), len(products), totalStock)
}
