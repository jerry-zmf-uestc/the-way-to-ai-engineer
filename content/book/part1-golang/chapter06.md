# 第 6 章：结构体与方法

Go 没有类继承体系：结构体保存状态、方法表达行为、接口约定调用方需要的能力，三者的组合就是 Go 的主要建模方式。

从 Python 或 Java 转过来，第一反应往往是找 `class`。设想把一个 Python 写的账户服务迁到 Go：原代码里 `class Account` 一把抓——字段、`deposit`/`withdraw` 方法、`__str__`，报表模块靠继承一个 `Formatter` 基类拿到多态。到了 Go，这三样东西要拆开声明、再按规则组合；迁移期最典型的两类迷惑——"方法明明执行了，余额为什么没变""接口明明不是 nil，为什么一调用就 panic"——全都来自把 class 的心智模型直接套过来。本章用一个可运行的账户例子把这三层各自的规则讲透。

<a id="structs-interfaces"></a>

## 6.1 结构体、方法与接口

`labs/part1-golang/chapter06/structs-methods-interfaces` 用一个账户例子串起三层：`Account` 保存 `Owner` 和 `Balance`（数据层），`Deposit`、`Withdraw`、`Summary` 把账户动作贴到类型上（行为层），`Formatter` 表示"只要能 Format，就能被调用方接受"（边界层）。后面的 HTTP handler、RPC service 和 repository 都是这套写法。

### 结构体：先把状态聚合起来

结构体只负责把相关字段聚合成一个类型，不携带任何行为——大致相当于 Python 的 dataclass 或 TS 里纯数据的 interface，但内存布局是确定的：

```go
// 摘录自 structs-methods-interfaces/main.go。
type Account struct {
	Owner   string
	Balance int
}
```

用字面量初始化时，没写出来的字段取零值：

```go
// 摘录自 structs-methods-interfaces/main.go（main 函数节选）。
// Balance 未写出来，取零值 0
account := Account{Owner: "Ada"}
```

零值可用是 Go 的重要约定：`Account{}` 不是"半成品"，而是一个余额为 0 的合法账户，不需要构造函数兜底。

### 会改状态的方法，用指针接收者

**接收者类型决定方法改的是原件还是副本**：要改状态就用指针接收者。

```go
// 摘录自 structs-methods-interfaces/main.go。
func (a *Account) Deposit(amount int) error {
	if amount <= 0 {
		return errors.New("deposit amount must be positive")
	}
	a.Balance += amount
	return nil
}
```

方法并不神秘：`account.Deposit(120)` 基本可以读作普通函数调用 `(*Account).Deposit(&account, 120)`，接收者就是第一个参数。值接收者拿到的是整个结构体的副本，改动随函数返回而丢弃；指针接收者拿到的是地址，改动落在原件上。第 4 章"Go 只有值传递"在这里依然成立——传指针也是值传递，拷贝的是地址本身。

`Withdraw` 展示了变更方法的完整校验顺序：

```go
// 摘录自 structs-methods-interfaces/main.go。
func (a *Account) Withdraw(amount int) error {
	if amount <= 0 {
		return errors.New("withdraw amount must be positive")
	}
	if amount > a.Balance {
		return fmt.Errorf("withdraw %d: balance %d", amount, a.Balance)
	}
	a.Balance -= amount
	return nil
}
```

先拒绝非正金额、再检查余额，两道校验都失败时**不碰状态**——余额在校验全部通过后才更新，失败路径没有副作用。错误消息带上金额与余额上下文，排查时不用翻日志猜数字。

接收者怎么选，按这张表判断：

| 情况 | 接收者 | 理由 |
| --- | --- | --- |
| 方法要修改字段 | 指针 | 值接收者改的是副本，改动丢失 |
| 结构体较大 | 指针 | 每次调用都拷贝整个结构体不划算 |
| 类型含锁等不可拷贝字段 | 指针 | 拷贝一份锁等于没锁 |
| 只读计算、结构体小 | 值 | 语义清晰：保证不改原件 |
| 同一类型内部 | 统一 | 见下文"值接收者的位置" |

### 方法集合：接口赋值看接收者类型

类型的**方法集合（method set）**是它能满足哪些接口的"能力清单"，规则只有两条：

| 类型 | 方法集合包含 |
| --- | --- |
| `T` | 所有值接收者方法 |
| `*T` | 值接收者方法 + 指针接收者方法 |

两条推论要分清：

- **调用时的自动取地址**：`account` 是可寻址变量时，`account.Deposit(120)` 编译器自动改写为 `(&account).Deposit(120)`，所以直接调用几乎感觉不到接收者差异。
- **接口赋值不做这种让步**：`var f Formatter = account` 要求 `account` 的类型本身的方法集合里就有 `Format`。如果 `Format` 声明在 `*Account` 上，这里编译直接报 `Account does not implement Formatter (Format method has pointer receiver)`，必须写 `&account`。

