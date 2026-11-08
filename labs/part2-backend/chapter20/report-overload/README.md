# 报表过载：用 503 保护执行容量

对应[第 19 章：并发、背压与过载保护](../../../../content/book/part2-backend/chapter20.md)。前置是能读懂 Go handler 和 channel。实验启动临时回环 HTTP 服务；用通道把执行中的导出停住，让所有请求先到达接纳点，不靠调大 sleep 制造过载。

从仓库根目录运行：

```sh
go -C labs run ./part2-backend/chapter20/report-overload
go -C labs test -race ./part2-backend/chapter20/report-overload -count=1 -v
```

1. 读 [main.go](main.go) 的 `exercise`。12 个客户端同时提交。用 4 个槽位作为下游预算：故障组忽略预算，修复组没有空槽就立即返回 503。
2. 比较 `offered`、`success`、`rejected` 和 `peak_inflight`。故障组没有报错，却用了预算的三倍；修复组用可识别的拒绝守住上限。
3. 验收：修复组只接纳 4 个，其余 8 个返回 503。放行在途任务后，再发一次请求，应得到 204，证明容量能释放。确认 `TestBurstAdmissionAndRecovery` 通过。

```text
limit=0 offered=12 success=12 rejected=0 peak_inflight=12 recovery_status=204
limit=4 offered=12 success=4 rejected=8 peak_inflight=4 recovery_status=204
```

动手改一次：先把 `capacity` 改为 2，预测峰值和拒绝数，再运行。思考页面遇到 503 应如何提示，以及为什么立即重试 10 次会让系统更难恢复。实验测的是接纳和恢复；阻塞的任务是合成负载，不能用这些数字估算线上吞吐或 p99。

原创实验，MIT。进一步定位真实瓶颈见 [Go 诊断文档](https://go.dev/doc/diagnostics)。

## 固定计划到达率的对照

```sh
go -C labs run ./part2-backend/chapter20/report-overload compare
```

每组发 60 个请求，计划每 2ms 到达一个，2 个执行槽，每次模拟 12ms 工作。比较 60、4、0 个等待位置，核对成功与拒绝之和始终是 60，过载卸载后 recovery 是 204。记录成功请求 p95、拒绝百分比、完成速率和最大排队时间。时间取决于机器调度，用相同机器重复观察趋势；完成速率使用从第一批发出到全部结束的窗口。

如需核对「慢在等什么」，采集确定性突发测试的阻塞剖面。比较 handler 的累积等待和客户端 HTTP 等待，再回源码定位释放信号。阻塞时间可以累加多个 goroutine，不能直接当成单次请求延迟。

```sh
profile_dir=$(mktemp -d /tmp/report-overload-profile.XXXXXX)
go -C labs test ./part2-backend/chapter20/report-overload -run '^TestBurstAdmissionAndRecovery$' -count=1 \
  -o "$profile_dir/overload.test" -blockprofile "$profile_dir/block.out"
go tool pprof -top "$profile_dir/overload.test" "$profile_dir/block.out"
```
