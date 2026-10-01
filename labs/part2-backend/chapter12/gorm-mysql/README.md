# 示例： GORM Model and MySQL Config

## Goal

Learn the smallest useful data-layer setup before wiring it into Hertz or Kitex:
define a GORM model, build a MySQL DSN from structured config, and prepare a
GORM settings plan without connecting to a real database.

## Core Model

- A GORM model should make table shape explicit: primary key, unique indexes,
  non-null fields, timestamps, and soft delete indexes.
- MySQL connection details belong in a structured config, not scattered string
  concatenation.
- `parseTime=true` is required when scanning MySQL `DATETIME` / `TIMESTAMP`
  columns into Go `time.Time`.
- The morning example does not call `gorm.Open`, so automation does not depend
  on a running MySQL instance.
- Day 53 can replace the standard-library DSN plan with a real
  `gorm.io/driver/mysql` dialector when migrations and seed data are introduced.

## Run

```bash
gofmt -l labs/part2-backend/chapter12/gorm-mysql
go -C labs vet ./part2-backend/chapter12/gorm-mysql
go -C labs test ./part2-backend/chapter12/gorm-mysql -race -count=1
go -C labs test ./part2-backend/chapter12/gorm-mysql -cover
go build -o /tmp/gorm-mysql ./labs/part2-backend/chapter12/gorm-mysql
go -C labs run ./part2-backend/chapter12/gorm-mysql
```

Expected output:

```text
gorm model=SKU-001 CloudWeGo Hoodie active $89.99 dialect=mysql table=products disable_fk_migration=true singular_table=false dsn=app:%2A%2A%2A@tcp(127.0.0.1:3306)/commerce?charset=utf8mb4&loc=Local&parseTime=true
```

## Code Map

- [main.go](main.go): model shape, DSN assembly, and bounded demo output
- [main_test.go](main_test.go): validates config, DSN formatting, and stable demo behavior
- The example deliberately stops before `gorm.Open`, so tests stay local and deterministic.

## Related Exercises

- [Chapter 10 · HTTP Exercises](../../../../book/part1-golang/chapter10.md#练习): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

This topic is the Day 50 data-layer entry point for the CloudWeGo track. Later
topics can reuse the `Product` model and `MySQLConfig` shape when adding a
repository, transaction boundaries, migrations, and seed data.
