# 示例： Repository Pattern

## Goal

Learn how a service layer depends on a small repository interface instead of
knowing storage details directly. The example uses an in-memory repository so
validation stays deterministic without a running MySQL instance.

## Core Model

- The `ProductRepository` interface captures data operations needed by the
  service: create, find by SKU, list, and update stock.
- `CatalogService` owns input validation and business workflow, while the
  repository owns persistence details and duplicate-key behavior.
- Repository methods accept `context.Context` so the same shape can later be
  backed by GORM, MySQL, Kitex calls, or a fake store in tests.
- The in-memory implementation copies values through the map boundary and
  guards state with a mutex, which keeps tests race-clean.
- Day 52 can extend the interface with transaction boundaries before Day 53
  connects this shape to migration and seed data.

## Run

```bash
gofmt -l labs/part2-backend/chapter12/repository-pattern
go -C labs vet ./part2-backend/chapter12/repository-pattern
go -C labs test ./part2-backend/chapter12/repository-pattern -race -count=1
go -C labs test ./part2-backend/chapter12/repository-pattern -cover
go build -o /tmp/repository-pattern ./labs/part2-backend/chapter12/repository-pattern
go -C labs run ./part2-backend/chapter12/repository-pattern
```

Expected output:

```text
repository first=SKU-101 CloudWeGo Mug active $19.99 stock=12 listed=2 total_stock=15
```

## Code Map

- [main.go](main.go): bounded demo for create, lookup, list, and stock summary
- [main_test.go](main_test.go): behavior checks around validation and duplicate data
- The in-memory repository keeps the contract visible without a database dependency.

## Related Exercises

- [Chapter 10 · HTTP Exercises](../../../../content/book/part1-golang/chapter10.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

This topic is the Day 51 data-layer boundary exercise for the CloudWeGo track.
Later topics can replace `InMemoryProductRepository` with a GORM-backed
implementation while keeping service tests focused on behavior.
