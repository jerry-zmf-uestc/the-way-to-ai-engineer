# 第 2 章：数据类型

第 1 章跑通了第一个程序，从这一章开始程序要真正处理数据了。先感受两个真实的失败：把 `"你好"` 按字节切成 `s[:1]`，得到的是半个 UTF-8 编码拼出的乱码；用 `uint` 做逆序循环计数器，`i >= 0` 永远成立，`i--` 在 0 处回绕成最大值，随后索引越界 panic。这两个 bug 都能编译通过，根源是同一个：没搞清楚 Go 的数据类型到底承诺了什么、不承诺什么。

Go 的数据类型分四类：**基础类型**（数字、字符串、布尔）、**复合类型**（数组、结构体等由其他类型组合而成）、**引用类型**（指针、切片、map、函数、通道——对引用类型数据的修改会影响它的所有拷贝）、**接口类型**（第 7 章）。Go 不给隐式转换留余地：不能混算、拷贝与共享泾渭分明，约束硬换来的是每行代码的数据行为都能本地推断。本章讲透前两类。

<a id="basic-types"></a>

## 2.1 基础数据类型

### 整型

**整型溢出会回绕，不同类型不能混算。** 有符号整型 `int8`/`int16`/`int32`/`int64`，无符号 `uint8`/`uint16`/`uint32`/`uint64`，以及随平台字长的 `int` 和 `uint`。n 位有符号的值域是 −2^(n−1) ~ 2^(n−1)−1（int8：−128~127），无符号是 0 ~ 2^n−1（uint8：0~255）。两个常用同义词：`rune` = `int32`（Unicode 码点），`byte` = `uint8`（原始字节）。

| 类型 | 位宽 | 值域 | 典型用途 |
| --- | --- | --- | --- |
| `int8` / `uint8` | 8 | −128~127 / 0~255 | 原始字节（`byte`）、小枚举 |
| `int16` / `uint16` | 16 | −32768~32767 / 0~65535 | 端口号等定宽协议字段 |
| `int32` / `uint32` | 32 | 约 ±21 亿 / 0~42 亿 | Unicode 码点（`rune`）、IPv4 地址 |
| `int64` / `uint64` | 64 | 约 ±9.2×10^18 | 时间戳、分布式 ID、文件偏移 |
| `int` / `uint` | 随平台（32 或 64） | 与对应位宽一致 | 默认选择；`len`、下标都用 `int` |

```go
var u uint8 = 255
fmt.Println(u, u+1, u*u) // "255 0 1"：溢出时高位被丢弃，可能回绕

var i int8 = 127
fmt.Println(i, i+1) // "127 -128"：有符号溢出同样回绕

fmt.Println(-5 % 3)  // "-2"：取模符号与被取模数一致
fmt.Println(5 / 4)   // "1"：整数除法向 0 截断
fmt.Println(5.0 / 4) // "1.25"
```

- `int32 + int16` 直接编译错误，必须显式转换；大转小、浮点转整会丢数据（向零截断）。
- 优先使用有符号 `int`：`len` 返回 int，无符号计数器在 0 处回绕会让逆序循环越界（见踩坑清单）；无符号数主要留给位运算、二进制协议和哈希场景。

```go
// ✅ 下标、计数、长度相关一律用 int——与 len 的返回类型一致，0 处不会回绕
for i := len(s) - 1; i >= 0; i-- {
	_ = s[i]
}

// ❌ 逆序计数器用 uint：i >= 0 恒真，0 再减 1 回绕成最大值，循环无法结束并越界
for i := uint(len(s) - 1); i >= 0; i-- {
	_ = s[i]
}
```

- 位运算：`&`、`|`、`^`、`&^`（位清空）、`<<`、`>>`；移位量必须是无符号数，`x<<n` 等价乘 2^n。位运算可以当集合用：交集 `x&y`、并集 `x|y`、差集 `x&^y`。

### 浮点数与复数

`float32`/`float64` 遵循 IEEE 754。**默认用 float64**：float32 只有约 6 位十进制精度，累计误差容易扩散：

```go
var f float32 = 16777216 // 1 << 24
fmt.Println(f == f+1)    // "true"：float32 已无法分辨 +1

var z float64
fmt.Println(1/z, -1/z, z/z) // "+Inf -Inf NaN"：除零得无穷，0/0 得 NaN
```