**自动取地址 ≠ 接口的方法集合**：可寻址的 Account 变量可以调用指针接收者方法，但不能据此推断 Account 值实现了只由 *Account 满足的接口。把接收者改成指针后，要检查接口赋值处，而不仅是直接调用处。

### 值接收者的位置：只读、不共享

值接收者不是错误写法，它适合"读一下就还回去"的方法：

```go
// 摘录自 structs-methods-interfaces/main.go。
func (a Account) Summary() string {
	return fmt.Sprintf("%s has %d credits", a.Owner, a.Balance)
}
```

`Summary` 用值接收者，签名本身就承诺"我不改账户"。实务上更常见的约定是：**一个类型只要有任何一个方法需要指针接收者，其余方法也用指针**，避免方法集合随方法不同而漂移。这个 lab 故意混用两种接收者，正是为了让上面的差异可见；正式代码里默认统一为指针，只有小而纯的值类型（如 `time.Time` 这类）例外。

### 接口约定能力，不定义继承层级

**接口只声明"需要什么能力"，谁实现都行。** Go 的接口是隐式实现的：类型不需要声明"我实现了某个接口"，方法集合对得上就算数——像 Python 的鸭子类型，但检查发生在编译期，写错了编译直接失败，不用等运行期 `AttributeError`。

```go
// 摘录自 structs-methods-interfaces/main.go。
type Formatter interface {
	Format() string
}

type AccountSummary struct {
	Account Account
}

func (s AccountSummary) Format() string {
	return s.Account.Summary()
}

func Format(formatter Formatter) string {
	return formatter.Format()
}
```

注意 `AccountSummary` 用的是**具名字段** `Account Account`，`Format` 里显式写 `s.Account.Summary()`——组合关系一目了然。也可以写成嵌入（embedding）形式 `type AccountSummary struct{ Account }`，让 `Account` 的方法提升到外层、可以直接 `summary.Summary()`；那是教学变体，本章示例源码保持具名写法。嵌入的方法提升**不是继承**，两者的差别见踩坑清单。

接口设计上两条对照：

```go
// ✅ 小接口，在调用方一侧定义：Format 函数只需要 Format() string 这一个能力
type Formatter interface {
	Format() string
}

// ❌ 大而全的接口，在实现方包里定义：实现者被迫填满无关方法
type AccountAPI interface {
	Format() string
	Deposit(amount int) error
	Withdraw(amount int) error
	Summary() string
	OwnerName() string
}
```

标准库就是按 ✅ 的样子来的：`fmt.Stringer` 只有一个 `String() string`，任何类型实现它就能被 `fmt.Println` 友好打印。接口越小、越靠近使用方，实现方和被实现方的耦合越低。

### 接口值 = 动态类型 + 动态值

接口类型的变量内部是两个字：一个**动态类型**、一个**动态值**。只有两个字都为空，接口才等于 nil。`DescribeValue` 把这件事变成可观察的输出：

```go
// 摘录自 structs-methods-interfaces/main.go。
func DescribeValue(v any) string {
	if v == nil {
		return "nil interface"
	}
	return "non-nil interface holding " + reflect.TypeOf(v).String()
}
```

main 函数末尾分别传入真正的 nil 和一个 typed nil：

```go
// 摘录自 structs-methods-interfaces/main.go（main 函数节选）。
fmt.Println(DescribeValue(nil))
var typedNil *Account
fmt.Println(DescribeValue(typedNil))
```

第二处传入的 `typedNil` 虽然是空指针，进入接口时动态类型字已被填成 `*Account`——**接口值本身非 nil，不代表里面的对象可用**（具体后果见踩坑清单）。

### 运行

完整示例在仓库根目录运行：

```bash
go -C labs run ./part1-golang/chapter06/structs-methods-interfaces
```

```text
Ada has 75 credits
nil interface
non-nil interface holding *main.Account
```

第一行对应 `Ada` 存入 120、取出 45 后的余额 75——`Deposit`、`Withdraw` 两次指针接收者调用都落在了同一个 `account` 上。后两行就是 `DescribeValue` 对 nil 接口与 typed nil 的不同回答。

`Withdraw` 已经检查非正金额与余额不足；正式应用处理金额还要约定单位、溢出和并发更新，这个账户示例只演示方法与接口。

## 踩坑清单

### 需要改状态却用了值接收者

```go
func (a Account) Deposit(amount int) {
	// 反例：这里改的是副本，不是外面的原始 Account。
	a.Balance += amount
}
```

