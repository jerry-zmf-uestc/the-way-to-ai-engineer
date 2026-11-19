package main

import (
	"context"
	"fmt"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter17/kitex-idl-handler/kitex_gen/order"
)

// ptr returns the address of v. Optional Thrift fields are generated as
// pointers, so a caller that WANTS to set one has to pass an address; leaving
// the field nil is how a caller says "I'm omitting this".
func ptr[T any](v T) *T { return &v }

// Demo runs three bounded handler calls that contrast how optional contract
// fields behave, then returns deterministic output. It never opens a port, so
// daily automation can run `go run` without hanging.
//
// The three calls model contract evolution:
//   - v1 caller: only sets required fields; optional Currency/DiscountBps stay
//     nil and the handler falls back to defaults (this is an old client that
//     predates the v2 fields, still working unchanged).
//   - v2 caller: sets the optional Currency.
//   - v2 caller: sets the optional DiscountBps.
func Demo() string {
	handler := NewOrderServiceHandler(map[string]int64{
		"GO-BOOK": 4999,
		"KITEX":   2999,
	})

	// v1-style request: optional fields omitted (nil pointers). Backward
	// compatible — the handler resolves currency to its USD default.
	v1, err := handler.Quote(context.Background(), &order.QuoteRequest{
		Sku:      " kitex ",
		Quantity: 3,
	})
	if err != nil {
		return fmt.Sprintf("v1 quote failed: %v", err)
	}

	// v2 request that opts into the currency field added in the IDL.
	v2Currency, err := handler.Quote(context.Background(), &order.QuoteRequest{
		Sku:      "kitex",
		Quantity: 3,
		Currency: ptr("EUR"),
	})
	if err != nil {
		return fmt.Sprintf("v2 currency quote failed: %v", err)
	}

	// v2 request that opts into the discount field (500 bps = 5% off).
	v2Discount, err := handler.Quote(context.Background(), &order.QuoteRequest{
		Sku:         "kitex",
		Quantity:    3,
		DiscountBps: ptr(int64(500)),
	})
	if err != nil {
		return fmt.Sprintf("v2 discount quote failed: %v", err)
	}

	return fmt.Sprintf(
		"v1(no-opt) -> %s | v2(currency) -> %s | v2(discount) -> %d cents",
		v1.GetMessage(),
		v2Currency.GetMessage(),
		v2Discount.GetTotalCents(),
	)
}

func main() {
	fmt.Println(Demo())
}
