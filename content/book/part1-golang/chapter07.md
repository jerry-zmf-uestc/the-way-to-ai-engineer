# 第 7 章：接口、泛型与反射

第 6 章末尾埋了一个反直觉的事实：把 `(*Account)(nil)` 放进接口变量，接口本身并不等于 nil——因为接口里装的是"（动态类型，动态值）"两件套，类型在，值是 nil。这个坑只是入口。真正的问题是：什么时候你**必须**接受"运行期才知道类型"，什么时候其实可以在编译期解决。想象你在写一个配置绑定器：用户给一个 `any`，里面是任意结构体指针，你要按字段名把字符串写进去——字段有几个、叫什么、什么类型，编译时一概不知，这正是反射的地盘。而如果你只是想让"求和"同时服务 `[]int` 和 `[]float64`，动用反射就是把编译器能做的事拖到运行期去爆炸。接口在编译期约束"能力"，泛型在编译期复用"同构不同型"的逻辑，反射处理"运行期才知道类型"的边界——能用接口或泛型解决的，就不动用反射，这是本章全部判断的主线。

<a id="reflection"></a>

## 7.1 接口与反射

**接口值 = （动态类型， 动态值）**。从接口取回具体类型用类型断言，三种形态的分工：

```go
// 类型断言与 type-switch（示意片段，不可独立运行）。
var v any = "hello"

s, ok := v.(string) // 双值形式：失败时 ok=false，不 panic
_ = s
n := v.(int)        // 单值形式：失败时 panic
_ = n

switch x := v.(type) { // type-switch：按动态类型分支
case string:
	_ = x // x 是 string
case int:
	_ = x // x 是 int
default:
	_ = x // x 与 v 同类型
}
```

- 空接口 `any`（`interface{}` 的别名）能装任何值，是 JSON 解码、格式化打印的入口；但每用一次 any，就把一次类型检查从编译期推迟到运行期——编译器不再替你挡错误，错误改在用户的输入数据上爆炸。
- 经验法则：**双值形式是默认选择**，单值形式只在"类型已由上游保证"的场景出现（比如刚做完 type-switch 的分支内）。

### 反射读类型：`TypeOf` / `ValueOf`

**`reflect.TypeOf` 和 `reflect.ValueOf` 把接口值拆成"类型"和"值"。** 读类型是反射里唯一不容易出事的部分——不修改、不调用，纯观察。摘录自 reflection-basics/main.go：

```go
func describeType(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return "<nil>"
	}
	name := t.Name()
	if name == "" {
		// Composite types such as []int have no name; fall back to String().
		name = t.String()
	}
	return fmt.Sprintf("%s(%s)", name, t.Kind())
}
```

三个返回值的语义要分开记：

| API | 返回什么 | 易错点 |
| --- | --- | --- |
| `Type.Name()` | 命名类型的名字，如 `User` | `[]int` 等未命名类型拿到空串，要展示完整类型形状用 `Type.String()` |
| `Type.String()` | 完整类型形状，如 `[]int`、`main.User` | 含包名，不适合做跨包的稳定标识 |
| `Type.Kind()` | 底层分类，如 `struct`、`slice` | 自定义类型 `type Score int` 的 Kind 是 `int`，不是 `Score` |

### 反射读 tag：结构体字段遍历

序列化、ORM、参数校验框架的核心动作都是同一个：**遍历结构体字段，读 tag，按 tag 决定行为**。`fieldTags` 就是这个动作的最小实现，注意它对指针的解引用和对未导出字段的跳过。摘录自 reflection-basics/main.go：

```go
func fieldTags(v any, tagKey string) ([]string, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("field tags for %s: %w", rv.Kind(), ErrNotStruct)
	}

	t := rv.Type()
	tags := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tags = append(tags, fmt.Sprintf("%s=%s", field.Name, field.Tag.Get(tagKey)))
	}
	return tags, nil
}
```

- 结构体标签在**类型**上而不在值上，所以遍历从 `rv.Type()` 开始；`NumField`/`Field(i)` 是 `Type` 的方法。
- `IsExported` 跳过的正是 lab 里 `User` 的 `role` 字段——未导出字段对反射"可见但不可碰"，这在写入路径上会再咬一次人。

### 反射改字段：从指针开始，逐层检查

