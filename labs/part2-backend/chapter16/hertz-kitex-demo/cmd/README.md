# 示例： hertz kitex demo/cmd

这是 `labs/part2-backend/chapter16/hertz-kitex-demo` 的命令行入口页。它只保留启动 wiring，把主要实现放进 `internal/hertzkitexdemo/`，用于展示“学习版微服务 demo 如何落到 `cmd + internal` 分层结构”。

## 运行

```bash
go -C labs test ./part2-backend/chapter16/hertz-kitex-demo/cmd -race -count=1
go -C labs run ./part2-backend/chapter16/hertz-kitex-demo/cmd
```

## 源码入口

- [main.go](main.go)：装配应用并启动 demo
- [main_test.go](main_test.go)：入口层最小回归测试

## 相关练习

- [第 16 章练习](../../../../../book/part2-backend/chapter16.md#练习)：回到正文里的练习区，按伪代码和验收标准继续
  扩展这个 topic。

## 继续阅读

- [返回第 16 章索引](../../../../../book/part2-backend/chapter16.md)
- [查看仓库级结构说明](../../../../../ARCHITECTURE.md)
