# 报表缓存：命中也可能读到旧版本

对应[第 16 章：缓存、读路径与一致性](../../../../content/book/part2-backend/chapter14.md)。前置是能区分数据库事实与前端查询缓存。实验只验证报表模板的旧值回填与写后读取；数据库由进程内变量代表，不涉及限流、锁或跨副本一致性。

在仓库根目录运行，使用 `labs/go.mod` 要求的 Go 版本。默认不用安装 Redis：

```sh
go -C labs run ./part2-backend/chapter14/report-cache
go -C labs test -race ./part2-backend/chapter14/report-cache -count=1 -v
```

1. 打开 [main.go](main.go) 的 `exercise`：读者先读到 v1，写者把事实改为 v2 并删缓存，延迟的读者才把 v1 写回。这个顺序由代码推进，不需要手动卡时间。
2. 比较两行输出。故障组命中旧值；修复组把**这一次写后读取**路由到事实源，普通缓存读仍可能陈旧。
3. 验收：`TestStaleCache` 通过，且观察到下面的差异。`source_reads` 包含交错开始时的那次读取。

```text
backend=memory fixed=false value=v1 hits=1 source_reads=1 stale=1
backend=memory fixed=true value=v2 hits=0 source_reads=2 stale=0
```

动手改一次：把 `exercise` 中删除缓存的位置挪到延迟回填之后。先预测故障组的 `hits`、`source_reads` 和 `stale`，再运行对照。保留原测试作为反例：原来的并发顺序仍能发生，一次调换顺序不能当成生产修复。

## 可选：用真实 Redis 重放同一时序

准备本机可信的 Redis 7.2.11 的 `redis-server` 和 `redis-cli`，将下面两处路径换成实际绝对路径。在仓库根目录用 POSIX shell 执行整个代码块。只创建临时 Unix socket，不监听 TCP 或写持久数据；退出时终止本次进程并清理临时目录。

```sh
(
  set -eu
  redis_server=/absolute/path/to/redis-server
  export REPORT_REDIS_CLI=/absolute/path/to/redis-cli
  lab_dir=$(mktemp -d /tmp/report-redis.XXXXXX)
  export REPORT_REDIS_SOCKET="$lab_dir/redis.sock"
  export REPORT_REDIS_E2E=1
  "$redis_server" --port 0 --unixsocket "$REPORT_REDIS_SOCKET" --unixsocketperm 600 \
    --save "" --appendonly no --daemonize no --maxmemory 16mb --maxclients 8 \
    > "$lab_dir/redis.log" 2>&1 &
  redis_pid=$!
  trap 'kill "$redis_pid" 2>/dev/null || :; wait "$redis_pid" 2>/dev/null || :; rm -rf "$lab_dir"' EXIT
  attempts=0
  until "$REPORT_REDIS_CLI" -s "$REPORT_REDIS_SOCKET" PING 2>/dev/null | grep -qx PONG; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 50 ]; then cat "$lab_dir/redis.log"; exit 1; fi
    sleep 0.1
  done
  go -C labs run ./part2-backend/chapter14/report-cache
  go -C labs test -race ./part2-backend/chapter14/report-cache -count=1 -v
)
```

验收：输出的 `backend` 变为 `redis`，其余字段保持一致，并看到 `TestRealRedis` 实际通过。每次运行使用新的合成 key，带 60 秒过期时间；实验不拿 CLI 子进程的耗时推断 Redis 性能。

原创实验，MIT。命令语义见 [Redis GET](https://redis.io/docs/latest/commands/get/)。
