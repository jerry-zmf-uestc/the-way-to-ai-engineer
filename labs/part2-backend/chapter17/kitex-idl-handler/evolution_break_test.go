package main

import (
	"testing"

	"github.com/cloudwego/frugal"
)

// This file is a cautionary counter-example. The other tests show COMPATIBLE
// evolution (adding optional fields / a new method). Here we prove, with a real
// Thrift Binary serialization round-trip, what BREAKING changes actually do to
// data on the wire.
//
// The core idea: a Thrift field's identity on the wire is its NUMBER, not its
// name. The decoder matches incoming bytes to struct fields purely by number.
// So renumbering a field is not a rename — it silently sends the old bytes to
// the wrong field (or nowhere).
//
// We model producer/consumer schema skew with small local structs that carry
// the same `frugal` tags codegen emits (e.g. `1,required,string`). frugal is
// the tag-driven reflection codec Kitex already depends on, so this is the same
// machinery a real service uses — no server or port required.

// --- Producer schema (v1): the bytes were written by an OLD peer ------------

type quoteReqV1 struct {
	Sku      string `frugal:"1,default,string"`
	Quantity int64  `frugal:"2,default,i64"`
}

// --- Consumer schema A (BREAKING): field number reused -----------------------
//
// Someone "renamed" quantity to itemCount but, crucially, also handed field
// number 2 to a brand-new string field and pushed itemCount to number 3. On the
// wire, number 2 now means "a string", but the old bytes at number 2 are an
// i64. Decoding mismatches the type and the real quantity (which the producer
// wrote at 2) never lands in ItemCount (now at 3).

type quoteReqBreaking struct {
	Sku       string `frugal:"1,default,string"`
	Note      string `frugal:"2,optional,string"` // NEW field grabbed number 2
	ItemCount int64  `frugal:"3,optional,i64"`    // old "quantity" moved to 3
}

// --- Consumer schema B (COMPATIBLE): only an optional field appended ---------
//
// Numbers 1 and 2 keep their original meaning; a new optional field takes the
// next free number (3). Old bytes decode exactly as before.

type quoteReqCompatible struct {
	Sku      string  `frugal:"1,default,string"`
	Quantity int64   `frugal:"2,default,i64"`
	Currency *string `frugal:"3,optional,string"` // appended, unused by old bytes
}

// --- Consumer schema C (BREAKING, silent): two same-typed numbers swapped ----
//
// The worst kind. Producer wrote warehouseID at 1 and shipToID at 2 (both i64).
// A refactor swaps their numbers. No type mismatch, no error, no data loss —
// the values just quietly land in each other's field. This is the bug that
// survives code review and ships to prod.

type shipmentV1 struct {
	WarehouseID int64 `frugal:"1,default,i64"`
	ShipToID    int64 `frugal:"2,default,i64"`
}

type shipmentSwapped struct {
	WarehouseID int64 `frugal:"2,default,i64"` // was 1
	ShipToID    int64 `frugal:"1,default,i64"` // was 2
}

func mustEncode(t *testing.T, v any) []byte {
	t.Helper()
	buf := make([]byte, frugal.EncodedSize(v))
	if _, err := frugal.EncodeObject(buf, nil, v); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf
}

// TestBreakingChange_RenumberedField is the headline counter-example: reusing a
// field number corrupts the round-trip even though the code compiles and no
// error is raised. The old Quantity=42 does NOT arrive as ItemCount.
func TestBreakingChange_RenumberedField(t *testing.T) {
	t.Parallel()

	// Old producer writes sku + quantity at their original numbers.
	wire := mustEncode(t, &quoteReqV1{Sku: "KITEX", Quantity: 42})

	// New consumer decodes with quantity's number reused by another field.
	var got quoteReqBreaking
	if _, err := frugal.DecodeObject(wire, &got); err != nil {
		// Depending on type-skew the decoder may or may not error; either way
		// the data is wrong. We assert on the silent-corruption outcome below.
		t.Logf("decode returned err (also acceptable evidence of breakage): %v", err)
	}

	// Sku (number 1, unchanged) survives.
	if got.Sku != "KITEX" {
		t.Fatalf("Sku = %q, want %q (number 1 was stable, should survive)", got.Sku, "KITEX")
	}

	// The whole point: the old quantity is LOST. It was written at number 2 as
	// an i64, but number 2 now means a string field (Note), and ItemCount lives
	// at number 3 where the old peer wrote nothing.
	if got.ItemCount == 42 {
		t.Fatalf("ItemCount = 42 — expected the renumber to LOSE the value, but it survived")
	}
	if got.ItemCount != 0 {
		t.Fatalf("ItemCount = %d, want 0 (old quantity must not reach the renumbered field)", got.ItemCount)
	}
	t.Logf("breaking change confirmed: quantity 42 vanished; ItemCount=%d, Note=%q", got.ItemCount, got.Note)
}

// TestCompatibleChange_AppendedOptionalField is the contrast: appending an
// optional field at a fresh number keeps old bytes fully decodable. sku and
// quantity still land correctly; the new field is simply absent (nil).
func TestCompatibleChange_AppendedOptionalField(t *testing.T) {
	t.Parallel()

	wire := mustEncode(t, &quoteReqV1{Sku: "KITEX", Quantity: 42})

	var got quoteReqCompatible
	if _, err := frugal.DecodeObject(wire, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Sku != "KITEX" {
		t.Fatalf("Sku = %q, want %q", got.Sku, "KITEX")
	}
	if got.Quantity != 42 {
		t.Fatalf("Quantity = %d, want 42 (number 2 kept its meaning)", got.Quantity)
	}
	if got.Currency != nil {
		t.Fatalf("Currency = %v, want nil (old bytes never set the appended field)", got.Currency)
	}
}

// TestBreakingChange_SwappedFieldNumbers is the most dangerous variant: two
// fields of the SAME type swap numbers. There is no type mismatch, no decode
// error, and no data loss — the values simply land in the wrong fields. A
// warehouse ID silently becomes a ship-to ID. Nothing fails; the data is just
// wrong, which is exactly why field numbers must be treated as immutable.
func TestBreakingChange_SwappedFieldNumbers(t *testing.T) {
	t.Parallel()

	// Old producer: warehouse=100 (number 1), shipTo=200 (number 2).
	wire := mustEncode(t, &shipmentV1{WarehouseID: 100, ShipToID: 200})

	var got shipmentSwapped
	if _, err := frugal.DecodeObject(wire, &got); err != nil {
		t.Fatalf("decode: %v (a swap of same-typed numbers should NOT even error)", err)
	}

	// The bytes at number 1 (value 100) now flow into ShipToID, and the bytes
	// at number 2 (value 200) flow into WarehouseID. Everything "works", and
	// everything is wrong.
	if got.ShipToID != 100 {
		t.Fatalf("ShipToID = %d, want 100 (number 1's bytes must follow the number, not the name)", got.ShipToID)
	}
	if got.WarehouseID != 200 {
		t.Fatalf("WarehouseID = %d, want 200 (number 2's bytes followed the number)", got.WarehouseID)
	}
	t.Logf("silent corruption confirmed: warehouse/shipTo swapped with no error (WarehouseID=%d, ShipToID=%d)", got.WarehouseID, got.ShipToID)
}
