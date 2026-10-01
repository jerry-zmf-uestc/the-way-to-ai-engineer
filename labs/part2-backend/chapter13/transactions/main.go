package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrDuplicateOrder = errors.New("order already exists")
	ErrInvalidOrder   = errors.New("invalid order")
	ErrNotFound       = errors.New("record not found")
	ErrOutOfStock     = errors.New("stock is not enough")
)

type OrderLine struct {
	SKU        string
	Quantity   int
	PriceCents int64
}

type Order struct {
	ID         string
	CustomerID string
	Lines      []OrderLine
	Status     string
	TotalCents int64
}

func (o Order) Summary() string {
	return fmt.Sprintf("%s customer=%s status=%s total=$%.2f lines=%d", o.ID, o.CustomerID, o.Status, float64(o.TotalCents)/100, len(o.Lines))
}

type InventoryItem struct {
	SKU   string
	Stock int
}

type TransactionalStore interface {
	WithinTx(ctx context.Context, fn func(context.Context, StoreTx) error) error
}

type StoreTx interface {
	CreateOrder(ctx context.Context, order Order) error
	ReserveStock(ctx context.Context, sku string, quantity int) error
}

type InMemoryStore struct {
	mu        sync.Mutex
	orders    map[string]Order
	inventory map[string]InventoryItem
}

func NewInMemoryStore(items []InventoryItem) *InMemoryStore {
	inventory := make(map[string]InventoryItem, len(items))
	for _, item := range items {
		item.SKU = normalizeSKU(item.SKU)
		inventory[item.SKU] = item
	}
	return &InMemoryStore{
		orders:    make(map[string]Order),
		inventory: inventory,
	}
}

func (s *InMemoryStore) WithinTx(ctx context.Context, fn func(context.Context, StoreTx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx := &memoryTx{
		orders:    cloneOrders(s.orders),
		inventory: cloneInventory(s.inventory),
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	s.orders = tx.orders
	s.inventory = tx.inventory
	return nil
}

func (s *InMemoryStore) Order(ctx context.Context, id string) (Order, error) {
	if err := ctx.Err(); err != nil {
		return Order{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	order, ok := s.orders[id]
	if !ok {
		return Order{}, fmt.Errorf("%w: order %s", ErrNotFound, id)
	}
	return cloneOrder(order), nil
}

func (s *InMemoryStore) Inventory(ctx context.Context) ([]InventoryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]InventoryItem, 0, len(s.inventory))
	for _, item := range s.inventory {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SKU < items[j].SKU
	})
	return items, nil
}

type memoryTx struct {
	orders    map[string]Order
	inventory map[string]InventoryItem
}

func (tx *memoryTx) CreateOrder(ctx context.Context, order Order) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	order.ID = strings.TrimSpace(order.ID)
	if order.ID == "" {
		return fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	if _, exists := tx.orders[order.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateOrder, order.ID)
	}
	tx.orders[order.ID] = cloneOrder(order)
	return nil
}

func (tx *memoryTx) ReserveStock(ctx context.Context, sku string, quantity int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if quantity <= 0 {
		return fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
	}

	key := normalizeSKU(sku)
	item, ok := tx.inventory[key]
	if !ok {
		return fmt.Errorf("%w: inventory %s", ErrNotFound, key)
	}
	if item.Stock < quantity {
		return fmt.Errorf("%w: sku=%s available=%d requested=%d", ErrOutOfStock, key, item.Stock, quantity)
	}
	item.Stock -= quantity
	tx.inventory[key] = item
	return nil
}

type CheckoutService struct {
	store TransactionalStore
}

func NewCheckoutService(store TransactionalStore) CheckoutService {
	return CheckoutService{store: store}
}

type PlaceOrderRequest struct {
	OrderID    string
	CustomerID string
	Lines      []OrderLine
}

func (s CheckoutService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (Order, error) {
	order, err := newPendingOrder(req)
	if err != nil {
		return Order{}, err
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx StoreTx) error {
		for _, line := range order.Lines {
			if err := tx.ReserveStock(ctx, line.SKU, line.Quantity); err != nil {
				return err
			}
		}
		order.Status = "created"
		return tx.CreateOrder(ctx, order)
	})
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

func newPendingOrder(req PlaceOrderRequest) (Order, error) {
	id := strings.TrimSpace(req.OrderID)
	customerID := strings.TrimSpace(req.CustomerID)
	if id == "" {
		return Order{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	if customerID == "" {
		return Order{}, fmt.Errorf("%w: customer id is required", ErrInvalidOrder)
	}
	if len(req.Lines) == 0 {
		return Order{}, fmt.Errorf("%w: at least one line is required", ErrInvalidOrder)
	}

	lines := make([]OrderLine, 0, len(req.Lines))
	var total int64
	for _, line := range req.Lines {
		line.SKU = normalizeSKU(line.SKU)
		if line.SKU == "" {
			return Order{}, fmt.Errorf("%w: line sku is required", ErrInvalidOrder)
		}
		if line.Quantity <= 0 {
			return Order{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
		}
		if line.PriceCents < 0 {
			return Order{}, fmt.Errorf("%w: price cents cannot be negative", ErrInvalidOrder)
		}
		total += int64(line.Quantity) * line.PriceCents
		lines = append(lines, line)
	}

	return Order{
		ID:         id,
		CustomerID: customerID,
		Lines:      lines,
		Status:     "pending",
		TotalCents: total,
	}, nil
}

func cloneOrders(src map[string]Order) map[string]Order {
	dst := make(map[string]Order, len(src))
	for id, order := range src {
		dst[id] = cloneOrder(order)
	}
	return dst
}

func cloneOrder(order Order) Order {
	order.Lines = append([]OrderLine(nil), order.Lines...)
	return order
}

func cloneInventory(src map[string]InventoryItem) map[string]InventoryItem {
	dst := make(map[string]InventoryItem, len(src))
	for sku, item := range src {
		dst[sku] = item
	}
	return dst
}

func normalizeSKU(sku string) string {
	return strings.ToUpper(strings.TrimSpace(sku))
}

func main() {
	ctx := context.Background()
	store := NewInMemoryStore([]InventoryItem{
		{SKU: "sku-101", Stock: 8},
		{SKU: "sku-102", Stock: 3},
	})
	service := NewCheckoutService(store)

	order, err := service.PlaceOrder(ctx, PlaceOrderRequest{
		OrderID:    "ORD-1001",
		CustomerID: "CUST-7",
		Lines: []OrderLine{
			{SKU: "sku-101", Quantity: 2, PriceCents: 1999},
			{SKU: "sku-102", Quantity: 1, PriceCents: 299},
		},
	})
	if err != nil {
		fmt.Printf("checkout failed: %v\n", err)
		return
	}

	items, _ := store.Inventory(ctx)
	fmt.Printf("transaction committed: %s remaining=%s:%d,%s:%d\n", order.Summary(), items[0].SKU, items[0].Stock, items[1].SKU, items[1].Stock)
}
