package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter17/kitex-idl-handler/kitex_gen/order"
)

// Sentinel errors let tests assert failure modes with errors.Is instead of
// matching on message strings.
var (
	ErrNilQuoteRequest = errors.New("quote request is nil")
	ErrMissingSKU      = errors.New("sku is required")
	ErrInvalidQuantity = errors.New("quantity must be positive")
	ErrNilPingRequest  = errors.New("ping request is nil")
)

// defaultCurrency is used when a caller omits the optional currency field. An
// old v1 client never sets Currency, so its requests arrive with a nil pointer
// and fall back to this value — that is what "backward compatible" means here.
const defaultCurrency = "USD"

// OrderServiceHandler implements the generated order.OrderService interface. It
// satisfies every method codegen declares (Quote + Ping), which is why the
// compile-time check in main_test.go holds: the handler can never silently
// drift from the contract.
type OrderServiceHandler struct {
	priceBook map[string]int64
}

// NewOrderServiceHandler copies the price book and normalizes SKU keys so
// lookups are case- and whitespace-insensitive.
func NewOrderServiceHandler(priceBook map[string]int64) OrderServiceHandler {
	copied := make(map[string]int64, len(priceBook))
	for sku, cents := range priceBook {
		copied[strings.ToUpper(strings.TrimSpace(sku))] = cents
	}
	return OrderServiceHandler{priceBook: copied}
}

// Quote prices a request and demonstrates how optional Thrift fields surface in
// Go. Currency and DiscountBps are `optional` in the IDL, so codegen emits them
// as *string / *int64. A nil pointer means "the caller omitted this field",
// which is deliberately different from an explicit zero value:
//
//   - Currency == nil        -> old client / not provided -> fall back to USD
//   - DiscountBps == nil     -> no discount requested
//   - DiscountBps == &0      -> caller explicitly asked for a 0 bps discount
//
// Reaching for req.GetCurrency() alone would hide that distinction because the
// generated getter returns the zero value for nil. We check IsSet* first.
func (h OrderServiceHandler) Quote(ctx context.Context, req *order.QuoteRequest) (*order.QuoteResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrNilQuoteRequest
	}

	sku := strings.ToUpper(strings.TrimSpace(req.GetSku()))
	if sku == "" {
		return nil, ErrMissingSKU
	}
	if req.GetQuantity() <= 0 {
		return nil, ErrInvalidQuantity
	}

	// Optional currency: nil pointer -> caller omitted it -> default. This is
	// the exact spot where "old client keeps working" is implemented.
	currency := defaultCurrency
	if req.IsSetCurrency() {
		currency = strings.ToUpper(strings.TrimSpace(req.GetCurrency()))
	}

	unitCents := h.priceBook[sku]
	if unitCents == 0 {
		unitCents = 1999
	}

	total := unitCents * req.GetQuantity()

	// Optional discount: distinguish "not requested" (nil) from an explicit 0.
	if req.IsSetDiscountBps() {
		bps := req.GetDiscountBps()
		if bps > 0 {
			total -= total * bps / 10000
		}
	}

	return &order.QuoteResponse{
		Sku:        sku,
		TotalCents: total,
		Message:    fmt.Sprintf("quote %s x%d = $%.2f %s", sku, req.GetQuantity(), float64(total)/100, currency),
		Currency:   &currency, // echo the resolved currency via the optional response field
	}, nil
}

// Ping is the v2 method. Adding it did not break any existing caller: clients
// that only know Quote keep compiling untouched. It exists to show that growing
// a service interface is backward compatible, while removing/renaming is not.
func (h OrderServiceHandler) Ping(ctx context.Context, req *order.PingRequest) (*order.PingResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrNilPingRequest
	}
	return &order.PingResponse{Status: "ok"}, nil
}
