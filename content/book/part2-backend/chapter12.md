# 第 13 章：数据建模、SQL 与索引

接口处理的是请求，数据库保存的是长期状态；两者之间需要明确的数据访问约定：怎样表示商品、怎样区分查无记录和查询失败、怎样避免业务代码依赖连接细节。相关 HTTP 场景见 [HTTP 与数据访问](../part1-golang/chapter10.md)。

## 先从页面查询设计表和索引

报表页面的筛选条件看起来像一个 TypeScript 对象，但存储模型要回答三件事：一行代表什么、谁拥有它、哪些查询要在数据量增长后仍然快。把一行定义为一次导出任务，主键定位任务，团队字段标识归属，状态和创建时间支持列表。页面 DTO 可以把时间格式化、隐藏内部重试次数；表结构则保留恢复任务需要的事实。租户从认证上下文取得；列表和详情查询都要带上授权范围，不能仅凭页面传来的任务 ID 访问。

假设常见查询是“某团队最近 20 个成功任务”，按创建时间和任务 ID 倒序。适合评估的组合索引是 (team_id, status, created_at, id)：先用等值条件定位团队和状态，再沿索引读取按时间和 ID 排序的一段记录。ID 是时间相同情况下的排序锚点。若页面去掉状态过滤，索引在团队内部先按状态分组，未必还能直接给出想要的时间顺序；要根据真实访问频率和执行计划决定是否另加索引。索引要占存储，每次写入也要维护。

在隔离数据库上先用 EXPLAIN 看访问方式、选择的索引和估算扫描量，再用代表性数据观察实际执行；少量样本上的全表扫描不一定是错误。列表翻到深页时，可让游标带上上一页最后一条记录的 (created_at, id)，用它做下一页边界。游标不能代替团队过滤；如果需要跨页看到完全一致的快照，还要定义快照或版本策略。

## 同一条 SQL 为什么不再排序

[真实 SQL 实验](../../../labs/part2-backend/chapter12/report-sql/README.md)用同一份 20,000 行合成数据执行两次查询，只在中间加入组合索引。查询限定 owner，按 created_at 和 id 倒序取前 20 行。无索引时，SQLite 扫描候选行并为 ORDER BY 建临时树；有索引时，owner 先缩小搜索范围，后两列已经给出需要的顺序。因为只返回索引包含的 id，计划还显示覆盖索引。

执行计划要与结果一起比较。如果加索引后快了却改变了分页顺序，业务仍然回归。游标需保存 created_at 和 id，时间相同的行由 id 打破平局。拿掉 owner 条件或换成按完成时间排序，不能沿用原结论。索引也增加每次写入的维护成本，因此每个新增索引都应能对应一条实际查询。

## 模型说明数据约束

**模型 tag 表达预期的字段约束，但 Go 编译器不会替数据库执行唯一检查。**

摘录自 [gorm-mysql/main.go](../../../labs/part2-backend/chapter12/gorm-mysql/main.go)：

```go
type Product struct {
	ID         uint       `gorm:"primaryKey"`
	SKU        string     `gorm:"size:64;uniqueIndex;not null"`
	Name       string     `gorm:"size:128;not null"`
	PriceCents int64      `gorm:"not null;check:price_cents >= 0"`
	Active     bool       `gorm:"not null;default:true"`
	CreatedAt  time.Time  `gorm:"not null"`
	UpdatedAt  time.Time  `gorm:"not null"`
	DeletedAt  *time.Time `gorm:"index"`
}
```

- 价格用整数分表示，避免把显示格式和存储单位混在一起；Summary 将分换算为展示文字，后续计算仍用整数，并检查乘法、累加是否溢出。
- 这个 Lab 不导入 GORM、不连接 MySQL，只展示字段与配置形状：不能用它的单元测试证明索引已经建立，也不能把 DeletedAt 指针等同于 GORM 的软删除插件行为。

## 配置集中校验，输出先脱敏

**MySQLConfig 把连接参数集中在一个结构里，Validate 先做边界校验再构造 DSN；日志只输出脱敏后的 DSN。**

Validate 先拒绝空用户、空主机、空数据库及超范围端口，再构造 DSN；驱动是否接受该 DSN 是接入真实数据库时的独立步骤，本例不验证。示例的固定密码仅用于本地字符串测试，不应照搬为部署凭据。

