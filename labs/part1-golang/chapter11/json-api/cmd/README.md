# 示例： json api

`json-api` is the Day 26 service-layering version of the JSON API practice.

## Goal

Move the single-package JSON API example into `cmd + internal`, so the process
entry stays small while the private HTTP logic becomes reusable and testable.

## Run

```bash
go -C labs run ./part1-golang/chapter11/json-api/cmd
```

Expected output:

```text
POST /api/echo -> 200 {"echo":"hello, json api","length":15}
```

## Layout

- `labs/part1-golang/chapter11/json-api/cmd` keeps only process wiring and demo output.
- `labs/part1-golang/chapter11/json-api/internal/jsonapi` owns the private HTTP handler and echo service.
- The Day 25 example remains in `labs/part1-golang/chapter11/practice-json-api` as the single-package learning version.

## Code Map

- [main.go](main.go): process entry that wires the HTTP demo without carrying the
  service logic itself.
- [main_test.go](main_test.go): keeps the command-level behavior bounded and
  stable.
- [../internal/jsonapi/server.go](../internal/jsonapi/server.go): private HTTP
  handler and route wiring.
- [../internal/jsonapi/service.go](../internal/jsonapi/service.go): echo service
  logic used by the handler.

## 相关练习

- [Chapter 11 · Engineering Exercises](../../../../../content/book/part1-golang/chapter11.md#练习)：回到正文里的练习区，按伪代码和验收标准继续
  扩展这个 topic。

## Continue

Read [practice-json-api](../../practice-json-api) first if you want the
single-package version, then compare how this command shrinks once `internal/`
absorbs the actual service code.
