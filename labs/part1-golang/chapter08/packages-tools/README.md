# 示例： packages tools

## Goal

理解包声明与包注释、导出/未导出命名约定，以及匿名导入（blank import）通过 init() 完成注册的机制。

## Concepts

- 每个源文件以 `package` 声明开头；包注释写在 package 语句前，说明整个包的用途。导出标识符首字母大写，未导出标识符首字母小写。
- `import _ "image/png"` 匿名导入只为执行 png 包的 init()，其中调用 `image.RegisterFormat` 注册 PNG 解码器；之后 `image.Decode` 才能识别 PNG。去掉该行会报 `image: unknown format`。
- 标准库中 image 包本身不默认注册任何格式：jpeg、gif 与 png 一样，需要各自匿名导入 `image/jpeg`、`image/gif` 后才注册（已用 Go 1.26 核实：只导入 png 时，JPEG/GIF 魔数返回 `image.ErrFormat`）。
- 源码内嵌 67 字节的最小合法 1x1 PNG，用魔数探测代替读取真实图片文件，保证离线可测。

## Run

```bash
gofmt -l labs/part1-golang/chapter08/packages-tools
go -C labs vet ./part1-golang/chapter08/packages-tools
go -C labs test ./part1-golang/chapter08/packages-tools -race -count=1
go -C labs run ./part1-golang/chapter08/packages-tools
```

Expected output:

```text
内嵌图片格式: png
图片尺寸: 1 x 1
已注册格式: [png]
```

## Code Map

- [main.go](main.go): 包注释、匿名导入注册机制，以及 `decodeFormat` / `registeredFormats` 两个可测试的探测函数。
- [main_test.go](main_test.go): 断言内嵌 PNG 被识别为 "png"，并断言只导入 png 时 JPEG/GIF 魔数报 `image.ErrFormat`，形成注册前后的对比。

## Exercise

在临时副本中再添加 `import _ "image/jpeg"` 和 `import _ "image/gif"`，重跑测试观察 `TestJPEGNotRegistered` 等断言失败，然后把 `registeredFormats` 的预期更新为 `[png jpeg gif]` 使测试恢复通过，体会注册机制对全局状态的影响。练习与验收见 [第 8 章](../../../../content/book/part1-golang/chapter08.md)。

## Reading

返回 [Chapter 8](../../../../content/book/part1-golang/chapter08.md)。参考 [Go 语言圣经 · 第 10 章 包和工具](https://golang-china.github.io/gopl-zh/ch10/ch10.html)（包声明、导入声明与匿名导入的 jpeg 示例）。License: MIT。
