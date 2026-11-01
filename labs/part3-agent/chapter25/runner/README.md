# 实验：运行状态、预算与有限重试

用固定动作序列隔离验证Runtime策略：一次Run独占Report，模型与工具尝试分别计数，暂时错误仅在RetrySafe成立时重试。默认离线，仅依赖标准库。

不实现动态规划、消息历史、持久恢复或真实Provider。模型动态选动作见[Loop实验](../../chapter23/minimal-agent-loop/README.md)；本Lab不导入它的私有代码。

## 前置与代码入口

先读[Runtime正文](../../../../book/part3-agent/runtime/chapter29-execution.md#topic-agent-runtime)，掌握Go错误链、context和基本测试。Go版本以[labs/go.mod](../../../go.mod)为准，准备方式见[运行约定](../../../../book/introduction.md#running-examples)。

- [main.go](main.go)：Job、Policy、Report、预算预留、可取消退避与Run。
- [main_test.go](main_test.go)：重试条件、预算、迟到结果、退避取消、下游deadline与运行隔离。
- [usage_test.go](usage_test.go)：可表示的最大总数、普通超限、整数溢出与取消后的usage保留。

## 预测再运行

模型动作首次返回暂时错误并回报3个Token，第二次成功回报5个Token，工具动作成功返回固定证据。先预测：模型次数、工具次数与累计Token分别是多少？首次失败是否留下输出？

仓库根目录执行：

```bash
go -C labs run ./part3-agent/chapter25/runner
go -C labs test ./part3-agent/chapter25/runner -race -count=1
go -C labs test ./part3-agent/chapter25/runner -cover
go -C labs build -o /tmp/agent-runner ./part3-agent/chapter25/runner
```

首行输出：

```text
state=completed model_calls=2 tool_calls=1 tokens=8 outputs=[read fixed evidence] error=<nil>
```

随后输出9条顺序事件：run_started、首轮attempt_started/attempt_finished、retry_wait、第二轮attempt_started/attempt_finished、工具attempt_started/attempt_finished、run_finished。失败尝试产生事件与消耗，但不产生成功Output。

## 边界观察

| 条件 | 应观察的行为 | 对应测试 |
| --- | --- | --- |
| 总模型配额1，首次暂时失败 | 第二次尝试前拒绝，仍保留首次usage | TestRetriesConsumeBudgetAndUsage |
| RetrySafe为false或错误永久 | 只执行一次，保留错误原因 | TestRetryRequiresBothConditions |
| 动作返回前取消 | 登记usage，不接受迟到Text，进入Canceled | TestCancellationRejectsLateResult |
| 退避一小时后收到取消 | 立即结束等待，不发起下一次尝试 | TestCancellationDuringBackoff |
| Run总截止时间在退避中到达 | 保留首次usage，返回DeadlineExceeded，不重置时限或继续尝试 | TestRunDeadlineIncludesRetryBackoff |
| 下游返回包装的DeadlineExceeded | 外层context未取消也终止，不重试 | TestWrappedDeadlineStopsWithoutRetry |
| 返回后Token超限或Text过大 | 本次动作已执行，后续动作不执行 | TestBudgetsPreventNextAction |
| 第一动作回报1、第二动作回报最大int | UsageOverflow=true；此前Tokens与本次OverflowUsage分开保留，无负数或第二项输出 | TestReportedTokenSumCannotWrap |
| 总数恰好为最大int或普通超限 | 最大值本身合法；可表示超限保存真实总数并返回ErrBudget | TestRepresentableTokenTotalsKeepTheirMeaning |
| 溢出动作返回前触发取消 | Canceled终态，不发布迟到Text，两部分usage仍可追查 | TestCanceledOverflowPreservesBothUsageParts |
| 多个独立Run并行 | Report与序号各自独立 | TestConcurrentRunsOwnTheirReports |

每次Run只有一个run_finished事件且位于末尾。事件不是持久日志，panic和进程崩溃不经过这条正常返回路径。

MaxModelCalls与MaxToolCalls限制Job.Execute调用尝试；MaxAttempts限制每动作尝试；MaxTokens记录回报的Token；MaxOutputBytes限制单次返回Text的字节。SDK内部的HTTP重试不会自动进入这些次数，需在adapter真实发送边界另记或共享预算。Token未回报时实验记0，真实系统必须标为未知；返回后检查不能阻止已经发生的费用或内存占用。Provider与工具需要自己的输入、读取、输出上限。

UsageOverflow表示总数不能用当前int表示，不等于Token配额的普通超限。此时Tokens仅是此前部分，OverflowUsage是本次回报；两部分均保留但不能直接用int求和，也不能把Tokens单独加入总费用报告。真实账本应采用足够范围的表示并核对实际usage；教学实验不转换价格或账单。

总时限由调用方传入的context控制，贯穿所有尝试和退避。默认demo使用Background以展示固定流程；它没有独立的总时间限制。实际调用配置Run截止时间，单次请求的子截止时间不得延长父级时限。

## 故障与迁移练习

1. **预算与重试**：将demo的MaxModelCalls改为1。先预测终态、次数与输出，再核对；恢复后给工具动作制造一次暂时错误，分别比较RetrySafe开关。
   再在独立副本中让Execute内部模拟三次请求，另记实际发送计数；比较Report与下游计数为何不同。自行设计共享发送配额或关闭内部重试的adapter并补测试，既有Job次数测试不能证明SDK内部次数已受控。
2. **取消与资源**：设计一个等待context的动作，验证上游取消；再用受控通道模拟忽略context的动作，解释为何Runtime不能强制中断它，测试结束前释放通道。
3. **动态接入设计**：画出Loop提交动作、Runtime预留配额、返回观察的顺序。实现一个最小适配时保持Run总预算，不要为每个Job重建Report；使用固定响应验证工具失败没有被最终文本覆盖。
4. **观测扩展**：为事件增加Run ID，验证两个运行可以合并查看且不混淆序号。不要把prompt、原始工具结果或秘密放入事件。

练习保留独立实现空间，不要求增加持久存储、统一平台或不可逆工具。

## 来源与许可

本课程原创代码，MIT。标准库行为参考[context](https://pkg.go.dev/context)、[errors](https://pkg.go.dev/errors)与[time](https://pkg.go.dev/time)。

<a id="book-agent-runtime-最佳实践"></a>

## 最佳实践

## 最佳实践

### 策略集中，动作保持可测试

Provider负责协议转换，Tool负责能力与参数边界，Runtime负责重试和运行配额。先直接测试动作，再验证策略包裹后的行为，最后接入动态Loop。每个独立Lab保留最小接口，不导入上一实验的私有实现。

### 先确定所有权，再引入并发

局部Report单调用方拥有，最容易建立正确语义。确实需要并行工具时，再设计结果提交、预算预留和取消传播；不要先共享状态，再靠race报告逐个补锁。

### 终止原因用于后续选择

预算触发可能说明重复调用，也可能说明任务确实需要更多资源。先看已有观察与失败链，再决定调高配额。失败保留错误链和部分执行事实，完成状态仍要经过任务评估；固定Job全部成功只证明工作流执行完。


<a id="book-agent-runtime-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 重试次数有上限，总调用仍超额

核对预算是否在每次Attempt前检查，以及重试是否重新创建了计数器。把总配额设为1、首轮返回暂时错误，确认第二次执行没有发生；不要只检查最终Attempts。

### 用户取消后，系统继续等待

先直接测试退避与下游context，再看Runtime边界，最后检查HTTP请求是否把取消传入任务。若下游忽略context，修复下游；Runtime无法抢占它。

### 运行失败，却已有工具结果

查看动作执行和结果提交顺序。前面的动作可能成功，后面的动作才失败；超限检查也不能撤销已完成工作。接口应保留部分执行事实，业务层决定这些结果是否可展示，不把终态改成成功掩盖失败。

