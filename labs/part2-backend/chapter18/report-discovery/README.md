# 报表服务发现与实例下线

在报表进度页，网关要向一个可用实例查询同一任务。先读 [第 17 章](../../../../content/book/part2-backend/chapter17.md)。实验用两个真实回环 HTTP 实例和进程内发现快照，模拟发现结果更新。

从仓库根目录运行。

```sh
go -C labs run ./part2-backend/chapter18/report-discovery
go -C labs test -race ./part2-backend/chapter18/report-discovery -count=1 -v
```

1. 打开 `exercise`，找出缓存的 A 地址。首次请求成功后，将 A 从就绪快照移除并关闭 A 的监听。
2. 对照 `target`、`served`、`success` 和 `failures`。旧地址调用失败，读取更新后的快照才会把下一次请求送到 B。
3. 验收三行输出，并确认 `TestInstanceDown` 通过。

```text
phase=before target=A served="A" success=1 failures=0
phase=pinned target=A served="" success=0 failures=1
phase=refreshed target=B served="B" success=1 failures=0
```

在练习副本中保留已关闭的 A、移除 B，观察发现列表仍有地址但请求失败。恢复原快照并重跑测试。实际注册中心的传播延迟、连接排空和负载均衡需要单独验证。

原创实验，MIT。