- NaN 与任何值（包括自身）比较都为 false，检测用 `math.IsNaN`。可能失败的计算优先返回 `(value, ok)` 双值，而不是让 NaN 静默传播。
- 金额不要用浮点数直接存：0.1 在二进制浮点里无法精确表示，误差会在对账时浮现；用整数保存最小单位（分），或引入 decimal 库。
- 复数 `complex64`/`complex128` 用内置 `complex(re, im)` 构造、`real()`/`imag()` 取部，`math/cmplx` 提供复数函数；日常业务很少用到。

### 布尔型

布尔值只有 `true`/`false`，与数字**不隐式互转**，要显式写 `i != 0` 这类判断。`&&`、`||` 有短路行为：`s != "" && s[0] == 'x'` 在 s 为空时不会求值右侧。

### 常量与 iota

**常量在编译期求值**，潜在类型只能是布尔、字符串、数字；常量溢出在编译期就报错（`const a = int32(0xdeadbeef)` 编译失败），比变量多一层保护。批量声明时可省略初始化表达式，继承上一行；`iota` 在每个 const 块内从 0 开始逐行递增，空行和注释不打断计数：

```go
type Weekday int

const (
	Sunday Weekday = iota // 0
	Monday                // 1
	Tuesday               // 2
	// ...
)

const (
	_  = 1 << (10 * iota)
	KiB // 1024
	MiB // 1048576
	GiB
)
```

**无类型常量**是 Go 常量最特别的机制：常量数字在编译器中以高精度保存，类型延迟到使用时绑定：

```go
const Pi = 3.14159265358979323846
var f32 float32 = Pi  // 无需显式转换
var f64 float64 = Pi

fmt.Println(5 / 9 * 1.0)   // "0"：5/9 先按无类型整数除法得 0
fmt.Println(5.0 / 9 * 1.0) // "0.555..."
```

简短声明的默认类型：`0` → int，`'\000'` → rune（int32），`0.0` → float64，`0i` → complex128。

### 变量、零值与默认值

编译期固定的数据用常量，运行期临时结果用变量；**没有显式赋值的变量有稳定零值**（数值 0、布尔 false、字符串 ""、指针/切片/map 等 nil）。摘录自 [values-variables-constants/main.go](../../../labs/part1-golang/chapter02/values-variables-constants/main.go)：

```go
const language = "Go"

func summary(version int, stable bool) string {
	status := "experimental"
	if stable {
		status = "stable"
	}
	return fmt.Sprintf("%s example v%d is %s", language, version, status)
}
```

三行各司其职：`const language` 是编译期常量，全程序共享一份；`status := "experimental"` 先给保底值，默认路径一眼可见，只有特殊情况才覆盖；`fmt.Sprintf` 按格式动词拼装，`%s` 放字符串、`%d` 放整数。零值稳定不等于业务默认值正确：重试次数可以默认 0，监听端口却通常还需要校验。`:=` 只能用于函数内部，同一作用域的短变量声明至少要引入一个新变量；内层代码块中的同名变量会遮蔽外层变量。

运行 [values-variables-constants](../../../labs/part1-golang/chapter02/values-variables-constants) 示例：

```bash
go -C labs run ./part1-golang/chapter02/values-variables-constants
```

输出 `Go example v1 is stable`。

### 指针初步

**Go 只有值传递**——函数实参总是拷贝，指针也不例外（拷贝的是地址）；切片、map、通道"看起来像引用"，是因为它们的头部结构里含指针，2.3 节会看到细节。

```go
x := 1
p := &x  // p 指向 x
*p = 2   // 通过指针修改 x
fmt.Println(x) // "2"
```

nil 是指针的零值，解引用 nil 指针会 panic。何时该传指针在第 4、5 章展开。

<a id="strings-runes"></a>

## 2.2 字符串、字节与 rune

### 不可变的字节序列

**字符串是不可变的字节序列**，可包含任意数据；文本通常按 UTF-8 解释为 rune 序列。`len(s)` 返回**字节数**（不是字符数），`s[i]` 取第 i 个字节，子串 `s[i:j]` 取字节区间 [i, j)。不可变意味着 `s[0] = 'L'` 是编译错误，`s += ...` 是赋新值而非改原串——这也让字符串的拷贝和切片可以安全共享底层内存，代价低廉。

字符串面值有两种：双引号（支持 `\n`、`\xhh`、`\uhhhh` 等转义）和反引号**原生字符串**（无转义、可跨行，适合正则、JSON、模板）。

