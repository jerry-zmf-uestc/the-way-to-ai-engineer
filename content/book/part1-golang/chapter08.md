# 第 8 章：包和工具

配套实验：[packages-tools](../../../labs/part1-golang/chapter08/packages-tools/README.md)。

第 7 章的接口、泛型和反射把"一个包内部"的抽象推到了极致；真实项目的麻烦恰恰从代码溢出单个包开始。一个起步时只有 main.go 的服务，半年后可能长成四十个文件挤在同一个目录：没人敢动 util.go，因为它什么都引用、什么也都引用它；而在另一个角落，有人删掉了一行"从来没被调用过"的 `_ "image/png"`，编译照常通过，部署后所有解码图片的请求开始返回 `image: unknown format`。

本章回答两个问题：Go 怎样用"包"这道边界控制可见性与依赖方向，以及 `go` 工具链怎样把包管理、构建、测试、文档收成一条命令。上面两个故障——util 包的依赖纠缠和匿名导入的"隐形副作用"——都将在本章得到解释和复现。

## 8.1 包用可见性建立编译边界

**包 = 名字空间 + 封装**：每个包独立命名，允许短而不冲突的名字；未导出标识符只在包内可见，维护者可以调整内部实现而不影响外部用户。

可见性规则简单到只有一条：**首字母大写导出，小写不导出**。Python 用 `_name` 前缀约定、靠 review 自觉维护；ES module 靠显式 `export` 清单。Go 把可见性编码进标识符本身，由编译器强制执行——`strings.Builder` 外部可用，而 `strings` 包内部的小写函数，外部连引用都无法通过编译。这像办公室里的白板：屋里的人随便写，门外的人看不见，屋主随时能擦掉重画而不必发公告。

封装带来的实际收益是**重构自由**：只要导出成员的签名不变，包内实现可以任意推翻重写。这也是 Go 标准库敢做兼容性承诺的底气之一——`bytes.Buffer` 的内部字段全部未导出，用户根本无从依赖它们。

Go 编译快，部分来自三个语言层面的设计：

1. 所有导入必须在每个文件开头显式声明，编译器无需分析整个文件就能确定依赖。
2. **禁止包的循环依赖**，依赖关系形成有向无环图，每个包可独立（甚至并发）编译。
3. 编译产物记录依赖关系，编译时只需读取直接导入包的信息。

什么时候该拆出一个新包？判断依据是依赖方向与封装需求，不是文件数量：

| 场景 | 拆不拆 | 理由 |
|---|---|---|
| 一块独立的领域能力（温度换算、图像处理） | ✅ 拆 | 名字空间让短名字可用，封装让实现可替换 |
| 需要向其他模块隐藏实现细节 | ✅ 拆 | 未导出标识符是 Go 唯一的硬封装手段 |
| 被多个 main 命令复用的逻辑 | ✅ 拆 | 库包承载共享逻辑，`cmd/` 下各 main 只做装配 |
| 几十行、只被一处使用的 helper | ❌ 不拆 | 包边界的认知成本大于收益 |
| 两个候选包互相需要对方的类型 | ❌ 先合并 | 循环依赖会被编译器直接拒绝——这是边界划错的信号，不是语法问题 |

## 8.2 导入路径定位包，package 声明决定名字

导入路径和包声明是两个东西：**导入路径全局唯一地定位包（它是地址）；包声明决定被导入时的默认标识符（它是门牌上的名字）**。惯例是默认包名 = 导入路径的最后一段：

```go
import (
	"math/rand"   // 导入路径 math/rand，默认包名 rand
	"crypto/rand" // 也是 rand——同名时需要重命名（见 8.3）
)
```

三种例外：

- **main 包**：生成可执行程序，导入路径无关紧要；每个目录只含一个包，可执行入口常放在 `cmd/` 子目录。
- **`xxx_test` 包**：`_test.go` 文件可声明为 `package xxx_test`，用于避免测试代码造成循环导入。
- **带版本号的导入路径**：如 `gopkg.in/yaml.v2`，包名仍是 `yaml`。Go modules 把这条规则正式化：主版本号 v2 及以上的模块，导入路径必须带 `/vN` 后缀（如 `github.com/foo/bar/v2`），同一代码库的不同主版本因而可以并存。

