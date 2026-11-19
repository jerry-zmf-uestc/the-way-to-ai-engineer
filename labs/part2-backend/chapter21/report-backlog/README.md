# 报表任务积压与恢复

报表页面一直显示处理中，worker 却每秒都有成功记录。先读 [第 20 章](../../../../content/book/part2-backend/chapter18.md)。实验用虚拟秒推进内存 FIFO 队列，每秒先接收任务，再完成最多一份报表。

从仓库根目录运行。

```sh
go -C labs run ./part2-backend/chapter21/report-backlog
go -C labs test -race ./part2-backend/chapter21/report-backlog -count=1 -v
```

1. 前三秒每秒进入 2 个任务，完成速率维持在 1。对照 `arrivals`、`completed`、`depth` 和 `oldest_seconds`。成功记录持续出现时，队列深度仍从 1 涨到 3。
2. 第 3 秒起停止提交，观察队列排空。深度开始下降后，最老等待时间仍可暂时上升。这里 `oldest_seconds` 只统计本秒末仍在队列中的任务，空队列记为 0。
3. 验收 `TestBacklogBuildsAndDrains` 通过。沿同一个任务 ID 查日志，`export-6` 在第 2 秒入队、第 5 秒完成，`queue_wait_seconds=3`，末尾队列深度为 0。

```text
metric tick=2 arrivals=2 completed=1 depth=3 oldest_seconds=1
metric tick=4 arrivals=0 completed=1 depth=1 oldest_seconds=2
event=completed tick=5 job=export-6 queue_wait_seconds=3
metric tick=5 arrivals=0 completed=1 depth=0 oldest_seconds=0
```

在练习副本里把到达量改成每秒 1 个，预测深度和等待时间再运行。原测试锁定积压场景，修改负载后可以给平衡场景另写断言。实验用进程内事件和计数演示排障，接入生产指标或跨服务追踪时需要另行验证采集链路。

原创实验，MIT。
