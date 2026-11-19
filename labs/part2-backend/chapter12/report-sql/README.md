# 报表数据库实验

需要 Go 和本机 `sqlite3` CLI。以下命令在仓库根目录运行，每次创建独立临时数据库，结束后清理。SQLite 在这里提供真实执行计划、连接间写锁和进程退出后的持久状态。

```sh
go -C labs run ./part2-backend/chapter12/report-sql plan
go -C labs run ./part2-backend/chapter12/report-sql transaction
go -C labs run ./part2-backend/chapter12/report-sql recovery
go -C labs test -race -v ./part2-backend/chapter12/report-sql
```

| 实验 | 观察与验收 |
| --- | --- |
| `plan` | 同一条按归属和时间排序的 SQL，先扫描并建立临时排序树，加入组合索引后按索引查找且省掉临时排序。两次查询返回同样的 20 个 ID。 |
| `transaction` | A 抢到写锁并更新一行，独立连接仍读到旧状态。B 在 A 未提交时遇到 `SQLITE_BUSY`，A 提交后重试只更新 0 行。随后执行一次更新并回滚，状态仍为 `running`。 |
| `recovery` | 先验证回滚的任务与待办都不留下，再提交一对任务和待办。子 worker 写完结果后以 73 退出，数据库仍显示结果存在、写入次数为 1、未确认。另一个 worker 进程恢复，确认标记变为 1，写入次数仍为 1。 |

把 `plan` 的索引列顺序改掉，再比较是否仍能免排序。将 `transaction` 的条件更新改成按 ID 无条件更新，观察第二个连接拿到写锁后是否会覆盖第一次领取。恢复实验里的待办表代表本地投递意图，结果和确认分开提交；它验证 SQLite 上可重放的本地写入，真实 Broker 的发布确认和外部副作用需要各自的验证。

SQLite 按数据库协调写入。本实验观察到的锁粒度和错误不能套到 MySQL 或 PostgreSQL。测试输出若显示缺少 sqlite3 而跳过，请安装后重新运行。原创实验，MIT。
