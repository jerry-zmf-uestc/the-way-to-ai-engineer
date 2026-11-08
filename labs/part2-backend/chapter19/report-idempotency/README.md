# 报表提交成功后丢失响应

在报表提交页，网络错误发生时，浏览器还不知道任务有没有创建。先读 [第 18 章](../../../../content/book/part2-backend/chapter19.md)。实验使用真实回环 HTTP，服务端先保存合成任务，再关闭连接，客户端收到传输错误。

从仓库根目录运行。

```sh
go -C labs run ./part2-backend/chapter19/report-idempotency
go -C labs test -race ./part2-backend/chapter19/report-idempotency -count=1 -v
```

1. 在 `exercise` 中定位首次提交、连接关闭和显式重试。用 `response_lost` 对照服务端的 `first_commit`，确认页面没有收到响应时服务端已创建任务。
2. 比较换键和沿用原键的 `retry_result` 与 `commits`。同一次操作重试沿用原键，才能找回首次提交的任务。
3. 验收以下输出，并确认两个测试通过。`TestSameKeyDifferentInput` 核对同键改参数不会覆盖原任务。

```text
reuse_key=false response_lost=true first_commit=export-1 retry_result=export-2 commits=2
reuse_key=true response_lost=true first_commit=export-1 retry_result=export-1 commits=1
```

在练习副本中让重试发送不同格式，观察同键请求返回 409。实验中的提交记录位于单进程内存，跨实例或重启后仍可去重需要持久存储和唯一约束。

原创实验，MIT。
