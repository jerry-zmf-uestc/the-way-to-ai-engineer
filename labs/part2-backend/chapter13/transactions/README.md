# 示例： Transaction Boundaries

## Goal

Learn where to place transaction boundaries when an order workflow has to
reserve inventory and create an order atomically. The example uses an in-memory
transaction manager so the behavior is testable without a MySQL server.

## Concepts

- `TransactionalStore` exposes `WithinTx`, keeping begin, commit, and rollback
  outside the service's business steps.
- `CheckoutService` validates a request, then runs stock reservation and order
  creation in one transaction callback.
- The in-memory transaction clones state before the callback and only swaps the
  snapshot into the store after the callback succeeds.
- Any validation, stock, duplicate-order, or context error returns before commit,
  so earlier successful operations inside the same transaction are rolled back.
- This shape can map to GORM's `db.Transaction(func(tx *gorm.DB) error)` later
  without leaking GORM into the service tests.

## Run

```bash
gofmt -l labs/part2-backend/chapter13/transactions
go -C labs vet ./part2-backend/chapter13/transactions
go -C labs test ./part2-backend/chapter13/transactions -race -count=1
go -C labs test ./part2-backend/chapter13/transactions -cover
go build -o /tmp/transactions ./labs/part2-backend/chapter13/transactions
go -C labs run ./part2-backend/chapter13/transactions
```

Expected output:

```text
transaction committed: ORD-1001 customer=CUST-7 status=created total=$42.97 lines=2 remaining=SKU-101:6,SKU-102:2
```

## Code Map

- [main.go](main.go): defines the in-memory transactional store, checkout
  workflow, and bounded demo output.
- [main_test.go](main_test.go): verifies commit and rollback behavior for the
  same service boundary exposed by the example.

## Project Use

This topic is the Day 52 data consistency exercise for the CloudWeGo track. It
bridges Day 51 repository interfaces and Day 53 migration or seed work by
showing which operations belong inside one commit boundary.

## Related Exercises

- [Chapter 13 Exercises](../../../../book/part2-backend/chapter13.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [chapter13/README.md](../../../../book/part2-backend/chapter13.md) for the chapter map, or continue with
[service-runtime](../../chapter21/service-runtime) to see how the same service
boundary grows into config, probe, timeout, and shutdown concerns.