### UTF-8 与 rune

UTF-8 是变长编码（1~4 字节），完全兼容 ASCII。一个中文字符通常占 3 字节，因此字节数和字符数经常不一致：

```go
s := "Hello, 世界"
fmt.Println(len(s))                    // "13" 字节
fmt.Println(utf8.RuneCountInString(s)) // "9" 个码点

for i, r := range s { // range 自动按 UTF-8 解码，非 ASCII 时 i 的步长 >1
	_ = i
	_ = r
}
```

- Go 字符串保存字节序列，不保证内容是合法 UTF-8；遇到非法输入时，解码和 range 会产生替换字符 `\uFFFD`（）——看到它是危险信号。
- `[]rune(s)` 与 `string(runes)` 互转会自动做 UTF-8 解码/编码，按 rune 处理文本就先转成 `[]rune`；**`int` 转 `string` 是码点转换，不是数字转换**：`string(65)` 得到 `"A"` 而不是 `"65"`，数字转字符串用 `strconv.Itoa`。
- rune 是 Unicode 码点，不等于人看到的一个完整字符：组合音标、肤色修饰和家庭 emoji 可能包含多个码点。按 rune 截取可以避免切断合法 UTF-8 编码，但不能保证不拆开字素簇；界面字符计数需要另一个层次的分段规则。
- 得益于 UTF-8 的设计，子串、前缀/后缀判断（`strings.HasPrefix`）直接按字节做也是安全的。

### 四个常用包

| 包 | 代表函数 | 什么时候用 |
| --- | --- | --- |
| `strings` | `Contains`、`Fields`、`Join`、`Replacer` | 字符串查询、替换、拆分、合并 |
| `bytes` | 同族函数 + `bytes.Buffer` | 处理 `[]byte`；Buffer 零值即可用，适合逐步构建字符串 |
| `strconv` | `Itoa`/`Atoi`/`ParseInt`/`FormatInt` | 数值与字符串互转 |
| `unicode` | `IsDigit`、`IsLetter`、`ToUpper` | 按 rune 做字符分类 |

循环里逐步拼长字符串时，`bytes.Buffer` 和 `+` 的差距是量级级的：

```go
// ✅ bytes.Buffer：内部维护可扩容的字节缓冲区，追加是摊还常数时间
var buf bytes.Buffer
for _, w := range words {
	buf.WriteString(w)
}
result := buf.String()

// ❌ 循环里 s += w：字符串不可变，每次拼接都分配新串并复制已有内容，n 次拼接累计复制 O(n²) 字节
var s string
for _, w := range words {
	s += w
}
```

[strings-runes](../../../labs/part1-golang/chapter02/strings-runes) 示例把文本处理拆成三类动作：统计（`Analyze`，字节数和 rune 数量可能不同）、截取（`FirstRunes`，按 rune 遍历，不切断编码）、归一化（`NormalizeWords`、`ReplaceTopic`，清洗输入比机械拼接更重要）。摘录自 [strings-runes/main.go](../../../labs/part1-golang/chapter02/strings-runes/main.go)：

```go
func Analyze(s string) StringStats {
	return StringStats{
		Bytes: len(s),
		Runes: utf8.RuneCountInString(s),
	}
}

func FirstRunes(s string, n int) []rune {
	if n <= 0 {
		return nil
	}

	result := make([]rune, 0, n)
	for _, r := range s {
		result = append(result, r)
		if len(result) == n {
			break
		}
	}
	return result
}
```

`FirstRunes` 的两个细节值得记住：`n <= 0` 直接返回 nil，把非法输入挡在入口；`make([]rune, 0, n)` 预留容量，避免 append 反复扩容。归一化和替换是另外两个函数，同样摘录自该文件：

```go
func NormalizeWords(s string) string {
	return strings.Join(strings.Fields(s), ",")
}

func ReplaceTopic(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}
```

`strings.Fields` 按连续空白拆分（制表符、换行、多个空格都算一个分隔），天然完成空白归一化，`Join` 再用单个逗号拼回；`ReplaceAll` 替换所有匹配并返回新串——字符串不可变，原串不动。

运行示例：

```bash
go -C labs run ./part1-golang/chapter02/strings-runes
```

```text
bytes=8 runes=4
你好
go,learn,go
learn go, write go
```

