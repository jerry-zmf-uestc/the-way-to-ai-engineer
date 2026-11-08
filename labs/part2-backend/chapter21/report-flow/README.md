# Part 2 综合实验：从提交到报表结果

跑一条前端可以直接调用的报表导出 API。先用正常模式走通提交和轮询，再一次只开启一种故障。每个模式启动一个新进程，任务和指标从零开始。

建议先读 [后端学习路线](../../../../content/book/README.md#backend-roadmap)。本实验把缓存、消息投递、实例发现、幂等、过载和积压的观察点放进同一条任务链。HTTP 监听和断开连接在本机真实发生。默认使用进程内模型。通过 `-db` 可把任务、幂等键、结果和投递待办存入 SQLite，并在重启后继续执行。

## 先跑正常链路

从仓库根目录打开第一个终端，用 `labs/go.mod` 要求的 Go 版本运行。

```sh
go -C labs run ./part2-backend/chapter21/report-flow -port 8080 -fault none
```

启动后，在浏览器打开 [本地报表导出练习台](http://127.0.0.1:8080/)。直接点「提交新任务」，看任务列表自动更新，点任务可查看对应 CSV。右侧指标持续刷新。启动参数指定的模式会显示在页面顶部，并给出本轮操作和验收提示。

| 浏览器练习 | 点什么与看什么 |
| --- | --- |
| `stale-cache` | 提交一次，等 CSV 出现，核对「发现旧快照」增加。 |
| `lost-ack` | 提交一次，核对投递为 2，ACK 丢失与重复投递各为 1，完成数仍为 1。 |
| `instance-down` | 提交一次，核对执行实例 B 和「实例不可用」计数，观察服务端切换后的 CSV。 |
| `lost-response` | 首次提交显示结果未知。先看接纳数，再点「用原键重试」，核对沿用原键计数与原任务 CSV。 |
| `overload` | 连续提交三次，看到第三次的 503 后点击「恢复消费」。等队列排空，再点「用原键重试」，核对第三份 CSV。 |
| `backlog` | 提交三次，记录两次最久等待时间。点击「恢复消费」后逐个查看结果，核对队列深度和等待时间归零。 |

每种故障用一个干净进程，从表中任选一行，将启动命令的 `-fault none` 换成对应模式。浏览器可保持打开，检测到服务重启后会清空上一轮页面任务和重试键。提交结果未知时，在当前页面用原键重试。浏览器模式在收到 202 响应头后截断响应体，避免浏览器自动重发完全没有响应的请求。页面拿到完整任务 ID 才开始轮询。

## 用命令行对照 HTTP

在第二个终端定义三个命令。每次主动发起新导出时用新键，同一次提交重试沿用原键。

```sh
base=http://127.0.0.1:8080
submit() {
  curl -sS -i --max-time 3 -X POST "$base/exports" \
    -H "Idempotency-Key: $1" -H 'Content-Type: application/json' \
    --data '{"format":"csv"}'
}
query() { curl -sS --max-time 3 "$base/exports/$1"; }
metrics() { curl -sS --max-time 3 "$base/metrics"; }
```

1. 运行 `submit one`。确认 HTTP 202，响应含 `id=export-1` 和 `status=queued`。
2. 运行 `query export-1`。如果任务还在 `queued` 或 `running`，等片刻再查。最终 HTTP 200，`status=succeeded`、`instance=A`，`result` 含 CSV 表头和 `export-1,42`。
3. 运行 `metrics`。确认 `accepted=1`、`deliveries=1`、`completed=1`、`depth=0`。到服务终端按 `job=export-1` 对照 `enqueued`、`running` 和 `completed`。

前端可以将 202 显示为已接收，根据查询中的 `status` 更新进度，仅在 `succeeded` 时展示结果。`GET /metrics` 返回 JSON 快照，其中 `depth` 是等待首次执行的任务数，`oldest_wait_ms` 是其中等待最久的任务已等了多久。`deliveries` 包括重投递。完成日志里的 `queue_wait_ms` 从入队计到开始执行。

## 逐个注入故障

每次先在服务终端按 Ctrl-C，再重跑启动命令，把 `-fault none` 的 `none` 换成表中模式。第二个终端继续用之前定义的命令。

| 模式 | 操作 | 验收 |
| --- | --- | --- |
| `stale-cache` | `submit one`，查 `export-1` 直到完成，再查指标 | 完成结果里有 CSV，`cache_stale=1`。日志出现 `stale_cache_discarded`，说明完成后的首次查询发现了旧的 running 快照。 |
| `lost-ack` | `submit one`，查 `export-1` 直到完成，再查指标，必要时再读一次指标等待重投递 | `deliveries=2`、`ack_drops=1`、`duplicates=1`、`completed=1`。同一任务经过消费者两次，结果只提交一次。 |
| `instance-down` | `submit one`，查 `export-1` 直到完成 | `instance_failures=1`，日志先记录 A 不可用，最终结果的 `instance=B`。 |
| `lost-response` | 第一次 `submit one` 应出现 curl 传输错误。先读指标，再运行一次 `submit one` 并查 `export-1` | 第一次请求后已有 `accepted=1` 和 `response_drops=1`。第二次返回原 ID，最终 `reused=1`、`completed=1`。在另一次干净运行中改用新键重试，可观察到第二个任务。 |
| `overload` | 依次 `submit one`、`submit two`、`submit three`。检查指标后执行下方恢复命令。查前两个结果，再用同一个 `three` 提交并查询第三个结果 | 第三次提交先收到 503，`depth=2`、`rejected=1`。恢复后该键返回 202 和 `export-3`，最终 `completed=3`、`depth=0`。 |
| `backlog` | 依次提交 `one`、`two`、`three`，间隔片刻读两次指标，再执行恢复命令并查询三个结果 | 暂停期间 `depth=3`、`completed=0`，`oldest_wait_ms` 随时间增长。恢复后 `peak_depth=3`、`completed=3`、`depth=0`、`oldest_wait_ms=0`。 |

恢复命令只用于让暂停的消费者继续运行，成功返回 204。

```sh
curl -sS -i --max-time 3 -X POST "$base/lab/resume"
```

## 用浏览器验证 SQLite 重启恢复

需要本机 `sqlite3` CLI。每组用独立临时数据库，同一个文件一次只启动一个实验服务。以下命令在仓库根目录执行，重启时保留同一个终端变量。

```sh
report_dir=$(mktemp -d /tmp/report-flow-sqlite.XXXXXX)
db="$report_dir/reports.db"
go -C labs run ./part2-backend/chapter21/report-flow -port 8080 -db "$db" -fault crash-before-result
```

1. 打开 [本地练习台](http://127.0.0.1:8080/)，点「提交新任务」。记下页面返回的任务 ID，空数据库的第一个任务是 `export-1`。
2. 点「注入故障」。终端会记录 `injected_crash`，worker 让整个实验进程以 73 退出。浏览器保持打开。
3. 在同一终端执行下面的查询。第一组应得到 `export-1|0|0`。最后两列分别是 CSV 持久写入次数和待办确认状态。

```sh
sqlite3 "$db" 'SELECT j.id,j.result_writes,o.acked FROM report_flow_jobs j JOIN report_flow_outbox o USING(id);'
```

4. 用原数据库和同一个端口启动服务，将模式切回 `none`。

```sh
go -C labs run ./part2-backend/chapter21/report-flow -port 8080 -db "$db" -fault none
```

5. 在页面的「按原任务 ID 查询」输入刚才的 ID 并查询。验收 CSV 包含 `export-1,42`，任务显示「结果写入 1 次」，启动时恢复的待办为 1。待 worker 处理后再执行 SQL，应得到 `export-1|1|1`。
6. 用一个新的临时目录重做，将启动参数改为 `-fault crash-after-result`。退出后应看到 `export-1|1|0`。同库重启后应看到 `export-1|1|1`，浏览器显示原 CSV 与「结果写入 1 次」，重复投递为 1。此时恢复只补确认。

任务与待办在同一事务内提交。恢复时读待办和结果记录，已提交结果的任务只确认。结果写入次数和 CSV 在同一条条件更新里落库，SQLite 查询可以在两个故障窗口分别核对它们。执行实例和 Redis 缓存仍由进程内模型提供，待办由 SQLite 表保存。

`go -C labs test -race ./part2-backend/chapter21/report-flow -run TestSQLiteRecoveryAcrossProcesses -v` 使用两个真实进程窗口验证查询、原键重试和磁盘状态。

## 自动验收

```sh
go -C labs test -race ./part2-backend/chapter21/report-flow -count=1 -v
```

确认 `TestReportFlow` 的正常模式和六个故障子测试全部通过。测试实际通过回环 HTTP 提交和查询，检查故障时的响应与指标，再逐个核对完成任务的 ID、执行实例和 CSV 内容。

回到单项 Lab 对照触发条件。[Redis 缓存](../../chapter14/report-cache/README.md) · [队列确认](../../chapter15/report-queue/README.md) · [实例下线](../../chapter18/report-discovery/README.md) · [响应丢失](../../chapter19/report-idempotency/README.md) · [并发保护](../../chapter20/report-overload/README.md) · [积压观测](../report-backlog/README.md)

动手练习可以从 backlog 开始。保持接口不变，在练习副本中调整接纳容量，预测第几个提交会返回 503。恢复后用同一个键重试，核对拒绝没有预先占用任务 ID 或幂等键。

原创实验，MIT。

## 检查资源归属与发布摘流量

浏览器默认使用教学身份 alice。用原有 `submit one` 取得任务 ID 后，给查询增加 `X-Lab-User: bob`，应收到 404；去掉该头，应返回 Alice 的记录。再用 Bob 提交相同幂等键，应返回不同 ID。头字段仅用于本地切换两份固定身份，生产环境由认证组件提供身份。

发布练习从干净的 `-fault backlog` 进程开始。运行 `submit one`，确认排队后执行：

```sh
curl -sS -i "$base/readyz"
curl -sS -i -X POST "$base/lab/drain"
curl -sS -i "$base/readyz"
```

第一次 readiness 为 204，摘流量后为 503。此时 `submit two` 返回 503，`submit one` 仍返回原任务。轮询原任务直至拿到 CSV，确认队列排空后按 Ctrl-C。再次启动并查询旧 ID，应返回 404。然后执行 [SQLite 重启实验](../../chapter12/report-sql/README.md)，对照持久任务恢复时必须读取哪些记录。

`TestExportOwnership` 和 `TestDrainKeepsAcceptedWorkQueryable` 检查归属、摘流量与旧任务结果。
