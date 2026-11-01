# Async Tasks：独立任务状态与后台执行

本实验解决请求返回以后，如何继续查、改、停一个后台任务。使用真实goroutine和标准库，不调用模型、网络或业务写工具。前置：[Delegation](../../../../content/book/part3-agent/runtime/chapter31-collaboration.md#topic-agent-delegation)、[DAG](../../../../content/book/part3-agent/runtime/chapter31-collaboration.md#topic-agent-dag)、Go context与锁。

## 机制与观察点

- Task不引用Session消息。Start立即返回Scope/ID，Check/List读取当前记录，Wait只是测试/程序收取终态的辅助方法。
- Manager限制1–8个在途任务、16条保留记录，输入最多4096字节、输出最多8192字节。没有隐藏等待队列，满额直接拒绝。
- 同Scope同幂等键同初始输入返回同ID；换输入拒绝。幂等键在任务TTL清理后失效，同键可创建新ID。
- Update是**替换输入并中断重跑**，不是追加完整会话历史。仅允许一次更新，Task ID保持不变，Revision递增，旧attempt退出后才开始下一次。总超时不因更新重置。
- Cancel返回canceling；执行器确认尚未启动或已有worker退出后，才是canceled并释放名额。入口已可见的取消/到期不调用worker，Attempt可为0；首次调用前更新可以直接执行新版本，Attempt为1。忽略context的worker无法被Go强行抢占，Close同样必须等已调用的worker退出。
- Worker自己的子context错误另记worker_canceled或worker_deadline_exceeded，不混同显式Cancel或任务总时限；先登记Update时，旧attempt的取消仍先按版本重跑处理。TestWorkerChildContextCategoriesAndCapacity验证终态、无残余输出、无包装诊断和退出后名额可复用。
- Worker错误只公开稳定分类，不公开原始错误；实验不采集私有错误日志。completed只表示非空有界输出通过本地提交，不代表事实正确。
- 一个Manager在单进程中使用；记录、ID序号与幂等索引均不持久化。重新创建Manager不支持旧ID恢复。Cleanup应由可信应用维护过程调用，不由模型指定时间。

## 从仓库根目录运行

Go及依赖准备见[运行说明](../../../../content/book/introduction.md#running-examples)。

```bash
go -C labs run ./part3-agent/chapter31/async-tasks
```

输出：

```text
task=task-1 state=completed revision=1 output="evidence: timeout"
```

先预测：取消返回以后，是否可以立即启动另一任务？更新以后，旧文本是否仍可能成为最终结果？随后运行：

```bash
go -C labs test ./part3-agent/chapter31/async-tasks ./internal/tasks -race -count=1 -v
```

测试应全部PASS。通道控制旧worker何时收到取消、何时退出，验证更新与取消不会释放尚在工作的名额；更新/提交竞争允许两种合法先后顺序，不把某种随机调度当作正确答案。另验证启动重试、跨用户查询、首次执行前更新、超时迟到结果、TTL及关闭拒绝。

[entry_test.go](../../../internal/tasks/entry_test.go)在同包建立与Start相同的记录、context和等待计数，直接检查确定的入口边界：先取消/到期时零回调和Attempt=0，先更新时只调用Revision=2一次；不依靠goroutine恰好迟启动。Start/Update竞争测试另计真实回调次数，与Attempt核对，允许旧执行已开始和未开始两种情况。检查之后仍可能取消，不能由这些测试推导原子阻止或抢占。

## 控制响应没有被调用方收取

[acknowledgement_test.go](acknowledgement_test.go)主动丢弃本地Update/Cancel成功返回，再提交旧版本请求并查询实际状态。真实Manager及channel控制worker退出；无网络故障注入，也不提供跨实例确认或持久控制操作账本。

```bash
go -C labs test ./part3-agent/chapter31/async-tasks -list 'TestUpdateWithoutAcknowledgement|TestCancelWithoutAcknowledgement'
go -C labs test ./part3-agent/chapter31/async-tasks -run 'TestUpdateWithoutAcknowledgement|TestCancelWithoutAcknowledgement' -race -count=1 -v
```

两个测试应实际执行并通过。更新测试确认旧版本重试冲突、当前记录为新输入/updating、原Start键和初始输入返回相同当前任务，旧worker退出后新版本只启动一次。取消测试确认旧版本冲突、查询可见canceling、同当前版本重复Cancel不改版本，退出前容量不释放且迟到输出不提交。

客户端不能自动换新版本重放修改：有其他控制者时，当前快照相符也不证明请求归属。远程确认需另设计操作身份与去重；本测试只验证当前同进程契约。

## 父目标与子结果准入

[parent.go](parent.go)保存trusted ParentGoal与应用私有ChildBinding；Start/Update返回时绑定父ID/Revision、Scope与childID/Revision，Wait后AdmitChild核对当前目标和结果。main接入该检查，原正常输出不变。

```bash
go -C labs test ./part3-agent/chapter31/async-tasks -run 'TestOldChildCompletion|TestUpdatedChild' -race -count=1 -v
```

[parent_test.go](parent_test.go)用真实Manager和受控worker：旧子任务仍completed时，父目标已增加Revision，旧绑定拒绝；原父目标仍可使用原结果。另一测试更新实际子任务，旧child版本绑定拒绝，新绑定接收this year结果。错childID/Scope/版本、running及Failure分别拒绝。测试无模型/网络，用channel确定顺序，不依赖sleep。

仅准入候选文本，不提供语义验证、父目标持久存储、跨进程身份、取消级联或原子客户端交付。当前目标与绑定由可信应用持有，不让模型构造；也不能将旧完成结果重新Bind到新目标来规避校验。原父取消/收齐练习仍待完成，本基线不代替新生命周期验收。

## 显式父停止与本组取消

父方先在应用状态关闭准入/增加当前目标Revision，再把旧父快照与私有bindings交给CancelBoundChildren。它整批拒绝错范围、错父版本和重复ID；运行期按原child版本Cancel，已更新则ErrConflict，不偷换版本。所有取消发送之后共享同一等待context；错误通过errors.Join保留，部分取消不回滚。

```bash
go -C labs test ./part3-agent/chapter31/async-tasks -run 'TestParentCancel|TestCancelBinding|TestUpdatedChild' -race -count=1 -v
```

[parent_cancel_test.go](parent_cancel_test.go)用channel确认两个worker进入：取消本组后另一父的任务仍running并可完成；重复取消可等待原终态。另一worker确认context取消后仍阻塞，父等待context停止时保留canceling与容量，不能抢占或提前释放。旧绑定取消实际更新后的child得到ErrConflict。没有用毫秒调度先后作为断言；真实跨服务/持久父注册与动作回滚不在该例中。

原父生命周期练习仍需学习者扩展父状态协调、完成/取消竞争与多子部分失败；新函数只是本组显式取消基线，不提供父数据库事务或完整练习答案。

## 练习

状态收取变体：在独立客户端副本使用真实Manager.Check与Wait，令worker通过channel等待。先让等待context到期，确认任务仍在执行；随后显式Cancel并等待退出，分别记录等待和任务终态。另设计最多3次的应用查询循环，在次数用尽时返回“尚未取得终态”，不得自动Cancel或重新Start。自行新增测试验证次数上限、迟到版本处理与任务仍可查询；现有状态测试仅作回归，远程通知协议不在本练习中假装实现。

| 任务 | 实际起点与新增行为 | 新验收与边界 |
| --- | --- | --- |
| 等待者离开，任务继续 | 从main.go和TestIndependentTaskRoundTrip起步，用channel固定worker等待，短等待结束后以新context收取 | 创建TestExerciseWaiterLeaves，核对等待错误、任务仍running、显式结束后可读；等待者context不得替代任务取消 |
| 扩展更新预算 | 从labs/internal/tasks/tasks.go的Update起步，在副本设计允许三次更新的有限策略，保留原life context | 创建TestExerciseThreeUpdatesShareDeadline，核对旧attempt退出后才开始新attempt、总deadline不重置、第四次更新拒绝；版本变更不代表已经执行 |
| 验证清理时序 | 从Manager.Cleanup及状态查询起步，分别在活跃期和终态TTL边界调用 | 创建TestExerciseCleanupBoundary，核对活跃任务不清理、过期ID不可读、旧幂等键到期后重新接纳产生新ID；不要用当前时间猜边界 |
| 给等待控制加HTTP入口 | 从Manager.Wait和Serving的可信Scope绑定方式起步，在独立handler新增等待超时响应映射 | 创建TestExerciseHTTPWaitTimeout，核对响应等待超时后后台任务仍可查询、跨Scope查询拒绝、连接断开不擅自Cancel；已有Serving入口只作参考，不重复实现接纳当作新任务 |
| 父任务取消或修改目标 | 从[parent.go](parent.go)的CancelBoundChildren、BindChild与AdmitChild起步，扩展父状态协调、完成/取消竞争和多子部分失败 | 创建TestExerciseParentCompletionRace，核对父版本变化先关闭旧结果入口、本组子任务收齐、其他父任务不受影响；Manager本身无Parent，现有显式取消基线不替代父生命周期协调 |
| 更新输入的语义 | 沿下文完整目标与增量请求的变体，实现自己的输入整理器 | 创建TestExerciseUpdatedTargetRetainsRequirements，核对年份变化、未变目标保留与旧Revision拒绝；字符串拼接不能证明语义合并正确 |


更新输入变体：从[labs/internal/tasks的Update](../../../internal/tasks/tasks.go)及本Lab的TestIndependentTaskRoundTrip起步，在调用层先保存完整目标。原任务同时要求超时、重试和来源，用户只提交“只看今年”；对照直接替换与调用层形成完整新目标两条路径。用固定worker捕获实际Task.Input，确认Manager不会自动保留旧要求。自行加入撤销旧年份、保留未变目标和重复旧Revision三种测试，不预先填好输入整理器。若整理使用模型，原目标/增量/实际新输入分别记录并独立核对，不能把字符串拼接当作语义合并已验证。

```bash
go -C labs test ./part3-agent/chapter31/async-tasks -list '^TestIndependentTaskRoundTrip$'
go -C labs test ./part3-agent/chapter31/async-tasks -run '^TestIndependentTaskRoundTrip$' -race -count=1 -v
```

这条既有基线验证任务往返，不验证新增输入整理。迁移测试由学习者创建，再先用-list核对筛选名称；新要求已登记、新worker已进入调用、当前结果已提交分别观察。当前Update不提供会话历史合并或持久恢复，原总时限、一次更新上限与权限边界继续保留。

迁移测试由学习者创建，仓库没有预填上述实现：

```bash
go -C labs test ./part3-agent/chapter31/async-tasks -list '^TestExercise'
go -C labs test ./part3-agent/chapter31/async-tasks -run 'TestExercise|TestIndependentTaskRoundTrip' -race -count=1 -v
```

只有原往返测试PASS不表示新任务已完成。修改共享tasks包时也须运行其原测试并做race验证；保留原取消、容量、版本与总期限边界，独立副本的变化不影响其他读者。

## 来源与许可

原创Go实现，MIT。教学结构参考[Deep Agents异步子Agent](https://github.com/datawhalechina/deepagents-in-action/blob/main/content/ch06-async-subagents.md)，未复制其代码、图片或框架API；这里的更新策略与其会话追加机制不同。底层语义见[Go context](https://pkg.go.dev/context)、[Go sync](https://pkg.go.dev/sync)。

Task Store实现与核心契约测试在[labs/internal/tasks](../../../internal/tasks/tasks.go)；本Lab只调用该稳定内部包，与Serving Lab分别独立启动，不依赖彼此运行状态。

<a id="book-agent-async-tasks-最佳实践"></a>

## 最佳实践

## 最佳实践

- **把任务状态作为交互依据**：主Agent通过Check/List获取状态，提示词压缩不影响查询；结果使用前再次核对当前版本和范围。
- **保留独立预算**：接纳、在途、保留记录、总时限、更新次数与模型费用分别限制；一次更新也计入整个任务消耗。
- **让worker协作退出**：context传给模型、工具与存储，阻塞操作有超时；不要在持有Manager锁时执行外部工作或等待退出。
- **先确定更新语义**：说明是否保留历史、能否重用旧证据、哪一步采用新要求；不能用“支持update”替代设计。
- **先测契约，再测收益**：固定worker验证状态与竞争，真实任务再比较等待体验、总延迟、成功率与费用；异步本身不提高推理能力。


<a id="book-agent-async-tasks-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

| 现象 | 先检查什么 | 原因与处理 |
| --- | --- | --- |
| 返回了ID，马上查不到 | 同一Manager实例与可信Scope | 身份范围错、实例不一致或过期；先验证本地Store，再查HTTP路由与鉴权 |
| 更新成功却返回旧答案 | Revision与结果提交顺序 | worker持有旧快照；结果提交必须核对版本，不能只检查err |
| 取消后任务仍占用名额 | 状态是否canceling、阻塞操作是否接受context | 尚未退出；修复下游超时和取消传播，不提前释放名额 |
| 相同请求创建多个任务 | 幂等键、初始输入及保留窗口 | 换键、跨范围或已清理；核对入口重试契约，而非让模型避免重复 |
| 刷新页面以后任务停止 | 是否用了请求context管理后台任务 | 等待与执行寿命混用；按应用生命周期和产品取消策略拆开 |
| 重启后旧ID失效 | 是否真的有持久存储与恢复所有权 | 同进程示例无恢复；补持久任务和Checkpoint，不能仅增加查询重试 |
| 主Agent不停查状态，用户无法继续说话 | Start后是否进入无界Check循环 | 把有限刷新交给应用；先返回任务身份，按需查询，不以更多查询驱动worker |

排错先直接调用Manager验证身份与状态，再核对worker边界，最后检查服务入口和模型决策。这样可以区分Store契约错误、下游不退出与主Agent重复派单。