<a id="collections"></a>

## 2.3 数组、切片与映射

### 如何组织一组数据

把数据放进集合，同一段循环就能处理数量不同的数据。三个集合类型各司其职：

| 类型 | 示例 | 主要特点 |
| --- | --- | --- |
| 数组 | `[3]int` | 长度属于类型，赋值复制整个数组值 |
| 切片 | `[]int` | 描述底层数组的一段区域，长度可以通过新切片值改变 |
| map | `map[string]int` | 通过键定位值，不承诺遍历顺序 |

选择类型时，先想后面的代码要怎样访问数据：按输入顺序处理用切片，按名字查找用 map，长度本身是约束时用数组。

### 数组

数组是固定长度的同构元素序列，**长度是类型的一部分**（`[3]int` 与 `[4]int` 是不同类型），长度必须是常量表达式。完整教学程序，在练习副本根目录保存为 `inspect.go` 后执行 `go run inspect.go`：

<!-- verify:array -->
```go
package main

import "fmt"

func main() {
	a := [3]int{1, 2, 3}
	b := a
	b[0] = 9
	fmt.Println(a)
	fmt.Println(b)
	fmt.Println(len(a))
}
```

输出：

```text
[1 2 3]
[9 2 3]
3
```

b 得到 a 的数组值副本，修改 b 不会改动 a。数组的元素类型可比较时数组本身可比较（`==` 逐元素比较），因此 `[32]byte` 这类定长数组可以直接判等——`sha256.Sum256` 就返回 `[32]byte`。数组作函数参数是整体拷贝，除定长哈希这类场景外，一般用切片代替数组。

### 切片的长度与容量

**切片 = 起点指针 + 长度 + 容量**，描述底层数组中的一段数据：

- `len(s)` 是当前长度，索引必须小于它。
- `cap(s)` 是从切片起点算起的容量。
- `make([]int, 2, 4)` 创建长度为 2、容量为 4 的切片。