**反射修改几乎总从指针开始**——值副本不可寻址，改不到原数据。`setString` 是写入路径的完整范本：每一层失败都变成一个哨兵错误（sentinel error），供调用方用 `errors.Is` 判断失败原因，而不是字符串匹配错误消息。先看错误定义，摘录自 reflection-basics/main.go：

```go
// Sentinel errors let callers use errors.Is instead of string comparison.
var (
	ErrNotStruct     = errors.New("reflection: value is not a struct")
	ErrNotPointer    = errors.New("reflection: value is not a non-nil pointer")
	ErrFieldMissing  = errors.New("reflection: field not found")
	ErrFieldType     = errors.New("reflection: field is not a string")
	ErrNotSettable   = errors.New("reflection: field is not settable")
	ErrMethodMissing = errors.New("reflection: method not found")
	ErrArgCount      = errors.New("reflection: argument count mismatch")
)
```

再看写入本身，每一行注释对应一道必须过的关卡。摘录自 reflection-basics/main.go：

```go
func setString(ptr any, field, value string) error {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("set %q: %w", field, ErrNotPointer)
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("set %q: %w", field, ErrNotStruct)
	}

	fv := elem.FieldByName(field)
	if !fv.IsValid() {
		return fmt.Errorf("set %q: %w", field, ErrFieldMissing)
	}
	if fv.Kind() != reflect.String {
		return fmt.Errorf("set %q: %w", field, ErrFieldType)
	}
	if !fv.CanSet() {
		// Unexported fields are addressable but not settable.
		return fmt.Errorf("set %q: %w", field, ErrNotSettable)
	}

	fv.SetString(value)
	return nil
}
```

写入路径的五道关卡——非 nil 指针 → 结构体 → 字段存在 → 类型匹配 → 可设置（`CanSet`）——一道都不能省。**只查一个 `CanSet` 不等于所有路径安全**：`FieldByName` 找不到字段时返回的是无效 Value，直接调 `Kind` 或 `CanSet` 之前必须先过 `IsValid`。`%w` 包装让调用方可以写 `errors.Is(err, ErrNotSettable)` 来做分支处理，这是第 5 章错误处理约定在反射场景的落地。

### 反射调方法：`MethodByName` + `Call`

最后一块能力：按名字动态调用方法。`callMethod` 检查方法存在和参数个数，然后逐个装箱参数、调用、拆箱返回值。摘录自 reflection-basics/main.go：

```go
func callMethod(receiver any, method string, args ...any) ([]any, error) {
	m := reflect.ValueOf(receiver).MethodByName(method)
	if !m.IsValid() {
		return nil, fmt.Errorf("call %q: %w", method, ErrMethodMissing)
	}
	if m.Type().NumIn() != len(args) {
		return nil, fmt.Errorf("call %q: want %d args, got %d: %w",
			method, m.Type().NumIn(), len(args), ErrArgCount)
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}

	out := m.Call(in)
	results := make([]any, len(out))
	for i, rv := range out {
		results[i] = rv.Interface()
	}
	return results, nil
}
```

注意这份实现的防御边界**故意只画到参数个数**：nil 接收者、参数类型不匹配仍会在 `Call` 里 panic。`m.Type().NumIn()` 只数了数量，`reflect.ValueOf(arg)` 并没有校验每个参数能否赋给形参类型。复用它时必须补上逐参数的类型检查（`arg.Type().AssignableTo(paramType)`），否则就要用 `recover` 兜底——这是反射调用的通行成本。

### 什么时候才轮到反射

| 场景 | 用反射？ | 理由 |
| --- | --- | --- |
| 序列化 / 反序列化（JSON、YAML） | ✅ | 类型由数据决定，编译期不可知 |
| 配置绑定、命令行参数填充 | ✅ | 字段集合由用户的结构体定义决定 |
| ORM、校验器、依赖注入框架 | ✅ | 框架要在任意用户类型上工作 |
| 框架适配层（HTTP handler 包装等） | ✅ | 签名多样，边界处一次性转换 |
| 已知类型的业务逻辑 | ❌ | 直接写具体类型，编译器帮你挡错 |
| "同一份逻辑服务几种数值类型" | ❌ | 泛型在编译期解决，见 7.2 |
| 高频热路径 | ❌ | 反射调用有装箱和方法查找开销，先 benchmark 再决定 |

