# 第 14 章：事务、隔离与并发写入

系统一旦涉及库存、订单、支付、积分这些多步骤动作，就必须回答：中间某一步失败时，前面已经做过的修改怎么办。事务的难点不只是"怎么回滚"，而是"哪些操作必须一起成功或一起失败"——本章用可测试的内存实现讲清边界语义，同样的结构可以映射到真实数据库。

## 先写下任何并发下都成立的规则

报表系统规定同一团队同时最多运行一个大导出。页面先查“有没有在跑”、再提交任务，和库存里“先查剩余数量、再扣减”有同一种竞态：两个请求可以同时看到相同的旧状态。按钮置灰和进程内锁挡不住另一个服务实例。让数据库仲裁：例如给团队一条状态行，通过带条件的更新尝试领取资格，检查受影响行数，并把领取和任务写入放在同一事务。唯一约束也适合表达“同一个业务标识只能有一份记录”。

事务的原子性回答“这些写入是否一起生效”；隔离级别回答“并发事务会看见什么”。以 MySQL 8.0 的 InnoDB 为例，普通一致性读在 READ COMMITTED 下按读语句取快照，在 REPEATABLE READ 下通常沿用事务第一次一致性读的快照；锁定读是另一种操作。页面连续两次 GET 通常是两个独立请求和事务，不能据此推断两次读取共享快照。需要保护写入时，用条件更新或明确的锁定读，配合索引缩小访问范围。

两个事务以相反顺序锁定同样的两行，可能形成死锁。数据库可以回滚其中一个；服务应保持事务短、按稳定顺序加锁，确认事务被回滚后才考虑有界地重跑整笔事务。若连接在提交阶段断开，客户端可能无法确定提交结果，先按业务标识查证，或用原幂等键重试。HTTP 调用和消息发布不会随 SQL 回滚；需要可靠发送时接着看[第 15 章](chapter15.md)。

## 争用与抢不到资格是两种结果

在[双连接实验](../../../labs/part2-backend/chapter12/report-sql/README.md)中，A 取得 SQLite 写锁，把任务从 queued 改成 running，暂不提交。此时普通读仍看到 queued，B 试图写入得到 SQLITE_BUSY。A 提交后 B 再执行同一条条件更新，语句成功但影响 0 行。第一次是暂时进不去，第二次是进入后发现领取资格已经消失。只有影响 1 行的执行方才可以开始生成报表。

SQLite 的写锁粒度与生产数据库不同。这里可以验证争用与业务资格必须分开处理；如果换 MySQL 或 PostgreSQL，要重新观察等待、死锁或隔离错误。失败回滚也要有可见证据，实验最后故意写 succeeded 再回滚，另一个读看到的仍是 running。

## 写法与行为

`labs/part2-backend/chapter13/transactions` 用一个下单流程分三层：事务外壳 `TransactionalStore.WithinTx` 负责 begin / commit / rollback 语义，业务服务 `CheckoutService.PlaceOrder` 只描述业务步骤顺序，`ReserveStock`、`CreateOrder` 是事务里执行的原子操作。

### 1. 事务外壳把 begin / commit / rollback 藏在实现里

**事务语义只存在于外壳实现里，业务代码看不到 begin / commit / rollback。** 摘录自 `transactions/main.go`：内存实现先克隆状态，在副本上执行回调，成功后才用副本替换原状态。

```go
func (s *InMemoryStore) WithinTx(ctx context.Context, fn func(context.Context, StoreTx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx := &memoryTx{
		orders:    cloneOrders(s.orders),
		inventory: cloneInventory(s.inventory),
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	s.orders = tx.orders
	s.inventory = tx.inventory
	return nil
}
```

- 这个实现演示了"操作作为整体提交或放弃"的边界，没有实现数据库的并发控制、隔离级别或持久化；两个并发事务如何相互影响，需要真实数据库实验另行验证。

### 2. 业务服务只描述步骤顺序，不直接碰事务细节

**PlaceOrder 只编排"预留库存 → 创建订单"，提交与回滚是外壳的事。** 摘录自 `transactions/main.go`：

```go
func (s CheckoutService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (Order, error) {
	order, err := newPendingOrder(req)
	if err != nil {
		return Order{}, err
	}

	err = s.store.WithinTx(ctx, func(ctx context.Context, tx StoreTx) error {
		for _, line := range order.Lines {
			if err := tx.ReserveStock(ctx, line.SKU, line.Quantity); err != nil {
				return err
			}
		}
		order.Status = "created"
		return tx.CreateOrder(ctx, order)
	})
	if err != nil {
		return Order{}, err
	}
	return order, nil
}
```

