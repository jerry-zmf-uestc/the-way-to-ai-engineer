# 示例： JSON API

本示例用标准库 `net/http` 和 `encoding/json` 实现一个最小 `POST /api/echo` endpoint，并用 `httptest` 做可重复验证。

## 学习目标

- 用 `json.Decoder` 从请求体读取 JSON，并拒绝无法解析的输入。
- 用 `json.Encoder` 输出稳定响应，设置明确的 `Content-Type`。
- 用 table-driven tests 覆盖成功请求、错误方法、坏 JSON、空消息和未知路由。
- 让 `go run` 输出一次有界 demo，避免自动化复盘被长驻服务阻塞。

## 运行

```bash
go -C labs test ./part1-golang/chapter11/practice-json-api -race -count=1
go -C labs run ./part1-golang/chapter11/practice-json-api
```

期望输出：

```text
POST /api/echo -> 200 {"echo":"hello, json api","length":15}
```

## Code Map

- [main.go](main.go)：单 package 版本的 HTTP handler、JSON 解码和有界 demo。
- [main_test.go](main_test.go)：用 `httptest` 覆盖成功和失败请求，不依赖真实端口。

## 取舍

- Day 25 继续保持在 `labs/part1-golang/chapter11/practice-json-api/`，先练习 JSON API 行为，不提前拆服务分层。
- endpoint 使用 `httptest` 演示请求，不绑定真实端口。
- 请求体只保留一个 `message` 字段，便于聚焦解码、校验和响应编码。

## 相关练习

- [Chapter 11 · Engineering Exercises](../../../../content/book/part1-golang/chapter11.md#练习)：回到正文里的练习区，按伪代码和验收标准继续
  扩展这个 topic。

## Continue

- <https://pkg.go.dev/net/http>
- <https://pkg.go.dev/net/http/httptest>
- <https://pkg.go.dev/encoding/json>
- 对比分层版本：[json-api/cmd](../json-api/cmd)