```bash
go -C labs run ./part2-backend/chapter12/gorm-mysql
```

```text
gorm model=SKU-001 CloudWeGo Hoodie active $89.99 dialect=mysql table=products disable_fk_migration=true singular_table=false dsn=app:%2A%2A%2A@tcp(127.0.0.1:3306)/commerce?charset=utf8mb4&loc=Local&parseTime=true
```

DSN 中的 `%2A%2A%2A` 是转义后的脱敏占位符，不是可用密码。

## Repository 隔开业务与存储

**业务通过接口访问存储，不需要知道数据来自 map 还是数据库。**

[repository-pattern](../../../labs/part2-backend/chapter12/repository-pattern/README.md) 定义数据访问接口：内存实现保存商品，CatalogService 负责添加商品、补货与查询有效目录。阅读时分别检查三件事：错误是否可用 errors.Is 识别，返回的数据是否与内部状态意外共享，Context 取消后是否仍修改状态。内存实现可以验证这些行为；并发控制、SQL 查询计划、连接池和数据库隔离需要另外测试。

```bash
go -C labs run ./part2-backend/chapter12/repository-pattern
```

```text
repository first=SKU-101 CloudWeGo Mug active $19.99 stock=12 listed=2 total_stock=15
```

## 踩坑清单

### 日志中出现连接密码

```go
cfg := DefaultMySQLConfig()
cfg.Password = "exercise-secret"
log.Println(cfg.DSN()) // 反例：完整 DSN 含明文密码，直接进日志
```

现象：日志泄漏连接密码。修复：改用 RedactedDSN 并检查它返回的错误，同时断言脱敏结果不含密码、原配置的 Password 未被改成占位符（含空密码的情况）。验证：

```bash
go -C labs test ./part2-backend/chapter12/gorm-mysql -run TestRedactedDSNDoesNotExposePassword -v -count=1
```

### 取消请求后仍然修改仓库

```go
// 反例：实现忽略 ctx，失败请求仍可能写入商品或改变库存
func (r *Repo) Restock(ctx context.Context, sku string, n int) error {
	p := r.items[sku]
	p.Stock += n // 没有先检查 ctx.Done()
	return nil
}
```

现象：已取消的请求仍修改了状态。修复：在修改状态之前检查取消信号，把错误交给 service，不要捕获后返回一份空的“成功结果”。验证时先记录调用前的商品和库存，调用后确认状态未变：

```bash
go -C labs test ./part2-backend/chapter12/repository-pattern -run TestRepositoryHonorsCancelledContext -v -count=1
```

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 12.1 检查端口边界 | 在 MySQLConfig.Validate 上逐一测试 0、1、65535、65536 四个端口值并断言错误，不能只调用函数不断言 | 0 和 65536 被拒绝；1 和 65535 通过端口检查 | `gorm-mysql/main_test.go` |
| 12.2 验证查无记录 | 用 `FindBySKU(ctx, "MISSING")` 断言错误，再查询列表确认读失败没有创建空商品 | `errors.Is(err, ErrNotFound)` 为 true，仓库状态不变；不与 `context.Canceled` 混成同一个错误 | `repository-pattern/main_test.go` |
| 12.3 设计报表列表 | 以团队、状态和创建时间为筛选条件，设计稳定排序、游标与索引；再去掉状态条件 | 能说明访问范围、同秒任务如何排序，以及去掉状态后为什么需要重查执行计划 | 本章报表案例 |

完成后运行：

```bash
go -C labs test ./part2-backend/chapter12/... -count=1
```

更多练习（日志脱敏断言等）见各 lab 的 README。

## 链接

- [MySQL EXPLAIN](https://dev.mysql.com/doc/refman/8.0/en/explain.html)：读懂数据库选择的访问方式与索引。
- [Go 官方数据库教程](https://go.dev/doc/database/execute-transactions)：对照真实 sql.Tx 的提交与回滚，明确本章内存仓库没有验证的数据库行为。

[返回全书目录](../README.md) · [上一篇：请求边界与任务 API](chapter16.md) · [下一篇：事务、隔离与并发写入](chapter13.md)