发布包时，导入路径应以组织域名为前缀保证全球唯一（如 `github.com/go-sql-driver/mysql`）。现代 Go 用 **module**（go.mod）管理依赖与版本——本仓库的 `labs/go.mod` 声明了实验代码的 module 路径，所有 Go Lab 都挂在它下面；早期的 GOPATH 工作区模式已被 module 取代，了解历史即可。

## 8.3 导入名只影响当前文件

**同时导入同名包时必须重命名其一**；导入按组分块，`gofmt` 和 `goimports` 会自动排序：

```go
import (
	"fmt"
	"os"

	crand "crypto/rand" // 重命名只影响当前文件
	mrand "math/rand"
)
```

- 重命名的其他用途：缩短过长的包名、避免与局部变量冲突（如文件里已有 `path` 变量，可把 `path` 包改名为 `pathpkg`）。
- 导入了却不使用的包会导致编译错误——这个限制逼着代码保持依赖干净，`goimports` 会自动增删导入项。
- 还有一种点导入 `import . "fmt"`，把包的导出成员直接倒进当前文件的名字空间，`Printf` 可以裸写。它模糊了每个符号的来源，标准库之外的代码基本不该用；想让测试访问被测包的内部标识符又避免循环导入，正路是 8.2 的 `xxx_test` 外部测试包。

## 8.4 匿名导入用 init 注册运行期能力

**匿名导入为副作用而导**：导入一个包只为了执行它的包级变量初始化和 `init` 函数，用空白标识符 `_` 抑制"未使用"错误。它支撑了 Go 里一种重要的扩展模式——**插件式注册**：`image.Decode` 像一台总机，只认识登记在册的分机号；匿名导入就是让 PNG 这台分机前来报到的那张登记表。

```go
// 摘录自 packages-tools/main.go。
import (
	"bytes"
	"errors"
	"fmt"
	"image"

	// 匿名导入：只为触发 image/png 包 init() 中的 image.RegisterFormat 注册 PNG 解码器，
	// 包本身不被直接引用。去掉本行后，image.Decode 对 PNG 数据会报 image: unknown format。
	// 同理，image/jpeg 与 image/gif 也需要各自匿名导入后才注册，image 包本身不默认注册任何格式。
	_ "image/png"
)
```

标准库的图像格式就是这样工作的：`image/png` 的 `init` 函数调用 `image.RegisterFormat("png", ...)` 把自己登记进格式列表，`image.Decode` 解码时依次查询已注册的格式。**标准库 image 包本身不默认注册任何格式**——没有这行匿名导入，解码 PNG 会得到 `image: unknown format`。`database/sql` 用同样的机制注册数据库驱动（第 10 章的 `_ "github.com/go-sql-driver/mysql"` 就是它）。

### init 什么时候跑

注册机制的便利性来自包初始化顺序的保证：**被导入的包先于导入它的包完成初始化**；单个包内，先按依赖关系求值包级变量，再按文件中的出现顺序执行各 `init` 函数；程序的所有依赖初始化完毕，`main` 才开始。`init` 无参数无返回值，一个包可以有多个，你不能显式调用它。这既是便利（导入即注册，调用方无感知），也是风险（副作用藏在依赖图深处，不读文档不知道它存在）——8.6 的文档注释约定和本章踩坑清单的第一条，都是为这种"看不见的代码"准备的。

### 从解码到探测：packages-tools 的三层实验

本章的 [packages-tools](../../../labs/part1-golang/chapter08/packages-tools) 示例把注册机制做成了可断言的三层实验。它不依赖任何真实图片文件：源码内嵌一个 67 字节的最小合法 1x1 PNG（以十六进制字面量写进 `tinyPNG`，保证离线可测），再用只含魔数的字节串探测其他格式。

第一层是正常解码，`image.Decode` 返回格式名：