一句话收束：**反射的位置在边界层**——序列化、配置绑定、ORM、框架适配。业务代码仍接收明确类型，不把每一次普通调用变成运行时猜测。

[reflection-basics](../../../labs/part1-golang/chapter07/reflection-basics) 演示了读类型、遍历 tag、指针改字段、动态调方法四件事（仓库根目录运行）：

```bash
go -C labs run ./part1-golang/chapter07/reflection-basics
```

```text
float64(float64)
User(struct)
[]int(slice)
json tags: [Name=name Age=age]
after set: name=new age=20
Add(3,4) = 7
```

输出的前三行正好对应前面 `Name`/`String`/`Kind` 的语义表：`3.14` 的名字和 Kind 都是 `float64`，`User` 是 Kind 为 `struct` 的命名类型，`[]int` 没有名字、只能靠 `String()` 兜底。

<a id="generics"></a>

## 7.2 泛型

**泛型把"同一份逻辑服务多种类型"放在编译期完成**，不用退回 `any` 加运行时断言。没有泛型时只剩两条路：复制代码（每种类型一份，改 bug 要改 N 遍）或 `any` + 反射（丢掉类型信息，错误拖到运行期）。[generics-basics](../../../labs/part1-golang/chapter07/generics-basics) 覆盖三类最常见用法：

| 用法 | 示例 | 解决的问题 |
| --- | --- | --- |
| 泛型函数 | `sum`、`mapSlice`、`first` | 一套逻辑服务多种元素类型 |
| 类型约束 | `number`、`comparable` | 限定允许的操作 |
| 泛型容器 | `set[T]` | 复用数据结构而不丢类型信息 |

下面逐个讲透。注意 lab 里全部是小写未导出名（`sum`、`set`），正文引用时保持与源码一致。

### 类型约束：决定类型参数能做什么

**约束（constraint）决定类型参数能做什么**：只有约束中所有成员都支持的操作，才能用在类型参数上；`any` 不保证加法和排序。先看约束定义，摘录自 generics-basics/main.go：

```go
type number interface {
	~int | ~int64 | ~float64
}
```

再看使用它的 `sum`，同样摘录自 generics-basics/main.go：

```go
func sum[T number](values []T) T {
	var total T
	for _, value := range values {
		total += value
	}
	return total
}
```

`total += value` 能编译，恰恰因为 `number` 约束里每个成员都支持 `+`——这是"约束即能力清单"的直接体现。两个细节：

- `~int` 接受**底层类型**为 int 的自定义类型（如 `type Score int`）；写成单独的 `int` 则只允许 int 本身。手写业务代码几乎总要带 `~`，否则调用方自定义的命名单类型全被挡在门外。
- `var total T` 拿到 T 的零值作为累加起点——泛型函数里"零值"依然良定义，这是 `(T, bool)` 惯用法的基础。

### 多类型参数：`mapSlice` 的输入输出解耦

类型参数可以有多个，各自独立推导。`mapSlice` 的输入元素类型 `T` 和输出元素类型 `R` 是**两个**类型参数，靠转换函数 `func(T) R` 关联。摘录自 generics-basics/main.go：

```go
func mapSlice[T any, R any](values []T, convert func(T) R) []R {
	results := make([]R, 0, len(values))
	for _, value := range values {
		results = append(results, convert(value))
	}
	return results
}
```

调用处不需要写出任何类型实参——编译器从 `names`（`[]string`）推出 `T = string`，从匿名函数的返回值推出 `R = int`。**类型推导让泛型函数的调用形态和普通函数没有区别**，这是它和"手写 `any` 加断言"在可读性上的分水岭。

### 零值与边界：`first` 的 `(T, bool)` 约定

泛型不替代边界判断。`first` 沿用第 4 章的"结果 + ok"惯例处理空输入。摘录自 generics-basics/main.go：

```go
func first[T any](values []T) (T, bool) {
	if len(values) == 0 {
		var zero T
		return zero, false
	}
	return values[0], true
}
```

把 `any` 换成类型参数不会自动修复 `values[0]` 的越界——泛型把类型关系交给编译器，边界条件仍然是你的责任。`var zero T` 在 T 是任何类型时都有定义（int 得 0、string 得 ""、指针得 nil），这让 `(T, bool)` 成为泛型时代的通用失败返回形态。