容量为 4 不表示可以直接访问 `s[3]`：索引受长度约束，扩展长度需要 append 或合法的重新切片。`s[low:high]` 包含 low，不包含 high，结果长度为 `high-low`；上界可以到 cap，而不仅是 len。[Go 规范：切片表达式](https://go.dev/ref/spec#Slice_expressions)

### 切片与底层数组

下面是 [arrays-slices/main.go](../../../labs/part1-golang/chapter02/arrays-slices/main.go) 的完整函数摘录，不是独立程序：

<!-- verify:window -->
```go
func window(values []int, start, end int) []int {
	if start < 0 || end > len(values) || start > end {
		return nil
	}
	return values[start:end]
}
```

`window(values, 1, 3)` 返回索引 1 和 2 对应的视图。**视图仍然指向原来的元素，没有复制**；修改返回切片中的元素，values 中对应的元素也会改变。排查共享修改时，先找出"返回切片下标 0 对应原切片哪个下标"的对应关系，再看赋值写到了哪里。

边界检查遇到负起点、超过当前长度的终点或颠倒区间时返回 nil。合法的空区间长度也为 0；只检查 len 就分不清"空结果"和"非法区间"——设计 API 时要把这个约定告诉调用方，或者增加一个返回值报告错误。

### append 的行为

**append 返回一个新切片值，必须接回**（`s = append(s, value)`）。旧容量够用时复用原底层数组；容量不足时分配新的底层数组并复制已有元素。"新切片值"和"新底层数组"要分开理解。

完整教学程序，用下面内容替换 `inspect.go` 后执行 `go run inspect.go`：

<!-- verify:append -->
```go
package main

import "fmt"

func main() {
	s := make([]int, 2, 3)
	s[0], s[1] = 1, 2

	t := append(s, 3) // 容量够用，仍共享底层数组。
	t[0] = 9
	fmt.Println(s, t)
	fmt.Println(len(s), len(t), cap(t))

	u := append(t, 4) // t 的长度已等于容量，需要新的底层数组。
	u[0] = 7
	fmt.Println(t, u)
	fmt.Println(len(u), cap(u) >= len(u))
}
```

输出：

```text
[9 2] [9 2 3]
2 3 3
[9 2 3] [7 2 3 4]
4 true
```

逐行对一下输出：`t := append(s, 3)` 时容量 3 够用，t 复用 s 的底层数组，所以 `t[0] = 9` 连 s 一起改了；但 s 的 len 仍是 2，打印 s 只看到 `[9 2]`。`u := append(t, 4)` 时 t 的 len 已等于 cap，append 分配了新的底层数组，此后 `u[0] = 7` 与 t 互不相干。判断共享不能只看长度，也不能只看打印出来的元素数量。不要把某次看到的扩容倍数写成程序约定——确认容量至少满足长度，再检查数据是否共享即可。[Go 切片说明](https://go.dev/blog/slices-intro)

切片之间不能用 `==` 比较（只能和 nil 比）；判断空切片用 `len(s) == 0`，不要用 `s == nil`——nil 切片的行为等同于零长切片。

### 用 copy 保留独立结果

下面是同一 [main.go](../../../labs/part1-golang/chapter02/arrays-slices/main.go) 的完整函数摘录：

<!-- verify:append-copy -->
```go
func appendCopy(base []int, values ...int) []int {
	out := make([]int, len(base), len(base)+len(values))
	copy(out, base)
	return append(out, values...)
}
```

`values ...int` 允许调用者传入多个 int（可变参数，第 4 章详述）；`values...` 把切片展开交给 append。函数先分配足够空间，将 base 复制到 out，再追加新值。copy 只复制两者长度较小的那部分，因此目标长度先设为 `len(base)`——长度为 0 时即使预留了容量，copy 也不会复制任何元素。

两个函数对应两种约定，选哪一种，要看调用方是否允许原数据被修改：

| 场景 | 选择 | 原因 |
| --- | --- | --- |
| 查看或就地修改原数据的一段 | `window` 式切片视图 | 零拷贝，改动直接反映到原数据 |
| 原数据不能动、要生成新结果 | `appendCopy` 式先复制再追加 | 新底层数组，调用方数据不受污染 |
| 拿不准是否共享 | 显式 `make` + `copy` | 把"独立副本"写进代码，而不是靠口头约定 |

无论选哪种，都把"是否共享"写进函数说明和测试。

### map 的读写与遍历

**map 是哈希表的引用**：键无序、唯一，查找/更新/删除为常数时间；**键类型必须支持 == 比较**（浮点数做键是坏主意：NaN 与任何值都不等）。下面是 [maps/main.go](../../../labs/part1-golang/chapter02/maps/main.go) 的源码摘录：

<!-- verify:count-words -->
```go
func countWords(words []string) map[string]int {
	counts := make(map[string]int)
	for _, word := range words {
		if word == "" {
			continue
		}
		counts[word]++
	}
	return counts
}
```

第一次遇到某键时，读取得到零值 0，递增后为 1——不需要先把每个键填成 0。查询时用双值形式区分"存在但值为 0"与"不存在"：

<!-- verify:lookup -->
```go
func lookup(counts map[string]int, word string) (int, bool) {
	count, ok := counts[word]
	return count, ok
}
```

`delete(counts, "go")` 删除键，删除不存在的键不报错。**不能对 map 元素取地址**（`_ = &counts["go"]` 编译错误）：map 扩容会重新分配内存，地址会失效。

遍历顺序有两个必须记住的事实：`for word, count := range counts` 的顺序不确定且每次随机（故意设计，防止程序依赖哈希实现）；要固定顺序就显式排序键，或沿输入顺序另存结果。

### 按首次出现的顺序去重

下面是同一 [main.go](../../../labs/part1-golang/chapter02/maps/main.go) 的函数摘录：

<!-- verify:unique-words -->
```go
func uniqueWords(words []string) []string {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(words))
	for _, word := range words {
		if word == "" || seen[word] {
			continue
		}
		seen[word] = true
		unique = append(unique, word)
	}
	return unique
}
```

map 只记录某个词是否已出现；切片负责保存结果顺序。map 负责判断该不该追加，切片负责记住追加顺序——一道题可以组合两种集合。

运行示例：

```bash
go -C labs run ./part1-golang/chapter02/arrays-slices
go -C labs run ./part1-golang/chapter02/maps
```

`arrays-slices` 的 main 把两个函数串起来（摘录自同一文件）：

```go
func main() {
	values := []int{1, 2, 3}
	extended := appendCopy(values, 4, 5)
	middle := window(values, 1, 3)

	fmt.Println("base:", values)
	fmt.Println("extended:", extended)
	fmt.Println("middle:", middle)
}
```

输出：

```text
base: [1 2 3]
extended: [1 2 3 4 5]
middle: [2 3]
```

`maps` 输出（输入为 `{"go", "learn", "go", ""}`）：

```text
map[go:2 learn:1]
2 true
[go learn]
```

fmt 打印 map 时按键排序，这不代表 `range map` 也有同样顺序。不要从打印效果推导语言的遍历约定。

<a id="struct-json"></a>

## 2.4 结构体与 JSON

### 结构体基础

结构体把一组命名字段聚合成一个值：

```go
type Employee struct {
	ID     int
	Name   string
	Salary int
}

var e1 Employee                          // 零值：每个字段都是零值
e2 := Employee{1, "Ada", 100}            // 按字段顺序（字段调整会破坏编译，慎用）
e3 := Employee{ID: 1, Name: "Ada"}       // 按字段名（推荐；未写字段取零值）
```

- 点操作符访问字段；结构体指针也可以直接用点操作符（自动解引用）。全部字段可比较时结构体可比较，可作 map 的键。
- 大写开头的字段是**导出**的，包外可见；小写字段包外不可见（第 8 章详述可见性规则）。
- 结构体不能递归包含自身，但可以包含指向自身的指针（链表、树的基础）。给结构体定义行为（方法）是第 6 章的内容。

### JSON 编组与解码

JSON 是服务间交换结构化数据的标准格式。Go 的 `encoding/json` 通过反射按结构体生成/解析 JSON，**字段名和 tag 就是数据契约**。摘录自 [json/main.go](../../../labs/part1-golang/chapter02/json/main.go)：

```go
type Profile struct {
	Name string   `json:"name"`
	Age  int      `json:"age,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

func EncodeProfile(profile Profile) (string, error) {
	data, err := json.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("marshal profile: %w", err)
	}
	return string(data), nil
}
```

逐行看 tag：`json:"name"` 把 JSON 键改成小写（默认键就是字段名 `Name`）；`omitempty` 让零值字段不输出——Age 为 0、Tags 为 nil 或空切片时，键直接从 JSON 里消失。tag 的完整写法：

| tag 写法 | 编码效果 |
| --- | --- |
| 不写 tag | JSON 键 = 字段名（大小写不变） |
| `json:"name"` | 键重命名为 name |
| `json:"name,omitempty"` | 重命名 + 零值（0、""、nil、空切片/map）不输出 |
| `json:",omitempty"` | 保留字段名作键，只加零值省略 |
| `json:"-"` | 字段完全不参与编码和解码 |

- **只有导出字段会被编码**；解码时只填结构体里声明的字段，其余 JSON 字段默认被忽略（后果见踩坑清单）。
- 普通值字段会让"缺失字段"和"显式零值"合并；业务需要区分时用指针字段——缺失为 nil，显式零值为指向零值的指针。

编码入口按输入形态选：

| 场景 | 用哪个 | 原因 |
| --- | --- | --- |
| 内存中已有完整值 / 字节串 | `json.Marshal` / `json.Unmarshal` | 一步到位 |
| HTTP body、文件等 `io.Reader` 流 | `json.NewDecoder(r).Decode(&v)` | 不必先读完整个字节串 |
| 需要拒绝未知字段 | `Decoder` + `DisallowUnknownFields()` | `Unmarshal` 没有这个开关 |
| 给人看的调试输出 | `json.MarshalIndent` | 缩进格式 |

不可编码的类型不是编译错误，而是运行时的 error。lab 里的 `BadEncodeJson` 故意序列化一个函数字段，摘录自同一文件：

```go
type BadCase struct {
	F func()
}

func BadEncodeJson() ([]byte, error) {
	bad := BadCase{
		F: func() {},
	}
	return json.Marshal(bad)
}
```

函数、通道、复数都没有 JSON 表示，Marshal 返回 error 而不是 panic——记得接住，别丢。

运行 [json](../../../labs/part1-golang/chapter02/json) 示例：

```bash
go -C labs run ./part1-golang/chapter02/json
```

```text
{"name":"Ada","tags":["go","json"]}
{Name:Ada Age:0 Tags:[]}
error: json: unsupported type: func()
```

三行输出分别对应：编码结果（Age 为 0 被 omitempty 省略）、宽松解码（unknown 字段被忽略）、不可编码类型的 error。

## 踩坑清单

### 局部值全写成 var

```go
func releaseLabel(stable bool) string {
	var status string
	status = "experimental" // 反例：合法但多了一次赋值，:= 已足够表达。
	if stable {
		status = "stable"
	}
	return status
}
```

现象：编译通过，但默认路径被两次赋值打散，噪音增加。原因：`var` 只声明不赋值，保底值被迫另起一行，读者要读完整个函数才能确认默认行为。修复：用 `:=` 直接给保底值（见 2.1 节 `summary` 的写法）。

### 用无符号数做逆序循环计数

```go
// 反例：i 是 uint 时 i >= 0 恒真，i-- 在 0 处回绕，循环无法结束并越界。
for i := uint(len(s) - 1); i >= 0; i-- {
	_ = s[i]
}
```

现象：`i >= 0` 对无符号数恒真，`i--` 在 0 处回绕为最大值，循环无法结束并越界。原因：无符号数没有负数，回绕是定义好的行为而非错误，编译器不会拦。修复：逆序循环计数器用有符号 `int`。

### 直接按字节下标切中文字符串

```go
s := "你好 Go"

// 反例：这里按字节切，不保证刚好落在字符边界上。
prefix := s[:1]
```

现象：切点不一定落在字符边界上，切断 UTF-8 编码得到乱码。原因：一个中文通常占 3 字节，`s[:1]` 只拿到第一个字节，它不是任何合法编码。修复：按 rune 处理——先转 `[]rune` 再切片，或用 range 遍历。

### 用 len 当"字符数"做限制

```go
func allowName(s string) bool {
	// 反例：len 看的是字节，不是用户理解的"几个字"。
	return len(s) <= 4
}
```

现象：像 `"你好"` 这种只有 2 个 rune、占 6 个字节的输入会被错误拒绝。原因：`len` 对字符串返回字节数，限长和校验要的是字符数。修复：用 `utf8.RuneCountInString` 计量字符数。

### 把切片窗口当成副本

<!-- verify:alias -->
```go
package main

import "fmt"

func main() {
	base := []int{1, 2, 3}
	alias := base[1:]
	alias[0] = 99
	fmt.Println(base)
}
```

输出 `[1 99 3]`——alias 的第一个元素就是 base 的第二个元素，窗口不是副本。原因：切片只是"起点指针 + 长度 + 容量"的头部，`base[1:]` 与 base 共享同一个底层数组。修复：用 make + copy 创建独立副本：

<!-- verify:alias-fix -->
```go
alias := make([]int, len(base)-1)
copy(alias, base[1:])
```

替换后重新运行 `go run inspect.go`，输出恢复为 `[1 2 3]`。

### 以为 append 原地生效

```go
s := []int{1, 2}
append(s, 3) // 反例：返回值被丢弃，s 还是 [1 2]。
```

现象：append 之后 s 毫无变化，元素"丢了"。原因：append 返回的是新切片值，长度信息在返回值里，旧变量不会自动更新——即使底层数组没换。修复：永远写 `s = append(s, 3)`；切片传给函数追加时，函数要返回新切片，调用方接回。

### 用计数为零判断键不存在

<!-- verify:map-zero -->
```go
package main

import "fmt"

func main() {
	counts := map[string]int{"go": 0}
	fmt.Println(counts["go"], counts["rust"])
	goCount, goOK := counts["go"]
	rustCount, rustOK := counts["rust"]
	fmt.Println(goCount, goOK)
	fmt.Println(rustCount, rustOK)
	delete(counts, "go")
	_, exists := counts["go"]
	fmt.Println(exists)
}
```

输出：

```text
0 0
0 true
0 false
false
```

第一行两个值都是 0，单看数值分不清哪个键存在；ok 分别是 true 和 false。原因：map 对缺失键返回值类型的零值，与"存在但值为零"撞车。修复：需要判断键是否存在时，把单值查找改成双值查找。

### 向 nil map 写入

<!-- verify:nil-map -->
```go
package main

func main() {
	var counts map[string]int
	counts["go"]++
}
```

现象：运行失败，错误包含 `assignment to entry in nil map`。原因：nil map 只有头部没有哈希表，读取缺失键得零值是安全的，写入却无处安放；递增既包含读取也包含写入，读取不存在键得到 0，并不代表写回也合法。修复：存数据前必须先 make。[Go map 说明](https://go.dev/blog/maps)

### JSON 宽松解码吞掉未知字段

输入 `{"name":"Ada","unknown":true}` 经过宽松解码（lab 中的 `DecodeProfile`）：

```go
func DecodeProfile(data []byte) (Profile, error) {
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("unmarshal profile: %w", err)
	}
	return profile, nil
}
```

现象：解码成功，unknown 字段被静默丢弃——对方系统改了字段名，你这边毫无知觉地丢数据。原因：解码只填结构体里声明的字段，其余 JSON 字段默认被忽略。修复：对外部输入用严格版（lab 中的 `DecodeProfileStrict`），开 `DisallowUnknownFields`：

```go
func DecodeProfileStrict(data []byte) (Profile, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var profile Profile
	if err := decoder.Decode(&profile); err != nil {
		return Profile{}, fmt.Errorf("decode profile strictly: %w", err)
	}
	return profile, nil
}
```

注意它只拒绝未知字段，不会自动拒绝第一个 JSON 值后面的第二个值，严格单对象输入还应继续解码并检查 EOF。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| <a id="basic-types-exercises"></a>2.1 增加 edition 参数 | `summary` 增加 `edition string` 参数，保留原来的状态分支，同步 main 和所有测试调用 | 输出包含 community；true/false 两种状态都被断言，测试通过 | `values-variables-constants/main.go`、`main_test.go` |
| <a id="strings-runes-exercises"></a>2.2 返回最后 n 个 rune | 实现 `LastRunes`：先处理 n<=0，再转 `[]rune`，限制 n 的上界，最后切片 | `LastRunes("你好🙂", 2)` 为 `好🙂`；n<=0 返回空串，超长返回全文；覆盖中文、emoji、空串、负数、0 和超长 | `strings-runes/main.go`、`main_test.go` |
| <a id="collections-exercises"></a>2.3 观察长度、容量和共享 | 打印 values、extended、middle 的 len/cap；分别修改 `extended[0]`、`middle[0]` 后对比输入；测试补上边界与修改后的断言 | values 3/3、extended 5/5、middle 2/2（len/cap）；appendCopy 的输入不变，window 共享原数据对应位置；`window(values,-1,2)`、`window(values,0,4)` 返回 nil，合法空窗口长度为 0 | `arrays-slices/main.go`、`main_test.go` |
| <a id="struct-json-exercises"></a>2.4 比较普通和严格 JSON 解码 | 对带 unknown 字段的输入分别调用 `DecodeProfile` 与 `DecodeProfileStrict`，补齐断言 | 普通模式成功、严格模式返回错误，两条断言都成立 | `json/main_test.go` |

完成后运行：

```bash
go -C labs test ./part1-golang/chapter02/values-variables-constants/... -count=1
go -C labs test ./part1-golang/chapter02/strings-runes/... -count=1
go -C labs test ./part1-golang/chapter02/arrays-slices/... -count=1
go -C labs test ./part1-golang/chapter02/maps/... -count=1
go -C labs test ./part1-golang/chapter02/json/... -count=1
```

更多练习（修改版本号、emoji 两种长度、空白归一化、计数大小写归一与合并、omitempty 对比、指针字段区分缺失与零值）见各 lab 的 README。

## 链接

- [Go 规范](https://go.dev/ref/spec)：核对[数组](https://go.dev/ref/spec#Array_types)、[切片](https://go.dev/ref/spec#Slice_types)、[map](https://go.dev/ref/spec#Map_types)、常量表达式的准确定义。
- [Go 程序设计语言（gopl）第 3 章：基础数据类型](https://golang-china.github.io/gopl-zh/ch3/ch3.html)与[第 4 章：复合数据类型](https://golang-china.github.io/gopl-zh/ch4/ch4.html)：本章结构的主要参考，含更多示例与练习。
- [Go Slices: usage and internals](https://go.dev/blog/slices-intro)：理解底层数组和切片视图。
- [Go maps in action](https://go.dev/blog/maps)：补充查找、零值和遍历说明。
- [Go 标准库：encoding/json](https://pkg.go.dev/encoding/json)：核对编码、解码、字段标签和 Decoder 的输入限制。
- [Go 标准库：unicode/utf8](https://pkg.go.dev/unicode/utf8)：RuneCountInString、DecodeRune 等 UTF-8 解码函数的准确定义。

[返回全书目录](../README.md) · [上一篇：Go 环境与第一个程序](chapter01.md) · [下一篇：控制流与边界分支](chapter03.md)