```go
// 摘录自 packages-tools/main.go。
// decodeFormat 用 image.Decode 解码 data 并返回格式名（如 "png"）。
// image.Decode 只能识别已注册解码器的格式；未注册时报 image: unknown format（image.ErrFormat）。
func decodeFormat(data []byte) (string, error) {
	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return format, nil
}
```

第二层只用魔数探测，不需要完整图片。JPEG 和 GIF 的探测数据就是"魔数 + 填充字节"，本身不是合法图片：

```go
// 摘录自 packages-tools/main.go。
// jpegMagic 与 gifMagic 只包含格式魔数和填充字节，本身不是完整图片，
// 仅用于探测对应解码器是否已注册。
var jpegMagic = append([]byte{0xff, 0xd8, 0xff, 0xe0}, make([]byte, 64)...)

var gifMagic = append([]byte("GIF89a"), make([]byte, 64)...)
```

为什么探测不完整的数据也行？因为这里要区分两种**语义完全不同的失败**：

- `image.ErrFormat`（`image: unknown format`）：没有任何解码器认领这个魔数——格式未注册；
- 格式专属的解码错误：有解码器认领了魔数，但数据不完整——格式已注册。

`formatRegistered` 正是靠 `errors.Is` 把这两种失败分开，让"探测不完整数据"成为合法的注册状态判据：

```go
// 摘录自 packages-tools/main.go。
// formatRegistered 用魔数探测某种格式的解码器是否已注册：
// 未注册时错误为 image.ErrFormat；已注册时要么解码成功，要么因数据不完整返回格式专属的解码错误。
func formatRegistered(data []byte) bool {
	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	return !errors.Is(err, image.ErrFormat)
}
```

第三层把三种格式各探一遍，返回当前已注册的格式列表：

```go
// 摘录自 packages-tools/main.go。
// registeredFormats 用各格式的魔数探测当前已注册的解码器，返回可识别的格式名列表。
// 本例只匿名导入了 image/png，因此结果是 ["png"]；
// 再添加 import _ "image/jpeg" 或 import _ "image/gif" 后，对应格式才会出现在列表中。
func registeredFormats() []string {
	probes := []struct {
		name string
		data []byte
	}{
		{"png", tinyPNG},
		{"jpeg", jpegMagic},
		{"gif", gifMagic},
	}
	formats := make([]string, 0, len(probes))
	for _, p := range probes {
		if formatRegistered(p.data) {
			formats = append(formats, p.name)
		}
	}
	return formats
}
```

配套测试把"未注册"这个负面事实也固化成断言——只导入 `image/png` 时，JPEG 魔数必须报 `image.ErrFormat`：

```go
// 摘录自 packages-tools/main_test.go。
// 标准库中 image 包本身不默认注册任何格式：jpeg 与 gif 和 png 一样，
// 需要各自 import _ "image/jpeg" / import _ "image/gif" 才会注册。
// 本例只匿名导入了 image/png，因此 JPEG/GIF 魔数报 image: unknown format。
// （已用 Go 1.26 核实该行为。）
func TestJPEGNotRegistered(t *testing.T) {
	_, err := decodeFormat(jpegMagic)
	if !errors.Is(err, image.ErrFormat) {
		t.Fatalf("decodeFormat(jpegMagic) error = %v, want image.ErrFormat", err)
	}
}
```

运行整个示例：

```bash
go -C labs run ./part1-golang/chapter08/packages-tools
```

预期输出：

```text
内嵌图片格式: png
图片尺寸: 1 x 1
已注册格式: [png]
```

## 8.5 包名表达领域，不表达目录层级

**包名要短而有领域语义，成员引用时包名已提供上下文，成员名不要重复包信息。**

- 起 `bufio`、`bytes`、`http`、`sort` 这样的名字；不要起 `util`、`common` 这类没有信息量的名字。
- 一般用单数；`bytes`、`strings` 用复数是为了避开预定义类型。
- 成员引用总是"包名.成员名"：`strings.Index` 而非 `strings.StringIndex`——调用处永远带着包名前缀，成员名里的 `String` 纯属冗余（`StringIndex` 是假想的反例，真实 API 就叫 `Index`）。
- 单一数据类型的包通常暴露一个主要类型 + 一个 `New` 构造函数（`rand.New`）；因此这类包名往往特别短。