### 泛型容器：`set[T]` 与 `comparable`

**`comparable` 让泛型值可以安全用作 map key**——但它只保证可比较性，不负责初始化 map。先看类型定义，摘录自 generics-basics/main.go：

```go
type set[T comparable] struct {
	items map[T]struct{}
}
```

再看构造函数和方法——构造函数分配 map，方法操作 map，摘录自 generics-basics/main.go：

```go
func newSet[T comparable](values ...T) set[T] {
	s := set[T]{items: make(map[T]struct{}, len(values))}
	for _, value := range values {
		s.add(value)
	}
	return s
}

func (s set[T]) add(value T) {
	s.items[value] = struct{}{}
}

func (s set[T]) contains(value T) bool {
	_, ok := s.items[value]
	return ok
}
```

一个值得停下来看的设计：`add` 是**值接收者**，却能改到调用方手里的 set——因为复制的只是 map 头部，头部里的指针仍指向同一个底层散列表（第 2 章"切片/map 是头部结构"的同一个道理）。这不意味着值接收者总是对的：如果改成"nil map 时延迟初始化"，`s.items = make(...)` 这步赋值就只对副本生效，调用方永远拿不到新 map——所以这份实现把分配集中在 `newSet` 里，值接收者只做写入。这也是为什么说 `comparable` 不负责初始化：**保证 key 可比较和保证 map 已分配是两件事**，前者编译器管，后者你的构造函数管。

### 泛型、接口、反射怎么选

| 你的处境 | 首选 | 原因 |
| --- | --- | --- |
| 逻辑同构、类型不同（容器、map/filter/sum） | 泛型 | 编译期实例化，零运行时类型开销 |
| 行为抽象、"能做什么"（`io.Reader`、`error`） | 接口 | 动态派发，调用方与实现解耦 |
| 类型由数据或用户代码决定（序列化、框架） | 反射 | 编译期确实不可知 |
| 只有两三种具体类型 | 直接写两三个函数 | 泛型的抽象成本可能高于复制 |

另外提醒：泛型的性能收益不是自动的——编译期实例化省去装箱和断言，但具体是否更快要 benchmark 验证；为"快"而泛型化之前先测量。

运行（仓库根目录）：

```bash
go -C labs run ./part1-golang/chapter07/generics-basics
```

```text
score total: 45
price total: 20.0
name lengths: [2 8]
first score: 10
contains cache: true, unique services: 2
```

`score total: 45` 和 `price total: 20.0` 来自同一个 `sum` 的两次实例化；`unique services: 2` 来自 `newSet("cache", "database", "cache")` 的去重——重复的 `"cache"` 在 map 里只占一个 key。

## 踩坑清单

### 反射修改却传了值

```go
user := User{Name: "old"}
_ = setString(user, "Name", "new") // 反例：传的是值副本，不是可写地址
```

症状：返回 `ErrNotPointer`（少了指针检查的写法则直接 panic），Name 保持 old。原因：`reflect.ValueOf` 拿到的值副本不可寻址，`Elem` 无从谈起。修复：传 `&user`，写入前逐层确认指针、结构体、字段存在与 `CanSet`。

### 对 nil 接口值直接用反射结果

症状：`reflect.TypeOf(nil)` 返回 nil，再调 `Name` 或 `Kind` 直接 panic；`reflect.ValueOf(nil)` 得到无效 Value，调任何方法都 panic。原因：nil 接口值连动态类型都没有，反射无从拆包。修复：先判 `t == nil` / `Value.IsValid()`。typed nil 指针又是另一种情况——类型在、值是 nil，先查 `Kind` 确认是指针类（Pointer、Slice、Map 等）再用 `IsNil` 判断。

### 已知类型却硬上反射

```go
// ❌ 反例：只想处理 int，类型错误被拖到运行期
func printInt(v any) {
	rv := reflect.ValueOf(v)
	fmt.Println(rv.Int())
}

// ✅ 正例：编译器在编译期拒绝错误类型
func printInt(v int) {
	fmt.Println(v)
}
```

症状：反例传入字符串在运行期才 panic，而且 panic 现场离真正的传错位置可能隔了好几层调用。原因：`any` 放弃了编译期类型检查。修复：参数直接写 `int`。反射是边界层的工具，不是业务代码的默认姿势。