现象：方法正常返回、没有报错，但调用结束后原对象的余额不会变。原因：值接收者拿到的是整个结构体的副本，副本随方法返回被丢弃。修复：接收者改为 *Account，改完检查接口赋值处——Account 值不再满足只由 *Account 实现的接口。

### 接口值持有 typed nil，接口本身仍非 nil

```go
var account *Account = nil
var v any = account

if v == nil {
	// 反例：这里不会进入，因为接口值已经带上了具体动态类型 *Account。
}
```

现象：接口是不是 nil 要看动态类型和值是否都为空，typed nil 过不了 `v == nil` 检查，而方法若解引用接收者仍会 panic。原因：把 `account` 装进接口时，动态类型字被填成 `*Account`，接口值不再是"空"。修复：判断"接口非 nil"不能代替对象生命周期检查，调用前检查实际指针；在练习副本中分别比较 nil 接口与保存 (*Account)(nil) 的接口即可复现。

### 在 nil 接收者上调用方法

typed nil 的另一半：接口非 nil 的检查放行了，方法照样炸。

```go
var account *Account
fmt.Println(account.Summary()) // 反例：值接收者要先解引用，panic: runtime error: invalid memory address or nil pointer dereference
```

现象：运行期 panic，不是编译错误。原因：值接收者方法等价于 `Summary(*account)`，nil 解引用在进函数体之前就发生；即使换成指针接收者方法能进入函数体，访问 `a.Balance` 时同样 panic。修复：用构造函数或约定保证对象非 nil；确实要容忍 nil 的类型（如链表节点的递归遍历），在指针接收者方法入口先判 `if a == nil`，让 nil 成为有定义的行为而不是崩溃。

### 把嵌入当继承，期待"子类"覆盖生效

```go
// 教学变体：嵌入 + 同名方法遮蔽（lab 源码用的是具名字段 Account Account）。
type AuditSummary struct{ Account }

func (s AuditSummary) Summary() string { // 遮蔽提升的 Account.Summary，不是覆盖
	return "audited: " + s.Account.Summary()
}
```

现象：给嵌入类型定义同名方法后，外部调用 `AuditSummary{}.Summary()` 走的是新版本，但 `Account` 内部若有调用 `Summary` 的方法，走的永远是 `Account` 自己的版本。原因：嵌入的方法提升是编译期的静态转发，Go 没有虚方法表，不存在"父类方法调用子类实现"的动态分发。修复：需要多态就用接口——像本章的 `Formatter` 那样把行为显式声明出来，让调用方依赖接口而不是嵌入关系。

<a id="structs-interfaces-exercises"></a>
<a id="练习"></a>

## 动手练

| 练习 | 任务 | 预期结果 | 入口文件 |
| --- | --- | --- | --- |
| 6.1 改变 Summary 接收者 | 把 Summary 接收者改为 *Account，返回逻辑不变；同步检查 AccountSummary.Format 的调用和测试，不只改签名 | 现有调用和测试通过；能解释 Account 与 *Account 方法集合的差异——改动前先预测哪几处会报编译错，改完逐条核对 | `structs-methods-interfaces/main.go`、`main_test.go` |
| 6.2 让余额不足可以识别 | 声明 ErrInsufficientBalance；Withdraw 保留 amount<=0 检查，余额不足时返回包装它的错误并带上金额与余额上下文；测试覆盖不足、负数和正常取款 | errors.Is 能识别不足；失败前后余额不变，负数取款仍拒绝；测试对三类输入分别断言，不只断言"有错误返回" | `structs-methods-interfaces/main.go`、`main_test.go` |

完成后运行：

```bash
go -C labs test ./part1-golang/chapter06/structs-methods-interfaces/... -count=1
```

已有测试通过只说明已覆盖的用例符合预期；更多练习（实现另一个 Formatter 等）见各 lab 的 README。

## 链接

- [Go 规范：类型与方法集合](https://go.dev/ref/spec)：方法集合与接口类型检查的准确定义。
- [fmt 包：Stringer 接口](https://pkg.go.dev/fmt#Stringer)：标准库最小接口的样本——只声明一个 `String() string`，`fmt.Println` 自动使用它。
- [结构体定义](https://learnku.com/docs/the-way-to-go/101-structure-definition/3638) 与 [匿名字段和嵌入结构体](https://learnku.com/docs/the-way-to-go/105-anonymous-field-and-embedded-structure/3643)：结构体语法与嵌入写法的展开。
- [方法](https://learnku.com/docs/the-way-to-go/106-method/3644) 与 [什么是接口](https://learnku.com/docs/the-way-to-go/what-is-the-111-interface/3647)：接收者语义与接口机制的展开。

[返回全书目录](../README.md) · [上一篇：错误处理与异常](chapter05.md) · [下一篇：接口、泛型与反射](chapter07.md)
