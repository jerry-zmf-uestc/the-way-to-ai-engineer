# 实验：有界并发、背压与失败传播

真实goroutine与channel调度固定任务：两个独立查询，汇总依赖二者。调度器独占状态，worker只持有依赖结果副本，限制并发及FIFO就绪队列。离线、标准库。

另有有限就绪任务的用户轮询与资源预留对照，不是持续流量公平、分布式队列、持久任务或实际LLM性能基准；Node.Run是应用登记函数，不由模型提供代码。

## 前置与入口

阅读[DAG正文](../../../../content/book/part3-agent/runtime/chapter31-collaboration.md#topic-agent-dag)、Planning、Delegation、Go并发和context。准备见[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：图校验、就绪/阻断、有限派发、单写者提交及取消收齐。
- [main_test.go](main_test.go)：受控并发、结果复制、失败传播、队列拒绝与取消。

## 预测与运行

timeout与retry同时执行，summary何时可以启动？一个失败是否应该抹掉独立任务结果？

```bash
go -C labs run ./part3-agent/chapter31/dag-scheduler
go -C labs test ./part3-agent/chapter31/dag-scheduler -race -count=1
go -C labs test ./part3-agent/chapter31/dag-scheduler -cover
go -C labs build -o /tmp/dag-scheduler ./part3-agent/chapter31/dag-scheduler
```

输出completed=3、peak=2、summary="timeout=30 retry=2"。峰值是调度器启动而未收齐的数量，不代表真实工具吞吐或容量。

## 关键失败

- 环、未知依赖、重复ID或空Handler：执行前ErrGraph。
- 前置failed：后续blocked，不执行；独立任务继续，原错误保留。
- 就绪超过queueLimit：ErrQueue，停止派发、取消剩余并收齐在途，不隐式扩大缓冲。
- 外层取消：不接受迟到文本，收齐已启动协作任务。
- worker修改依赖map：不影响Report中的前置结果。

最多16节点、8并发、16队列容量，结果4KiB。输入图顺序决定当前运行FIFO；它不提供跨用户公平；有限用户轮询对照见下节。Handler不响应context时仍可阻塞返回，不能由调度器强制抢占。

## 就绪任务的资源预留与轮询

```bash
go -C labs run ./part3-agent/chapter31/dag-scheduler -resources
go -C labs test ./part3-agent/chapter31/dag-scheduler -run 'TestResource|TestAcyclicTasks' -race -count=1 -v
```

实际输出fifo=[a1 a2 a3 b1 b2]、round_robin=[a1 b1 a2 b2 a3]、completed=5、peak=1、free_db=1。两种方式使用相同任务、资源与全局并发配置；db是逻辑资源槽，不连接数据库。Peak=1来自资源容量，不能只看parallel=2就宣称两路并行。

[resources.go](resources.go)接受已满足依赖的ReadyJob，单调度者按资源可用性派发，分别限制16个总任务、8个资源种类、8个在途与每租户接纳量。每种资源容量最多8，每任务最多8个互不重复的资源名；非法/重复ID、资源名与需求在派发前拒绝；整批校验完成后才提交queued，接纳失败返回空状态，不留下部分排队记录。来源身份由应用提供，队列分组不负责认证。

预留先检查全部声明资源，再全部扣槽；不持有半份资源等待。worker只能返回结果，调度器按自己保存的预留回收，错误或取消后不提交残余文本。取消停止派发并收齐在途，恢复free计数；Handler仍须协作响应context。

TestAcyclicTasksCanHaveResourceCycle用两个真实goroutine各持一个channel资源，确定两者开始等待后验证尚未完成，再取消、收齐并释放；没有任务依赖环却出现资源环。TestResourceReservationAllOrNone验证某资源不足时其它资源不被扣留。测试不是任意程序死锁检测器。

租户轮询保持每租户队头次序，跳过当前无法预留的队头；FIFO基线按提交次序扫描可预留任务。当前仅比较有限批次的派发次序、相同结果与资源回收，不模拟持续新请求、不实现优先级抢占或租户SLO。原DAG.Run没有改变，两套机制没有自动集成为服务。

### 异构资源下的队头选择

```bash
go -C labs test ./part3-agent/chapter31/dag-scheduler -run '^TestResourcePoliciesTreatBlockedTenantHeadDifferently$' -race -count=1 -v
```

DB/CPU各1槽、parallel2：a-hold先占DB，后面a-wait也需DB，a-small与b-small需CPU。顺序扫描跳过a-wait派发a-small；租户轮询只看a队头，因此派发b-small。测试分别确认Dispatch=[a-hold a-small]和[a-hold b-small]。

worker先报告开始，再等待context；收到两个开始信号后取消并收齐，验证无迟到结果且两槽归还。两秒仅为测试守护期限，不计算调度延迟。Dispatch由调度器登记，与goroutine发出开始信号的先后分开。

这说明基线并非所有资源条件下的严格FIFO，而是提交顺序扫描首个可预留任务；轮询另保留每租户队头。相同DB五任务例子排除了这项差异，异构例子则用于揭示它。两种策略未改，原练习仍用于自行设计保持/绕行和分资源队列，不从有限派发推导持续无饥饿或吞吐提升。

练习：让某租户队头需要两种资源、后续只需一种，比较保持队头与允许绕行；记录派发机会和完成延迟分别变化。添加按资源类型分队列时，明确同一租户能否多次参与轮询，防止配额被拆分绕过。

## 前置完成与业务输入检查

[evidence_gate.go](evidence_gate.go)的CheckReportInputs按本合成报告要求验证timeout/retry整数，不认证来源。两项查询经check再进入summary；返回not found的查询仍completed，check失败后summary阻断，独立分支继续。原图、Node接口与CLI不变。

```bash
go -C labs test ./part3-agent/chapter31/dag-scheduler -list 'TestCompletedDependencyStillNeedsBusinessInput|TestReportInputContractAndCancellation'
go -C labs test ./part3-agent/chapter31/dag-scheduler -run 'TestCompletedDependencyStillNeedsBusinessInput|TestReportInputContractAndCancellation' -race -count=1 -v
```

两个测试实际执行完整调度与输入检查，验证前置完成不保证参数可用、失败保留ErrEvidence、summary不调用，以及边界值/缺项/额外输入/取消。timeout秒范围1–300、retry次数0–10仅属于本任务，不是配置建议。

迁移到自己的资料任务时，先定义必要字段、资料版本和缺项策略，再替换检查节点；自行添加同一数值来自不同版本、单位变化与部分报告的验收测试。现有数字检查只作基线，不等于已完成来源和证据支持检查。

## 迁移练习

在自己的副本完成。以下测试名称均由学习者创建，现有测试只作回归；不要复制原测试换名字就记为完成。

| 任务 | 起点与需要新增的行为 | 新验收与观察 |
| --- | --- | --- |
| 1. 多层失败传播 | 从main.go的Node/Run及TestFailureBlocksDescendantsButIndependentCompletes起步，构造至少三层依赖和一个独立审计分支 | 新建TestExerciseThreeLevelFailure，核对失败的直接及间接后继未调用、独立结果保留，整体错误仍含原原因 |
| 2. 完成与取消竞争 | 从Run的completion提交分支与worker.go起步，用channel控制返回和取消；为需要严格胜负的本地提交设计版本或共同锁 | 新建TestExerciseCancelAtCommit，分别控制两种顺序，核对接受结果的依据、终态唯一和在途收齐；不要以sleep猜谁先到 |
| 3. 用户配额与公平 | 从live.go的RunReadyStream与TestLiveAdmissionGivesNewTenantNextEligibleTurn起步，新增多轮迟到租户、异构资源及一种权重或老化策略 | 新建TestExerciseLateTenantAdmission，记录较晚加入用户获得派发的机会及原用户配额，区分有限批次轮询与持续流量无饥饿，不以一次派发顺序证明公平 |
| 4. 无环图中的资源死锁 | 从TestAcyclicTasksCanHaveResourceCycle起步，扩展到三个任务/资源，使用受控channel确认持有和等待，再设计统一顺序或预留 | 新建TestExerciseThreeResourceCycle，核对等待环真实形成、取消后资源回收、修复后同任务可完成；所有测试自有goroutine必须退出 |
| 5. 串行与并发的任务效果 | 从Run的parallel参数及完整报告检查节点起步，使用同一查询任务和资料、保留失败与超时 | 新建TestExerciseParallelCostAndCompletion，先验证产物与来源等价，再记录延迟、尝试及已知消耗；没有usage时保持未知，goroutine数不是收益 |
| 6. 部分继续与fail-fast | 从Run的失败处理分支及TestFailureBlocksDescendantsButIndependentCompletes起步，在副本新增fail-fast策略 | 新建TestExerciseFailFastDrain，同一失败图对照独立分支、blocked节点、取消收齐与资源释放；业务是否接受部分回答另写标准，不删除失败节点重算全完成 |
| 7. 目录版本成为真实前置 | 从Node/Run及evidence_gate.go起步，新增catalog→timeout/retry→summary；查询只从依赖输入取得版本 | 新建TestExerciseCatalogVersionConsistency，先删除一条业务边，验证Validate可能通过但查询拒绝缺输入；恢复后核对两份结果版本一致、查询不提前开始 |
| 8. 等待与执行分开计时 | 从Report.Events及派发/回收位置起步，新增就绪、派发、实际开始与完成时刻；用channel控制目录和两个查询 | 新建TestExerciseWaitAndExecutionTiming，核对查询在目录完成后开始、汇总等两份结果，再对照parallel=1/2。正文7秒/5秒是假设算例，不作为机器耗时断言 |

先核对新测试实际存在，再执行新任务与原回归：

```bash
go -C labs test ./part3-agent/chapter31/dag-scheduler -list '^TestExercise'
go -C labs test ./part3-agent/chapter31/dag-scheduler -run 'TestExercise|TestFailureBlocksDescendantsButIndependentCompletes|TestCancellationDrainsStartedWorkAndRejectsLateText' -race -count=1 -v
```

仓库没有预填上述八个测试。第一条未列出所做任务的新名称，或第二条只有原测试PASS，都不算迁移完成。保留失败和设计取舍；取消检查、依赖完整性、执行完成与任务质量分别核对，不能用测试数量证明学会。

## 来源与许可

原创代码，MIT。同步约定参考[Go内存模型](https://go.dev/ref/mem)。

## 派发与worker实际入口的取消边界

worker.go的executeWork由Run和RunReady共用。context在入口已停止时不调用Handler，返回后先检查取消，再检查4096字节上限及普通错误；失败返回空文本；取消优先，非取消的超大输出返回ErrGraph，输出合规的普通错误保留原error链。worker仍向done发送一次completion，调度器按原策略收齐、减少active并释放资源。running表示已派发，不保证Handler已进入。

```bash
go -C labs test ./part3-agent/chapter31/dag-scheduler -list 'TestStoppedWorker|TestWorker|TestCancellationDrains|TestResourceCancelDrain'
go -C labs test ./part3-agent/chapter31/dag-scheduler -run 'TestStoppedWorker|TestWorker|TestCancellationDrains|TestResourceCancelDrain' -race -count=1
```

入口反例核对调用次数为0；返回反例核对小与超大迟到文本均保留context.Canceled且文本为空；活动context的超大输出仍是ErrGraph，普通错误保留原因。原图/资源取消测试核对收齐和资源释放。检查后立即取消的竞争、忽略context的回调与外部动作回滚不由此保证。

## 运行中接纳新租户的就绪任务

live.go的RunReadyStream接收可信应用channel中的ReadyJob；既有RunReady与默认CLI不变。关闭输入后执行并收齐已接纳任务。非法任务、重复ID、未知/重复资源返回ErrGraph；每租户待派发或总接纳超限返回ErrQueue，并取消、收齐当前Run。拒绝的任务不记queued，普通执行失败保留原因、其他任务继续。

限制分别为资源1–8种、每种1–8槽、并发1–8、每租户等待1–16、Run总接纳1–128。资源合法性按初始容量检查，暂时没有空闲仍可排队。轮询保存最近成功派发的位置，加入新租户后从其后继继续；仅各租户队头参与，原先队头阻塞取舍仍在。maxTasks不代替总时限，输入持续开放时须提供context截止时间。

```bash
go -C labs test ./part3-agent/chapter31/dag-scheduler -list '^TestLive'
go -C labs test ./part3-agent/chapter31/dag-scheduler -run '^TestLive' -race -count=1 -v
```

TestLiveAdmissionGivesNewTenantNextEligibleTurn用channel确认a1进入后再提交a2/a3/b1，b接纳以后才释放a1；实际派发为a1、b1、a2、a3，最终四结果、DB槽归还。其他测试核对队列超限后取消并等待worker退出、关闭输入后完成、普通错误回收、未知资源/重复ID/总接纳预算及预取消。没有用sleep指定先后，3秒仅作测试失败保护。

调用方拥有producer生命周期，提交后不修改Needs或闭包共享状态；Run返回后producer不能继续无期限发送，不由接收方关闭其channel。资源槽是逻辑预留，Tenant是应用标签，实验不提供身份认证、真实DB连接、持久队列或长期公平保证。

原迁移练习的持续流量任务以此作新增基线：进一步设计多轮迟到租户、异构资源/权重或老化策略，以及等待指标与取消窗口；TestLive通过不能代替TestExerciseLateTenantAdmission中的新策略验证。

## 调用层收齐生产者

live_demo.go使用固定四任务生产者。首个任务进入后继续发送，同租户待派发上限1拒绝第三项；调度器取消并收齐worker，调用层随后取消生产者并等待退出。仅producer关闭自己拥有的input，发送使用context选择；默认和-resources演示保持，-live与-resources不能同时设置。

```bash
go -C labs run ./part3-agent/chapter31/dag-scheduler -live
go -C labs test ./part3-agent/chapter31/dag-scheduler -list '^TestLiveDemoStopsProducerBeforeParentDeadline$'
go -C labs test ./part3-agent/chapter31/dag-scheduler -run '^TestLiveDemoStopsProducerBeforeParentDeadline$' -race -count=1 -v
```

输出cause=queue_full、producer_stopped=true、free_db=1、results=0。main为演示提供5秒保护，测试父期限3秒仅作失败保护；测试核对父context尚未到期，不能靠等到父超时才让producer退出。省略调用层cancel时，受控反例直到父期限才结束；修复后队列原因保留、资源已回收、worker和producer都完成退出。

producer_stopped由演示实际等待退出后输出，不作为通用远端服务状态。固定任务源没有失败重试或持久接纳；实际来源需要自己的错误与取消协议，非协作函数无法抢占。原多轮/异构/权重/老化迁移继续在此基线上扩展，不复制演示当作新任务完成。

<a id="book-agent-dag-让业务检查成为下一步的输入条件"></a>

## 让业务检查成为下一步的输入条件

### 让业务检查成为下一步的输入条件

超时查询返回“not found”且没有调用错误时，查询动作可以标为completed：资料查询确实结束了。若报告必须比较超时与重试参数，下一步仍缺少必要输入。不能把查询状态改成“成功找到参数”，也不能直接把这段文本交给模型写完整报告。

可以在查询与汇总之间增加应用检查节点：两份查询结果先进入check，summary只依赖check接受的产物。若任务允许缺项报告，则采用另一份明确的交付契约，保留缺项；不要为了继续执行而静默放宽当前检查。

[CheckReportInputs](evidence_gate.go)用于固定参数报告：超时按秒计，取1–300；重试按次数计，取0–10。这些是本任务的合成约束，不能推广为通用服务配置范围。检查函数可直接作为Node.Run：

```go
func CheckReportInputs(ctx context.Context, deps map[string]string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(deps) != 2 || len(deps["timeout"]) == 0 || len(deps["timeout"]) > 32 || len(deps["retry"]) == 0 || len(deps["retry"]) > 32 {
		return "", ErrEvidence
	}
	timeout, err := strconv.Atoi(deps["timeout"])
	if err != nil || timeout < 1 || timeout > 300 {
		return "", ErrEvidence
	}
	retry, err := strconv.Atoi(deps["retry"])
	if err != nil || retry < 0 || retry > 10 {
		return "", ErrEvidence
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("timeout_seconds=%d retry_count=%d", timeout, retry), nil
}
```

这条链路分三步理解：

1. **查询完成**：timeout与retry节点各自返回实际结果；非空文本不代表参数存在。
2. **检查必要输入**：check按任务契约验证两项依赖的数量、类型、单位约定与范围；不接受额外字段来代替缺项。
3. **决定是否汇总**：check完成后，summary消费检查后的参数；check失败时summary为blocked，独立分支仍按既有策略执行。

检查失败保留ErrEvidence，外层同时报告ErrFailed；前置查询仍保持自己的completed事实。合法数字只证明格式与当前范围满足要求，不证明来源正确或同一配置版本。委派结果中的Missing、来源、版本和冲突也要在业务检查中处理；当前固定字符串示例没有实现这些更广的证据检查。

前面解决的是“结果够不够格交给下游”。再往下一层，先别谈并发收益，而是先确认这张图本身有没有缺边、坏边或环，避免调度器把一张业务上错误的图正确地跑完。


<a id="book-agent-dag-新租户在运行中加入轮询从哪里继续"></a>

## 新租户在运行中加入，轮询从哪里继续

### 新租户在运行中加入，轮询从哪里继续

A的第一个任务已经开始，A又提交了两个任务，B此时才出现。预先固定批次的派发顺序，无法回答B加入以后会不会获得机会；调度器需要在执行期间接纳任务，并保留轮询位置。

若只保存“下一次数组下标”，派发A时只有一个租户，游标可能已经取模回到0。随后追加B，下一次又从A开始。保存最近派发的位置，再按当前租户集合寻找其后继，才能解释成员变化后的轮次。

[RunReadyStream](live.go)沿用每租户队头和全部资源预留，从最近一次派发之后继续：

```go
	pick := func() (ReadyJob, bool) {
		for offset := 1; offset <= len(tenants); offset++ {
			n := (last + offset) % len(tenants)
			queue := queues[tenants[n]]
			if len(queue) > 0 && pool.take(queue[0]) {
				queues[tenants[n]] = queue[1:]
				last = n
				return queue[0], true
			}
		}
		return ReadyJob{}, false
	}
```

新租户追加到应用队列，last只在成功预留并选中任务时更新；资源不足的队头暂时跳过，该租户内部不绕行。当A1仍占DB槽时，A2与B1都可以合法排队。接纳校验查看配置容量，派发检查才查看当前空闲容量；把“此刻没空闲”当作参数非法，会直接拒绝本来应该等待的任务。

动态接纳的分支保留关闭与取消语义：

```go
		case job, ok := <-input:
			if !ok {
				input = nil
				continue
			}
			if err := ctx.Err(); err != nil {
				stop(err)
				continue
			}
			if err := admit(job); err != nil {
				stop(err)
			}
```

关闭输入表示不再接纳，已接纳工作仍执行并收齐。上游取消、坏任务或准入超限则关闭本Run的派发，取消并收齐在途；被拒绝的任务没有变成queued。普通worker失败保留原因并释放资源，其他合法任务仍按部分继续策略推进。

这条链路按四个边界理解：**接纳→排队→预留并派发→完成后回收**。租户待派发上限、总接纳上限、并发上限和资源槽是不同约束；maxTasks不代替截止时间，输入一直开放时仍需调用方提供总context。生产者由调用方管理，提交后不再修改Needs或不安全的闭包状态，并在运行结束后停止发送；接收方不关闭调用方的channel。

当前入口是有限Run中的实时接纳，不是持久服务或无限流量证明。轮询保证的是符合资源条件时的下一次选择，不能使长任务变短，也不保证不同资源需求下的等待时长相等。Tenant只是应用排队标签，身份授权、真实连接池、加权公平、抢占、跨实例恢复与SLO仍需各自实现和验证。DAG负责产生就绪任务，动态接纳不允许未满足依赖的节点绕过原图检查。


<a id="book-agent-dag-调度器结束以后生产者也要退出"></a>

## 调度器结束以后，生产者也要退出

### 调度器结束以后，生产者也要退出

接纳超限时，调度器收齐worker后可以返回。生产者若仍尝试向同一个无缓冲channel发送下一项任务，就没有接收者；worker已经退出，整个调用层仍不能结束。只给worker传context，不能解决这个发送等待。

调用层应同时拥有生产者的生命周期和退出通知。生产者发送时在input与context之间选择，接收方不关闭它拥有的发送端；调度器结束后，由调用层取消生产者并收齐退出。

[固定调用层演示](live_demo.go)在返回前完成这两步：

```go
	report, err := RunReadyStream(life, input, map[string]int{"db": 1}, 1, 1, 4)
	cancel()
	<-producerDone
	return report, err
```

完整过程是：创建本次生产者context与输入→接纳并派发任务→因关闭、取消或拒绝结束调度→取消仍活跃的生产者→等待退出后向调用方返回。队列超限仍返回ErrQueue，生产者的协作退出不把它改成用户取消；已执行动作、原失败和资源回收结果保留。

这个演示只产生固定只读任务，没有通用任务源框架。实际数据库扫描或远端队列读取也须响应context，忽略取消的生产者不能被Go强制抢占。停止生产、停止派发、取消执行与等待退出是不同动作，需要明确谁拥有每个阶段。


<a id="book-agent-dag-最佳实践"></a>

## 最佳实践

## 最佳实践

### 将依赖建立在可用结果上

只有completed结果进入后续输入，保留来源和完整性。需要部分结果时另定义条件，不以宽松状态判定绕过失败。

### 状态有单一提交者

worker返回记录，调度器更新全局状态。共享Handler同步与输入复制分别检查，不能靠一个全局锁覆盖所有外部资源。

### 背压可以被调用方理解

排队、拒绝、超时和取消有明确原因与任务身份。可持久排队时也定义容量，不让磁盘变成无限缓冲。


<a id="book-agent-dag-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 汇总拿到空或旧结果

检查依赖定义、就绪判断与状态提交，再看是否直接共享可变map。让两个查询一快一慢，汇总必须等待全部有效结果。

### race通过却仍死锁

检查锁与连接池等待顺序、worker是否等待同池任务、channel是否还有消费者。race查数据竞争，不检测任意死锁或容量不足。

### 取消后CPU或连接仍被占用

核对下游context、是否收齐在途结果、资源释放与关闭顺序。调度器停止派发不能替代Handler自己停止。

