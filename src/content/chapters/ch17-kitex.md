---
pageId: "ch17-kitex"
title: "第 17 章：Kitex 与 RPC 契约"
part: "Part 2 · 后端进阶"
unitNumber: 17
order: 34
description: "IDL、代码生成与 optional 字段的契约演进。"
published: true
source: "book/part2-backend/chapter17.md"
labs: ["labs/part2-backend/chapter17/kitex-idl-handler","labs/part2-backend/chapter17/kitex-rpc"]
capabilities: ["Kitex","IDL","契约演进"]
---
HTTP 接口改错了，问题还能在文本层被看见；RPC 契约改错了，最危险的情况是"还能跑，只是数据悄悄错位"。本章用两个 Kitex 示例建立 IDL → 生成代码 → handler 的链路，重点在字段编号与 optional 的演进纪律。

## 17.1 IDL → codegen → handler

**IDL 定义契约，生成代码承载协议，handler 实现业务**；编译期断言把三层钉在一起。

```thrift
// 摘录自 kitex-idl-handler/idl/order.thrift。
service OrderService {
	QuoteResponse Quote(1: QuoteRequest req)
	PingResponse Ping(1: PingRequest req)
}
```

```go
// 摘录自 kitex-idl-handler/main_test.go：编译期契约断言。
var _ order.OrderService = NewOrderServiceHandler(nil)

// 摘录自 kitex-idl-handler/handler.go。
type OrderServiceHandler struct {
	priceBook map[string]int64
}
```

- IDL 新增方法后，`kitex_gen` 会生成新的方法集合；handler 未实现新方法，编译就失败——契约检查发生在编译期，不用等到运行。

[kitex-rpc](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/tree/main/labs/part2-backend/chapter17/kitex-rpc) 是最小 Thrift-first RPC，用来先跑通这条链路：

```bash
go -C labs run ./part2-backend/chapter17/kitex-rpc
```

预期输出：

```text
Echo -> hello kitex (11), Add -> 1024
```

## 17.2 optional：区分"没传"与"显式零值"

**Thrift `optional` 在 Go 里的真实语义：`GetXxx()` 只给值，`IsSetXxx()` 才告诉你调用方到底传没传。** 旧客户端不传字段应走默认值，新客户端显式传入则覆盖默认逻辑：

```go
// 摘录自 kitex-idl-handler/handler.go。
func (h OrderServiceHandler) Quote(ctx context.Context, req *order.QuoteRequest) (*order.QuoteResponse, error) {
	sku := strings.ToUpper(strings.TrimSpace(req.GetSku()))
	if sku == "" {
		return nil, ErrMissingSKU
	}

	currency := defaultCurrency
	if req.IsSetCurrency() {
		currency = strings.ToUpper(strings.TrimSpace(req.GetCurrency()))
	}

	unitCents := h.priceBook[sku]
	if unitCents == 0 {
		unitCents = 1999
	}
	total := unitCents * req.GetQuantity()
	if req.IsSetDiscountBps() {
		bps := req.GetDiscountBps()
		if bps > 0 {
			total -= total * bps / 10000
		}
	}

	return &order.QuoteResponse{
		Sku:        sku,
		TotalCents: total,
		Currency:   &currency,
		Message:    fmt.Sprintf("quote %s x%d = $%.2f %s", sku, req.GetQuantity(), float64(total)/100, currency),
	}, nil
}
```

- 生成的 getter 只返回零值，"字段缺席"和"字段存在但值为空"只能靠 `IsSet*` 或指针是否为 nil 区分。

[kitex-idl-handler](https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/tree/main/labs/part2-backend/chapter17/kitex-idl-handler) 演示 v1（不传 optional）与 v2（显式传入）调用方的差异：

```bash
go -C labs run ./part2-backend/chapter17/kitex-idl-handler
```

预期输出：

```text
v1(no-opt) -> quote KITEX x3 = $89.97 USD | v2(currency) -> quote KITEX x3 = $89.97 EUR | v2(discount) -> 8548 cents
```

## 17.3 契约演进：新增能力，保留旧编号语义

**RPC 最稳的演进方式是"新增字段 / 新增方法 / 保留旧语义"，而不是复用已有编号去表达新含义。** 测试把这条原则写成断言——旧调用方沿老路径工作，新调用方安全使用新方法：

