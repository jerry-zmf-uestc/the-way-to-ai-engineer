# review-cli

工程示例，命令在仓库根目录执行。

```bash
go -C labs test ./part1-golang/chapter11/review-cli/... -count=1
go -C labs run ./part1-golang/chapter11/review-cli/cmd -item gofmt=5 -item errors=4
```

先读入口，再沿 internal 包查看实现与测试。
