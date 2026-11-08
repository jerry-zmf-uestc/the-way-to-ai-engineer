# 报表队列：结果已提交，ACK 却丢了

对应[第 15 章：消息队列与异步任务](../../../../content/book/part2-backend/chapter15.md)。从前端任务进度出发：页面只发起了一次导出，服务端也可能收到两次投递。这个 Go Lab 用进程内 Broker 和结果库模型复现该窗口，不需要启动 RabbitMQ；它不验证真实 Broker 的持久化、连接或 Outbox 发布。

从仓库根目录运行：

```sh
go -C labs run ./part2-backend/chapter15/report-queue
go -C labs test -race ./part2-backend/chapter15/report-queue -count=1 -v
```

1. 看 [main.go](main.go) 的 `exercise`：第一次投递后先提交报表，再故意省略 ACK；重新投递携带同一个事件 ID。
2. 对照两组 `deliveries`、`acks`、`artifacts`、`pending`。修复时把事件 ID 和报表结果作为一次提交，用事件 ID 识别已经处理的工作。
3. 验收：两组都投递两次、成功 ACK 一次，最终无待确认消息；报表产物从两份变成一份。测试还确认不同事件不会被误去重。

```text
fixed=false deliveries=2 acks=1 artifacts=2 pending=0
fixed=true deliveries=2 acks=1 artifacts=1 pending=0
```

动手改一次：在练习副本增加“先 ACK，提交前失败”分支，画出哪些状态还能恢复。新增断言：此时产物是 0，Broker 也没有消息可以重发。这是需要避免的结果，不能靠继续重试消费解决。

原创实验，MIT。真实确认语义参考 [RabbitMQ 的消费确认与发布确认](https://www.rabbitmq.com/docs/confirms)。