命名的对错要到调用处才看得出来：

```go
index := strings.Index(s, ",")   // ✅ 读作"在 strings 里找 Index"，包名即上下文
index := stringutil.FindStringIndexInString(s, ",") // ❌ 假想的反例：包名无领域语义，成员名重复包信息
```

## 8.6 工具链围绕 module 依赖图工作

**`go` 命令身兼三职：包管理器、构建系统、测试驱动。** 常用子命令：

| 命令 | 作用 |
| --- | --- |
| `go build` / `go run` | 编译包 / 编译并运行（main 包生成可执行文件，取导入路径最后一段为名） |
| `go test` / `go vet` | 运行测试 / 静态检查（Printf 格式、锁复制等常见错误） |
| `go doc` | 查看包或符号的文档：`go doc time.Since`、`go doc json.Decoder.Decode` |
| `go list` | 查询包元数据：`go list ./...` 列出仓库全部包，`go list -json hash` 输出完整信息 |
| `go install` / `go env` | 编译安装 / 查看环境变量（GOOS、GOARCH 等） |
| `go fmt` | 按 gofmt 规则格式化；日常更常用编辑器集成的 `goimports`（顺带整理导入） |
| `go get` / `go mod tidy` | 添加、升级依赖 / 让 go.mod 与实际导入对齐，增删依赖条目 |

其中 build、run、install 都会编译，差别在产物去向：

| 子命令 | 产物去向 | 什么时候用 |
| --- | --- | --- |
| `go run` | 临时目录编译、跑完即弃 | 调试、跑 Lab 示例，不留二进制 |
| `go build` | main 包在当前目录留可执行文件；库包只验证可编译 | 产出部署物；`go build ./...` 做全仓编译检查 |
| `go install` | 编译后放进 `$GOBIN`（默认 `$GOPATH/bin`） | 把开发工具装进 PATH，随处可用 |

三个实用技巧：

- **交叉编译**：`GOOS=linux GOARCH=amd64 go build ./cmd/app` 直接产出目标平台的二进制，无需目标机器。
- **平台相关文件**：文件名含 `_linux.go`、`_amd64.go` 后缀的只在对应平台编译；更细的控制用 `//go:build` 约束注释。
- **internal 包**：路径中含 `internal` 的包只能被与其 internal 目录同父的包导入——这是"比包内私有宽、比公开窄"的中间档可见性，本仓库的 `json-api/internal` 就是例子（第 11 章展开）。

文档注释约定：包声明前写包注释（长注释可放独立的 `doc.go`），导出成员的注释以成员名开头、第一句是摘要，如 `// Fprintf formats according to a format specifier ...`。8.4 的 packages-tools 就是按这个约定写的：文件开头的包注释说明整个示例的用途，每个可测函数都带行为说明——这正是 `go doc` 和 pkg.go.dev 展示给读者的内容。

## 踩坑清单

### 删掉"看似没用"的匿名导入

```go
import (
	"image"
	_ "image/png" // 反例：以为这行是多余的可以删掉
)
```

症状：删掉后程序仍能编译，但运行 `image.Decode` 得到 `image: unknown format`。原因：这行 import 的价值不在名字引用，而在 png 包 `init()` 里 `image.RegisterFormat` 的副作用；`_` 豁免了编译器的"未使用导入"检查，删与不删编译器都管不着。解法：恢复该行匿名导入；在 packages-tools 的临时副本中删除本行运行即可复现，恢复后输出 png、测试通过。

### 建一个什么都装的 `util` 包

```text
util/
  string.go   // 字符串 helper
  http.go     // HTTP helper
  time.go     // 时间 helper
```