```go
// 摘录自 kitex-idl-handler/main_test.go。
func TestQuote_OptionalCurrency(t *testing.T) {
	got, err := handler.Quote(context.Background(), &order.QuoteRequest{
		Sku:      "kitex",
		Quantity: 1,
		// Currency 留空：这就是旧 v1 调用方。
	})
	if err != nil { t.Fatal(err) }
	if got == nil { t.Fatal("nil response") }
	// 还需断言 currency 或 status 等业务字段，不能只检查调用成功。
}

func TestPing(t *testing.T) {
	got, err := handler.Ping(context.Background(), &order.PingRequest{})
	if err != nil { t.Fatal(err) }
	if got == nil { t.Fatal("nil response") }
	// 还需断言 currency 或 status 等业务字段，不能只检查调用成功。
}
```

- Thrift 字段编号是序列化契约的一部分，字段改名不等于编号可以复用。
- 新增 optional 字段仍需定义默认行为：兼容不是只看新代码能编译，还要覆盖旧客户端与新服务端之间的实际编解码。

## 踩坑清单

### 直接改 `kitex_gen/` 里的生成文件

```go
// kitex_gen/order/order.go
type QuoteRequest struct {
	Currency *string
}
```

现象：下次 codegen 覆盖手工改动，写在生成文件里的业务逻辑随之丢失。修复：改动放回 `idl/*.thrift`，在副本中用项目锁定版本重新生成（目录内有生成命令），再调整 handler；本仓已保留生成结果，正常测试不需要重装生成器。

### 复用字段编号

```thrift
struct QuoteRequest {
  1: required string sku
  2: required i64 quantity
  3: optional string currency
}

// 反例：后续把 3 号字段改成别的语义，例如：
// 3: optional i64 discount_bps
```

现象分两类：线型不匹配（string 改成 i64）时解码器可能跳过字段或报错，不能假定它会把 EUR 读成整数；同线型换语义（3 号复用为另一个含义的 string）则不报错、静默误读，更危险。修复：旧字段的含义与编号保持冻结，新语义一律用新编号；兼容性测试分别覆盖这两类，不能用一次新客户端调用代替验证。

### 只看 getter，不看 `IsSet*`

```go
func badCurrency(req *order.QuoteRequest) string {
	return req.GetCurrency() // 反例：nil pointer 和显式空字符串都会被压平成零值。
}
```

现象："没传"和"传了零值"被混成一件事，旧客户端本该走的默认分支被跳过，后续分支逻辑越来越乱。修复：决定默认值前先查 `IsSet*` 或字段指针，再读 getter；测试要分别断言"未传"与"显式空串"两条分支。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 17.1 新增 RPC 方法 | 在 kitex-rpc 的 IDL 增加 `string Ping()`，在副本按 README 重新生成并实现 handler | 重新生成后编译器要求 handler 实现 Ping，补齐实现与调用断言后测试通过 | `kitex-rpc/idl/echo.thrift`、`kitex-rpc/main.go` |
| 17.2 扩展 optional 字段 | 用新编号 5 增加 `optional string region`（不复用旧编号），重新生成后检查 `IsSetRegion` 并定义缺省值 | region 未传时旧请求仍成功，传入时按约定处理；未传/已传各有测试 | `kitex-idl-handler/idl/order.thrift`、`kitex-idl-handler/handler.go` |

完成后运行：

```bash
go -C labs test ./part2-backend/chapter17/kitex-idl-handler/... -count=1
go -C labs test ./part2-backend/chapter17/kitex-rpc/... -count=1
```

更多练习见各 lab 的 README。

## 链接

- [Kitex 官方文档](https://www.cloudwego.io/docs/kitex/)：IDL、代码生成和 RPC 接入的官方说明；接口细节以本仓锁定版本为准。

> 《Go 入门指南》没有直接覆盖 Kitex，但 RPC 与接口契约相关章节仍有参考价值。

- [RPC 远程调用](https://learnku.com/docs/the-way-to-go/159-rpc-remote-call/3711)
- [接口和动态类型](https://learnku.com/docs/the-way-to-go/1112-interface-and-dynamic-type/3657)
- [Go 中面向对象的总结](https://learnku.com/docs/the-way-to-go/1113-summary-object-oriented-in-go/3658)