- 检查回滚不能只看返回了错误：下单失败后要同时检查库存、订单集合以及已经成功处理的前一条库存记录，否则可能出现"错误是对的，状态却已经改了"。现有测试覆盖"库存预留后订单创建失败"这条路径，阅读时沿着它追踪副本与原状态。
- 重复订单返回 `ErrDuplicateOrder`，与幂等请求"再次返回首次成功结果"是不同契约；数据库中需要唯一约束支撑重复检测，不能用先查询再插入代替。网络支付和消息发送也不会随事务自动回滚，事务内尽量限制外部副作用。

运行完整示例（实现与测试见 [transactions](../../../labs/part2-backend/chapter13/transactions/README.md)，源码 [main.go](../../../labs/part2-backend/chapter13/transactions/main.go)）：

```bash
go -C labs run ./part2-backend/chapter13/transactions
```

```text
transaction committed: ORD-1001 customer=CUST-7 status=created total=$42.97 lines=2 remaining=SKU-101:6,SKU-102:2
```

## 踩坑清单

### 在业务函数里手写 begin / commit / rollback

```go
func (svc *CheckoutService) PlaceOrder(ctx context.Context, req Request) error {
	tx := db.Begin() // 反例：事务细节直接渗进业务层。
	// ...
	tx.Commit()
	return nil
}
```

现象：service 与具体数据库 API 强耦合，测试也更难做。修复：把 begin / commit / rollback 收回事务适配器，业务服务通过 `WithinTx` 组织步骤；用同一业务测试替换存储实现复验，成功路径不丢步骤，失败路径不提交部分状态。

### 只验证成功路径，不验证失败回滚

```go
func TestPlaceOrder(t *testing.T) {
	// 反例：只断言订单创建成功，却没检查库存不足、重复订单时是否真正回滚。
}
```

现象：事务的作用主要在失败路径上体现，只测成功等于没验证回滚。修复：制造库存不足和重复订单号，记录调用前后的库存与订单数量，断言错误可识别且失败前后状态一致（内存测试不证明数据库隔离级别）。

<a id="练习"></a>

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 13.1 制造库存不足 | 参照 `TestPlaceOrderRollsBackAllReservations`，给请求加一行 sku-102、Quantity=999，先保存库存快照再调用 `PlaceOrder` | 返回库存不足错误，订单不增加；逐项比较库存与订单集合，证明前一项成功预留也被回滚 | `transactions/main_test.go` |
| 13.2 重复提交订单号 | 首次 `PlaceOrder` 成功后记录库存，再以同一订单号调用（不忽略第一次调用的错误） | 第二次 `errors.Is(err, ErrDuplicateOrder)` 为 true，订单数量与库存不再变化 | `transactions/main_test.go` |
| 13.3 推演并发领取 | 让两个请求先后读到空闲，再分别模拟无条件写与带条件的更新 | 能找出竞争窗口，说明为何只有更新成功者才可写入新任务 | 本章报表案例 |

完成后运行：

```bash
go -C labs test ./part2-backend/chapter13/transactions/... -count=1
```

更多练习（如标出换成 GORM 后的数据库替换位置）见各 lab 的 README。

## 链接

- [MySQL InnoDB 事务与锁](https://dev.mysql.com/doc/refman/8.0/en/innodb-locking-transaction-model.html)：区分一致性读、隔离级别、锁和死锁。
- [Go 官方事务教程](https://go.dev/doc/database/execute-transactions)：BeginTx、Commit、Rollback 的官方写法，对照内存模拟与真实数据库事务的差别。
- 《The Way to Go》完整应用实例：[介绍](https://learnku.com/docs/the-way-to-go/191-introduction/3741)、[Web 服务器前端](https://learnku.com/docs/the-way-to-go/194-our-user-interface-a-web-server-frontend/3744)、[使用 JSON 作为存储](https://learnku.com/docs/the-way-to-go/197-using-json-for-storage/3747)、[总结与增强](https://learnku.com/docs/the-way-to-go/1910-summary-and-enhancements/3750)：一个真实小应用的存储层取舍，可作对照阅读。

[返回全书目录](../README.md) · [上一篇：数据建模、SQL 与索引](chapter12.md) · [下一篇：任务队列、投递与恢复](chapter15.md)