症状：`util` 不表达任何领域语义，最后变成依赖磁铁：所有包都依赖它，它也可能反向依赖任何包，极易形成循环。原因：包是按领域划分的依赖边界，而 `util`/`common` 是按"代码形态"划分的垃圾桶，两种划分天然冲突。解法：按用途命名（`imageutil`、`tempconv`），让每个包名对应明确职责、依赖方向单向。

### 循环依赖：import cycle not allowed

```go
// a/a.go
package a
import "example.com/m/b" // a 依赖 b

// b/b.go
package b
import "example.com/m/a" // 反例：b 又回头依赖 a
```

症状：编译直接失败，报 `import cycle not allowed`。原因：两个包互相需要对方的导出成员，依赖图出现环——8.1 说过，禁止循环依赖是 Go 编译快的设计前提，编译器绝不放行。解法：把双方都需要的类型下沉到第三个包；或者让一侧只依赖接口而非具体包——接口由使用方定义时（第 7 章），依赖方向可以反转。出现这个错误永远意味着边界划错了，不要想着绕过。

### 让别的包导入 main 包

症状：`import "..." is a program, not an importable package`。原因：可复用的逻辑写进了 main 包，而 main 包是程序入口，不能被当作库导入。解法：逻辑下沉为库包，main 只剩参数解析与装配；惯例是把入口放进 `cmd/app/` 子目录，仓库根部的包留给可复用逻辑。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 8.1 验证注册机制的必要性 | 在临时副本中删除 `_ "image/png"`，先预测 main 的哪一行输出会变化再运行验证，然后恢复；在注释里写清 TestDecodeFormatEmbeddedPNG 依赖哪一行的副作用 | 删除后 `image.Decode` 返回 unknown format，测试以 `image.ErrFormat` 语义失败；恢复后输出 `内嵌图片格式: png`，测试通过 | `packages-tools/main.go`、`main_test.go` |
| 8.2 再注册一种格式 | 增加 `_ "image/jpeg"` 匿名导入，更新 registeredFormats 的预期（断言要对顺序不敏感或与实现顺序一致），并同步调整 TestJPEGNotRegistered 的断言方向 | registeredFormats 返回包含 jpeg 和 png 两种格式；JPEG 魔数不再报 `image.ErrFormat`，而是格式专属的解码错误 | `packages-tools/main.go`、`main_test.go` |

完成后运行：

```bash
go -C labs test ./part1-golang/chapter08/packages-tools/... -count=1
```

更多练习（用 `go list`、`go doc` 查询本仓库的包，再注册 `image/gif`）见各 lab 的 README。

## 小结

- 包 = 名字空间 + 封装；首字母大小写就是可见性，未导出标识符换来的是重构自由。
- 循环依赖被编译器直接拒绝——`import cycle not allowed` 永远意味着包边界划错，抽第三包或用接口反转依赖。
- 匿名导入为 `init` 副作用而导；`image`、`database/sql` 的格式与驱动全靠注册机制，删掉"看似没用"的 `_` 导入，故障要到运行时才出现。
- 包名要短而有领域语义；`util`/`common` 是依赖磁铁的起点。
- `go list`、`go test`、`go vet`、`go build` 面向同一 module 图工作，分别回答依赖、行为、静态风险和构建问题。

## 链接

- [Go 程序设计语言（gopl）第 10 章：包和工具](https://golang-china.github.io/gopl-zh/ch10/ch10.html)：本章的主要参考；其中 GOPATH 工作区部分已被 Go modules 取代，以 [go.mod 官方参考](https://go.dev/doc/modules/gomod-ref)为准。
- [Go 命令文档](https://pkg.go.dev/cmd/go)：build/run/test/vet/doc/list 各子命令的完整说明。
- [Go 规范：包](https://go.dev/ref/spec#Packages)：包声明、导入声明与包初始化顺序的准确定义。
- [Effective Go：包命名](https://go.dev/doc/effective_go#package-names)：短包名与 `rand.New` 式构造函数的官方约定。

[返回全书目录](../README.md) · [上一篇：接口、泛型与反射](chapter07.md) · [下一篇：并发编程](chapter09.md)
