# 实验：提交窗口与进程中断恢复

在真实临时文件中保存身份、计划指纹、游标、结果与尝试次数。工具仅返回固定只读资料，写入只发生在实验自己的恢复文件和测试调用记录中。默认不访问网络。

不是分布式租约、外部exactly-once或断电可靠性证明；也不是完整Agent状态恢复。

## 前置与入口

阅读[Checkpoint正文](../../../../content/book/part3-agent/runtime/chapter29-execution.md#topic-agent-checkpoint)、Runtime、Session与幂等概念。Go版本以[labs/go.mod](../../../go.mod)为准；os.Root要求对应工具链，准备见[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：有界严格载入、文件同步与替换、排他领取、在途记录及结果提交。
- [main_test.go](main_test.go)：真实子进程硬退出、未知窗口、重复查询、游标继续、身份、预算、锁与取消。

## 预测与运行

两个动作read-A/read-B。工具完成后保存结果之前退出，与保存结果之后退出，恢复时read-A分别会执行几次？

```bash
go -C labs run ./part3-agent/chapter25/checkpoint
go -C labs test ./part3-agent/chapter25/checkpoint -race -count=1
go -C labs test ./part3-agent/chapter25/checkpoint -cover
go -C labs build -o /tmp/checkpoint-demo ./part3-agent/chapter25/checkpoint
```

默认演示使用自动创建的私有临时目录，完成后移除该实验目录。输出phase=completed、cursor=2、results=2、attempts=2。

新状态格式Schema=2保存MaxAttempts；同Run恢复配置必须与已存上限相同，增大或减小均返回ErrState，不执行动作或改写正式状态。Attempts累积到原上限时返回ErrBudget，重启不重新分配额度。CLI上限仍为8，固定输出不变。

Schema1旧状态可载入查看，但因没有可信的原上限而拒绝Run恢复。保留原目录与记录；不要删除状态重跑，或直接改Schema/MaxAttempts以绕过检查。格式迁移需有原预算依据，本Lab尚无迁移命令。

需要观察同一状态的续跑时，先创建自己拥有的私有目录，再使用`-dir /path/to/private-directory`；目录须已存在，不将它指向源码、共享数据或其他应用状态。再次运行已完成状态，不再执行动作。模型和工具不能提供文件路径。

## 硬退出观察

TestHardExitBeforeAndAfterResultCommit启动真实测试子进程，在after_execute或after_commit以退出码93结束。测试等待进程退出，再处置遗留writer.lock，重新读取状态。

| 位置 | 初次退出后的状态 | 明确允许只读重做后 |
| --- | --- | --- |
| after_execute | cursor=0，phase=in_flight，attempts=1 | read-A共执行2次，read-B1次 |
| after_commit | cursor=1，已保存read-A | read-A共执行1次，read-B1次 |

关闭readonlyReplay后，in_flight返回ErrUnknown，不执行恢复动作。策略开关属于可信应用，不由模型决定；只读重做仍可能增加费用或读到新资料版本。

## 替换前退出与临时文件残留

```bash
go -C labs test ./part3-agent/chapter25/checkpoint -run '^TestHardExitBeforeRename' -race -count=1 -v
```

测试先提交pending基线，真实子进程在临时文件同步/关闭后、Rename前以93退出。Wait确认终止后，正式文件仍为cursor=0、attempts=0；checkpoint.tmp存在，calls.log不存在，工具还没执行。只处置writer.lock再运行仍因临时名已存在而失败，且没有工具调用；显式处置候选文件后才能从基线完成两个只读动作。

beforeRename为私有可信故障钩子，正常CLI为nil；不是模型可调用功能。不自动偷锁、删除或提升临时候选。测试处置的均是自己的临时目录，真实部署先保留调查所需文件，再核对身份、提交协议与原执行者退出。不能按游标大小选择候选状态，也不能用这个进程实验证明断电持久性。

## 其他失败

- 第二个写者领取：ErrBusy，不自动偷锁。
- 不同租户/用户/会话/Run或动作指纹：ErrState，不执行。
- Schema不支持、游标/结果不一致：拒绝恢复。
- 重复JSON键（含嵌套、转义同键和大小写别名）、非法UTF-8字节或递归深度超过8（根节点按0计）：Load返回ErrState；字段名限定ASCII，正常中文结果仍可载入。未知字段、64KiB与尾随数据限制继续执行。
- 预算不足：ErrBudget；Attempts持久保留，重启不清零。
- 恢复上限与MaxAttempts不一致、上限非法或Attempts超过上限：ErrState；执行前拒绝。
- 取消发生在执行返回前：保留in_flight，不提交迟到结果。
- 结果提交后释放锁失败：仍返回可见错误；不能因phase=completed忽略清理失败。

临时文件残留或遗留锁需确认执行者已终止后按操作流程处理。测试不模拟断电；同步与原子替换需要对应平台/文件系统支持，错误不静默跳过。

## 费用命令客户端

[fee_client.go](fee_client.go)通过[公开费用JSON协议](../../chapter33/cost-ledger/README.md#固定json命令入口)调用固定程序，不导入其他topic私有实现。FeeCLI由可信应用绑定Program绝对路径、SHA256、已有私有费用Directory和执行Identity；代码不接收模型给出的程序或shell。

程序为规则指定来源构建的不可变产物，最多32MiB；每次执行前核对哈希与可执行文件。子进程最多10秒，stdout64KiB/stderr8KiB，不继承Provider环境。完整Tenant/User/Session/Run派生费用Run身份；仅解释fixture/v1/microcredit、限额100和单Call50的合成协议。

```bash
go -C labs test ./part3-agent/chapter25/checkpoint -list '^TestFee'
go -C labs test ./part3-agent/chapter25/checkpoint -run '^TestFee' -race -count=1 -v
```

真实测试在私有临时目录固定构建费用CLI，调用prepare两次仍ID1/Held50，begin后再打开变unknown，重复begin拒绝，settle40重报保持Used40/Held0。不同Session拒绝混用已有账本，哈希不匹配或已取消context不执行。独立反例覆盖错版本/动作/身份/请求/条目/汇总、重复/尾随/非法UTF-8/超限JSON、退出码矛盾和成功却非sent。

FeeReply与FeeSnapshot是公开协议的DTO，校验固定费用语义，不提供通用计价或文件认证。ErrFeeProtocol表示格式/配置/结果不可信，ErrFeeOperation表示已解析的费用拒绝；context错误保留，原输出与路径不拼进错误。合法错误可用于诊断，但不能作为继续执行的成功回应。

哈希与执行之间仍有窗口，调用者必须保护构建产物；不要从收到的响应获得新的Program/Digest。下面的固定联合样例接入这个客户端，客户端测试本身仍不替代联合故障验收。

## 联合恢复：执行证据与费用交接

[joint.go](joint.go)的RunJoint使用joint.json/joint.tmp保存完整Identity、原计划/尝试上限、游标、结果与每次调用；复用本Lab的单写者锁与原子保存方法。原Run仍使用checkpoint.json/Schema2，默认demo不变。费用经固定CLI协议单独提交，不import费用topic私有代码。

```bash
go -C labs test ./part3-agent/chapter25/checkpoint -list '^TestJoint'
go -C labs test ./part3-agent/chapter25/checkpoint -run '^TestJoint' -race -count=1 -v
```

测试在私有目录固定构建费用程序，固定只读动作与合成usage40，费用限额100/预留50。TestJointCrashWorker只由父测试的八个真实子进程启用；它以93退出，父进程Wait确认后仅处置自己执行目录的writer.lock，费用CLI在对应命令返回前已退出。

| 退出阶段 | 恢复行为 | 最终真实工具日志/费用 |
| --- | --- | --- |
| after_intent / after_fee_prepare / after_binding | 完成同一次幂等准备并执行 | 一次read，Used40/Held0，只有一个费用条目 |
| after_fee_begin | 缺结果，拒绝直接重发 | 零次read，unknown/Held50 |
| after_execute | 已调用但无已存返回证据，保留未知 | 一次read，unknown/Held50 |
| after_receipt / after_fee_settle / after_result_commit | 只补结算或结果提交 | 一次read，Used40/Held0，结果与游标各一份 |

另验证只读重做：初次发送意图确认后取消，原上限2恢复，新尝试执行成功；旧预留50保留、新费用40已知，只有两条费用记录。把上限改成3拒绝。tool_failed或取消返回已知usage20时仍结算20，不提交迟到文本；unknown usage保持Held50；超额usage110真实入账并冻结，执行结果不标成功。已确认费用记录丢失时，恢复拒绝，不重新查询工具或伪造零成本。

RunJoint的read回调为可信只读Go代码，返回JointResponse（Text、可空InputUsage、稳定Failure）；Failure允许tool_failed/canceled/deadline。usage nil表示未知，明确0用指针表示。replayReadonly由应用声明，未知和普通工具错误可在原尝试/费用配额内新建尝试；取消、超额不自动重试。每次恢复核对费用关联，返回证据不等于任务目标已达成。

恢复audit使用费用lookup核对key/hash→EntryID，而不只比较金额和状态。TestJointRejectsWrongFeeAttemptBinding先建立两条同为40的已结算记录，再让执行结果误指向第二条：恢复返回ErrState，不重复工具。lookup不存在不新增预留；它通过公开协议查询映射，未导入费用私有代码。

本实验没有真实LLM、Provider账单、动态工具参数、并行Loop、分布式所有权或断电证明。迁移任务继续把原模型提议与ToolCallID、工具/资料/策略版本接入同一记录，再按任务Eval判断完成；当前八处固定故障不代替完整四类作品或生产验收。

## 迁移练习

在自己的临时目录与副本中完成迁移。硬退出用测试自有子进程，不杀共享服务；未知状态与旧记录先保留，再按显式恢复政策处理。

| 任务 | 起点与改动 | 反例与完成证据 |
| --- | --- | --- |
| 1. 对照未知窗口 | 从Run的hook与TestHardExitBeforeAndAfterResultCommit起步，在after_inflight及工具已返回、结果未保存两处退出 | 分别记录实际工具次数与保存状态：两者都可能只留下in_flight，但不能据此断言都未执行。新增窗口验证应检查原执行事实、恢复是否再次调用及累计Attempts，不只看游标 |
| 2. 核查外部结果 | 在Run遇到ErrUnknown的位置设计可信结果查询或幂等键；绑定执行身份、动作参数与来源版本，不开放副作用工具 | 自行构造同金额/文本但错调用身份的返回、查询失败及确实匹配三种情况；只有可信匹配才能补提交。查询失败保持未知，批准只读重做另记新尝试，不自动偷锁或清零预算 |
| 3. 扩展配置指纹 | 从fingerprint及validate起步，将新增参数、工具与资料版本纳入绑定；写清Schema兼容和迁移依据 | 同名动作换参数/版本后旧状态拒绝，未知Schema不继续执行；新增状态格式须保留旧记录，不能直接改字段使它通过。指纹匹配不认证来源，也不能证明外部结果真实 |
| 4. 迁移所有权 | 对照Store.claim的单写者行为，先设计数据库CAS/租约、领取epoch与状态revision的提交条件，再实现独立测试存储 | 两实例竞争、租约失效后旧worker迟到提交、新worker接续各有用例；所有权校验与结果提交在同一强制边界完成。文件锁PASS或只检查过期时间，不证明fencing成立 |
| 5. 扩展执行与费用交接 | 从State.MaxAttempts、RunJoint及fee_client.go起步，在已有固定金额/未知费用联合基线上新增动态调用身份或一种缺失usage路径；继续使用固定公开协议，不导入费用topic私有实现 | 对照执行返回、观察提交、费用结算的中断窗口，核对累计尝试、Held/Used和原调用关联；重复恢复不重复释放/结算，未知不填零。八处固定故障只作回归，新增路径须有独立记录和最终任务判断 |

先核对基线名称，再执行：

```bash
go -C labs test ./part3-agent/chapter25/checkpoint -list 'TestHardExitBeforeAndAfterResultCommit|TestIdentityLockSchemaAndBudget'
go -C labs test ./part3-agent/chapter25/checkpoint -run '^(TestHardExitBeforeAndAfterResultCommit|TestIdentityLockSchemaAndBudget)$' -race -count=1 -v
```

新增测试由学习者编写，并先用-list核对名称。原基线、固定恢复、真实模型usage与最终任务效果分别验收；no tests to run、全跳过或恢复返回nil均不等于新路径已完成。

## 来源与许可

原创代码，MIT。[os](https://pkg.go.dev/os)用于核对Root、Sync与Rename；所有临时状态不提交仓库。

## 正文中的恢复判断对应的运行观察

硬退出测试在工具完成后和结果提交后分别终止子进程，再重新读取状态。前一种情况下，只读恢复会再次执行首个查询；关闭重做策略则返回结果未知。后一种情况下，首个结果保留，从第二项继续。

这证明相应进程中断窗口与重复风险，不证明外部服务支持幂等，也不证明断电后磁盘状态。失败测试与生产可靠性证据要区分，不能把几次重启成功作为全部恢复能力。

<a id="book-agent-checkpoint-最佳实践"></a>

## 最佳实践

## 最佳实践

### 提交单位围绕恢复判断设计

结果与游标一起提交，来源和版本保持关联。大产物先确保可以持久读取，再提交引用，避免游标前进后产物丢失。

### 未知结果先分类

可查询外部状态时先查；有可信幂等契约时按契约重试；只读重做明确版本和预算；无法确认且有副作用时停下处理。不要给所有失败套相同重试函数。

### 恢复预算也属于任务

持久保留尝试与消耗，避免每次重启获得新的无限配额。模型usage未知时标记未知，而非因为Checkpoint没有数字就填零。


<a id="book-agent-checkpoint-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 重启后重复执行首个动作

先检查退出位置，再核对结果与游标是否一起提交。如果只有in_flight，应按未知窗口处理；不能仅凭“上次日志说执行完成”跳过提交检查。

### 锁文件存在，恢复无法开始

确认对应执行者是否仍活着，以及是否存在另一个合法写者。确认退出后按明确操作流程处置；不要根据文件年龄自动删除，避免两个执行者同时运行。

### 新版本读取旧状态后结果异常

核对Schema、参数指纹、工具/资料版本与迁移规则。拒绝不支持的状态并保留调查依据，不重新从零执行副作用来掩盖兼容失败。