### 约束写成 any 却想做受限操作

```go
func Sum[T any](values []T) T {
	var total T
	// 反例：T 是 any，编译器根本不知道是否允许 +
	// total += values[0]
	return total
}
```

症状：放开注释后编译失败——这不是运行期坑，是编译器在替你工作。原因：`any` 作为约束不承诺任何操作。修复：约束改成 `number` 这类带 `~` 的类型集合，配合完整遍历求和。

### 给零值泛型容器直接写 map

症状：`var s set[string]; s.add("x")` 报 "assignment to entry in nil map" panic。原因：`comparable` 只保证键可比较，不分配 map；零值 `set` 的 `items` 是 nil map。修复：用构造函数分配（lab 中的 `newSet`），或者把 `Add` 改成指针接收者并在写入前做 `if s.items == nil { s.items = make(...) }` 延迟初始化——注意延迟初始化必须配指针接收者，否则新 map 只活在副本里。

### 动态调方法只数参数个数

症状：`callMethod(calc, "Add", 3, "4")` 参数个数正确，却在 `Call` 里 panic。原因：`NumIn` 只校验数量，`reflect.ValueOf(arg)` 不校验每个实参能否赋给形参类型；nil 接收者同样会炸。修复：复用 `callMethod` 前补上逐参数类型检查（`AssignableTo`），或在调用方 recover。

### 以为泛型化之后就不用手动边界检查

```go
func First[T any](values []T) T {
	return values[0] // 反例：空切片照样越界 panic
}
```

症状：空输入在运行期 panic。原因：泛型管的是类型关系，不管切片长度。修复：沿用 `first` 的 `(T, bool)` 约定，空输入返回零值加 false。

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| <a id="reflection-exercises"></a>7.1 验证不可设置的输入 | 对 `User` 值（非指针）调用 `setString`，补齐断言；再用 `errors.Is` 断言返回的错误链上挂着 `ErrNotPointer` | 返回错误而非 panic，`user.Name` 保持 `"old"`；把输入换成 `&User{}` 但字段名写错，断言换成 `ErrFieldMissing` 同样成立 | `reflection-basics/main_test.go` |
| <a id="generics-exercises"></a>7.2 用自定义数值类型求和 | 定义 `type Score int`，对 `[]Score{10, 20, 30}` 调 `sum`；在副本中把约束的 `~int` 改成 `int` 观察差异后恢复 | 求和得 `Score(60)`；去掉 `~` 后编译失败，体会 `~` 接受底层类型匹配的自定义类型 | `generics-basics/main_test.go` |

完成后运行：

```bash
go -C labs test ./part1-golang/chapter07/reflection-basics/... -count=1
go -C labs test ./part1-golang/chapter07/generics-basics/... -count=1
```

更多练习（tag 字段遍历、values 方法实现、first 边界对比）见各 lab 的 README。

## 链接

- [Go 规范：接口类型](https://go.dev/ref/spec#Interface_types)与[类型参数声明](https://go.dev/ref/spec#Type_parameter_declarations)：接口约束、类型集合与 `~` 的准确定义。
- [reflect 包文档](https://pkg.go.dev/reflect)：`TypeOf`/`ValueOf`、`CanSet`、`MethodByName` 的完整契约。
- 《Go 入门指南》对应章节成书早于 Go 1.18，适合作为"泛型出现前 Go 如何做通用抽象"的背景阅读：[接口](https://learnku.com/docs/the-way-to-go/what-is-the-111-interface/3647)、[类型断言](https://learnku.com/docs/the-way-to-go/113-type-assertion-how-to-detect-and-transform-the-type-of-interface-variables/3649)、[type-switch](https://learnku.com/docs/the-way-to-go/114-type-judgment-type-switch/3650)、[反射器](https://learnku.com/docs/the-way-to-go/1110-reflector/3656)、[空接口](https://learnku.com/docs/the-way-to-go/119-empty-interface/3655)、[接口和动态类型](https://learnku.com/docs/the-way-to-go/1112-interface-and-dynamic-type/3657)。

[返回全书目录](../README.md) · [上一篇：结构体与方法](chapter06.md) · [下一篇：包和工具](chapter08.md)
