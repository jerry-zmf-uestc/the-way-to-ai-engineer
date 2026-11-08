package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jerry-zmf-uestc/the-way-to-ai-engineer/labs/part2-backend/chapter17/kitex-idl-handler/kitex_gen/order"
)

// TestHandlerSatisfiesGeneratedInterface fails to compile if OrderServiceHandler
// ever drifts from the interface generated out of idl/order.thrift. This is the
// real "IDL -> generated code -> handler" contract check: the IDL owns the
// method set (Quote + the v2 Ping), codegen turns it into order.OrderService,
// and the handler must satisfy every method. Add a method to the IDL without
// implementing it and this line stops compiling.
func TestHandlerSatisfiesGeneratedInterface(t *testing.T) {
	t.Parallel()

	var _ order.OrderService = NewOrderServiceHandler(nil)
}

func TestQuote(t *testing.T) {
	t.Parallel()

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name       string
		ctx        context.Context
		req        *order.QuoteRequest
		wantSKU    string
		wantTotal  int64
		wantErr    error
		wantPhrase string
	}{
		{
			name:       "normalizes sku and multiplies configured price",
			ctx:        context.Background(),
			req:        &order.QuoteRequest{Sku: " kitex ", Quantity: 3},
			wantSKU:    "KITEX",
			wantTotal:  8997,
			wantPhrase: "quote KITEX x3 = $89.97 USD",
		},
		{
			name:       "uses default price for unknown sku",
			ctx:        context.Background(),
			req:        &order.QuoteRequest{Sku: "new-item", Quantity: 2},
			wantSKU:    "NEW-ITEM",
			wantTotal:  3998,
			wantPhrase: "quote NEW-ITEM x2 = $39.98 USD",
		},
		{
			name:    "rejects nil request",
			ctx:     context.Background(),
			req:     nil,
			wantErr: ErrNilQuoteRequest,
		},
		{
			name:    "rejects missing sku",
			ctx:     context.Background(),
			req:     &order.QuoteRequest{Sku: " \t ", Quantity: 1},
			wantErr: ErrMissingSKU,
		},
		{
			name:    "rejects non-positive quantity",
			ctx:     context.Background(),
			req:     &order.QuoteRequest{Sku: "kitex", Quantity: 0},
			wantErr: ErrInvalidQuantity,
		},
		{
			name:    "preserves canceled context",
			ctx:     canceledCtx,
			req:     &order.QuoteRequest{Sku: "kitex", Quantity: 1},
			wantErr: context.Canceled,
		},
	}

	handler := NewOrderServiceHandler(map[string]int64{
		"KITEX": 2999,
	})

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := handler.Quote(tt.ctx, tt.req)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Quote() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Quote() error = %v", err)
			}
			if got.GetSku() != tt.wantSKU {
				t.Fatalf("Sku = %q, want %q", got.GetSku(), tt.wantSKU)
			}
			if got.GetTotalCents() != tt.wantTotal {
				t.Fatalf("TotalCents = %d, want %d", got.GetTotalCents(), tt.wantTotal)
			}
			if got.GetMessage() != tt.wantPhrase {
				t.Fatalf("Message = %q, want %q", got.GetMessage(), tt.wantPhrase)
			}
		})
	}
}

// TestQuote_OptionalCurrency documents the core teaching point: an `optional`
// Thrift field is generated as a pointer, so nil ("caller omitted it") and a
// set value are distinguishable. A v1 client that never sets Currency must keep
// working and fall back to the default.
func TestQuote_OptionalCurrency(t *testing.T) {
	t.Parallel()

	handler := NewOrderServiceHandler(map[string]int64{"KITEX": 2999})

	t.Run("nil currency falls back to default (v1 client)", func(t *testing.T) {
		t.Parallel()

		got, err := handler.Quote(context.Background(), &order.QuoteRequest{
			Sku:      "kitex",
			Quantity: 1,
			// Currency left nil on purpose: this is the old client.
		})
		if err != nil {
			t.Fatalf("Quote() error = %v", err)
		}
		if got.GetCurrency() != defaultCurrency {
			t.Fatalf("Currency = %q, want default %q", got.GetCurrency(), defaultCurrency)
		}
	})

	t.Run("set currency is honored (v2 client)", func(t *testing.T) {
		t.Parallel()

		want := "EUR"
		got, err := handler.Quote(context.Background(), &order.QuoteRequest{
			Sku:      "kitex",
			Quantity: 1,
			Currency: &want,
		})
		if err != nil {
			t.Fatalf("Quote() error = %v", err)
		}
		if got.GetCurrency() != want {
			t.Fatalf("Currency = %q, want %q", got.GetCurrency(), want)
		}
	})
}

// TestQuote_OptionalDiscount shows why nil vs an explicit zero matters: both
// leave the total unchanged here, but only because the handler treats "not
// requested" and "0 bps" the same on purpose. The pointer lets it tell them
// apart if the business ever needs to.
func TestQuote_OptionalDiscount(t *testing.T) {
	t.Parallel()

	handler := NewOrderServiceHandler(map[string]int64{"KITEX": 2999})
	zero := int64(0)
	fivePct := int64(500)

	tests := []struct {
		name      string
		discount  *int64
		wantTotal int64
	}{
		{name: "nil discount not requested", discount: nil, wantTotal: 8997},
		{name: "explicit zero discount", discount: &zero, wantTotal: 8997},
		{name: "500 bps trims five percent", discount: &fivePct, wantTotal: 8548},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := handler.Quote(context.Background(), &order.QuoteRequest{
				Sku:         "kitex",
				Quantity:    3,
				DiscountBps: tt.discount,
			})
			if err != nil {
				t.Fatalf("Quote() error = %v", err)
			}
			if got.GetTotalCents() != tt.wantTotal {
				t.Fatalf("TotalCents = %d, want %d", got.GetTotalCents(), tt.wantTotal)
			}
		})
	}
}

// TestPing covers the v2 method added to the service. Its existence is the
// backward-compatible-growth story: old callers ignore it, new callers use it.
func TestPing(t *testing.T) {
	t.Parallel()

	handler := NewOrderServiceHandler(nil)

	t.Run("returns ok", func(t *testing.T) {
		t.Parallel()

		got, err := handler.Ping(context.Background(), &order.PingRequest{})
		if err != nil {
			t.Fatalf("Ping() error = %v", err)
		}
		if got.GetStatus() != "ok" {
			t.Fatalf("Status = %q, want %q", got.GetStatus(), "ok")
		}
	})

	t.Run("rejects nil request", func(t *testing.T) {
		t.Parallel()

		if _, err := handler.Ping(context.Background(), nil); !errors.Is(err, ErrNilPingRequest) {
			t.Fatalf("Ping() error = %v, want %v", err, ErrNilPingRequest)
		}
	})

	t.Run("preserves canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := handler.Ping(ctx, &order.PingRequest{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("Ping() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestDemo(t *testing.T) {
	t.Parallel()

	got := Demo()
	want := "v1(no-opt) -> quote KITEX x3 = $89.97 USD | v2(currency) -> quote KITEX x3 = $89.97 EUR | v2(discount) -> 8548 cents"
	if got != want {
		t.Fatalf("Demo() = %q, want %q", got, want)
	}
}

func ExampleDemo() {
	fmt.Println(Demo())
	// Output:
	// v1(no-opt) -> quote KITEX x3 = $89.97 USD | v2(currency) -> quote KITEX x3 = $89.97 EUR | v2(discount) -> 8548 cents
}
