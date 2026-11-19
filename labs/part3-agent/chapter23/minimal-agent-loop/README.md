# 配套实验：Agent Loop

本实验验证“模型提议→工具执行→观察回填→下一轮决策”。脚本模型与只读工具使结果确定，不需要模型密钥。正文见[Agent Harness · Agent Loop](../../../../content/book/part3-agent/building/chapter24-decision-loop.md#topic-ch23-tool-calling)。

不实现真实Provider、Streaming、Checkpoint、完整Schema校验或生产错误分类器。工具执行边界另见[Tool实验](../tool-calling/README.md)。

## 准备与观察位置

在仓库根目录运行，Go版本以[labs/go.mod](../../../go.mod)为准。先掌握接口、错误链与context；准备步骤见[引言](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：Message、Model、ToolSet、Session与Agent.Run。
- [main_test.go](main_test.go)：正常完成、观察内容、超限、空响应、取消、超时与消息复制。
- ScriptedModel固定输出；其Final是预写文本，不能证明真实模型读懂了工具结果。
- MaxSteps计算模型轮次；一次响应可以包含多个工具调用，工具在下一轮被拒绝前已经执行。
- 普通工具失败作为error观察继续；context取消或超时直接终止。

## 运行与测试

```bash
go -C labs run ./part3-agent/chapter23/minimal-agent-loop
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -race -count=1
```

演示的稳定输出：

```text
answer="6+7=13；当前时间 2026-01-01T08:00:00Z" steps=2 messages=5
```

先预测为什么是2轮、5条消息，再检查角色与ToolCallID。测试结果应全部PASS；用测试名缩小到一个失败分支，不依靠最终文本判断控制流正确。

## 练习

在自己的分支或副本中完成，不改动只读工具边界。

| 练习 | 起点与任务 | 验证方式 |
| --- | --- | --- |
| 轮次耗尽的中断点 | 将MaxSteps设为1，首轮包含两个工具；先预测执行数与历史，再构造断言 | 模型1次、工具2次、4条消息、ErrMaxStepsExceeded；参考TestMaxStepsCountsModelRoundsAfterToolObservation |
| 跨轮观察配对 | 用ScriptedModel连续两轮调用同一工具，使用不同ID，第三轮给Final | 断言每轮Model.LastHistory的角色、ToolCallID与观察内容，不只检查最终文本 |
| 模型能否看到观察 | 在正常测试中故意让Run不追加tool消息，预测哪些断言失败，然后恢复 | 观察内容与历史断言必须失败；最终文本可能仍正确，修复后重跑正常测试 |
| 取消不能当作成功 | 保留TestCanceledFinalResponseIsNotCommitted，暂时移除模型返回后的ctx.Err检查 | 测试必须失败；恢复检查后通过，最终消息不得保存 |
| Session续聊与Checkpoint | 使用新ScriptedModel，在相同Session运行第二个用户问题 | 历史延续但Steps重新计数；不能把这当作继续上次待执行工具 |

练习后执行：

```bash
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -race -count=1
```

## 失败与边界

1. 空Response触发ErrEmptyModelReply，模型错误保留错误链；二者都不自动重试。
2. 未知工具不执行，可以把拒绝原因回填，不能动态增加权限。普通工具错误在教学循环中直接转为字符串；真实应用必须分类和脱敏。
3. 工具返回包装的Canceled或DeadlineExceeded时直接终止；普通业务错误仍可继续。context须传给配合取消的下游，不能强制终止不检查信号的函数。
4. Session.Add与History复制JSON形状的嵌套map/slice，任意Go对象不在深拷贝契约内。Messages由单一所有者管理，不支持并发读写。
5. 保存消息只支持续聊，没有执行游标、幂等键或结果提交协议。生产恢复需要独立Checkpoint。
6. 同时包含Final与ToolCalls时，本实现优先工具分支；调用ID完整性也没有通用校验。接真实Provider前需要adapter与协议检查。

## 来源与许可

本课程原创代码，MIT许可。协议延伸阅读见[OpenAI Function Calling](https://developers.openai.com/api/docs/guides/function-calling)，取消契约见[Go context](https://pkg.go.dev/context)。

## Loop 环节试学

[试学材料](reader-check.md)用于第一阶段中的Loop环节：先预测，再运行原例，自行完成“第二个工具取消、首个观察已保存”的迁移，最后关闭资料解释并反馈。材料没有预填新测试，不将原回归PASS作为读者掌握证明；完整阶段和Tool等相邻主题继续分别验收。

## 实验导读

以下命令均从仓库根目录运行；修改或故障注入请使用自己的分支或副本。

### 两轮调用留下五条消息

在仓库根目录执行；Go版本以[labs/go.mod](../../../go.mod)为准，准备方式见[运行示例说明](../../../../content/book/introduction.md#running-examples)。本实验仅使用标准库和fake，不调用真实模型或网络。

```bash
go -C labs run ./part3-agent/chapter23/minimal-agent-loop
```

输出：

```text
answer="6+7=13；当前时间 2026-01-01T08:00:00Z" steps=2 messages=5
```

五条消息来自下面的执行过程：

1. **接收问题**：记录一条user消息。
2. **第一轮模型调用**：模型提出两个工具调用，记录一条assistant提议。
3. **执行两个工具**：分别回填一条tool观察，共两条。
4. **第二轮模型调用**：模型给出最终文本，记录一条assistant消息，结束运行。

因此，消息数为1+1+2+1=5，`steps=2`计模型轮次，不计工具调用次数或消息条数。

ScriptedModel的最终文本是预先写好的，漏掉观察回填也可能输出相同答案。因此，`TestRunCompletesWithToolCall`还检查角色顺序、调用ID、工具结果和模型次数；`TestToolFailureFedBackAsObservation`检查下一轮模型收到的历史。

运行这两个测试：

```bash
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -run 'TestRunCompletesWithToolCall|TestToolFailureFedBackAsObservation' -v -count=1
```

两个测试应均为PASS。失败时按下面的顺序定位：

1. **查消息**：核对角色顺序、ToolCallID和观察回填。
2. **查工具**：单独运行工具测试，确认返回内容与错误。
3. **查适配与决策**：前两层正确后，再检查Provider转换与模型响应。

### MaxSteps限制模型轮次

把MaxSteps设为1，首轮仍可执行多个工具。上限阻止的是下一次模型调用，无法撤销已经完成的工具。

运行前，先判断这次调用会不会返回答案、执行工具并保存观察：

```bash
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -run '^TestMaxStepsCountsModelRoundsAfterToolObservation$' -v -count=1
```

测试中，模型首轮提出两次read。断言检查模型调用1次、工具执行2次，历史留下user、assistant、tool、tool四条消息，最后返回`ErrMaxStepsExceeded`。超限检查阻止了下一轮模型调用，但本轮工具和观察回填已经完成。

MaxSteps也无法限制总费用：单轮调用可能消耗大量Token、请求多个工具，或收到大块工具数据。Runtime需要分别限制轮次、工具次数、输入输出大小和成本，在执行前检查剩余配额，返回后记录实际消耗。

发生超限时，先检查是否重复调用同一工具、漏掉观察，或始终找不到足够证据。确认任务确实需要更多步骤后，再根据评估结果增加预算。

### 工具失败后的继续与终止

普通工具错误会转为`error: ...`观察，供模型选择其他动作或报告失败。未知工具由执行器拒绝，即使拒绝原因回填给模型，该工具也不会执行。

| 条件 | 本实验的行为 | 调用方需要理解的边界 |
| --- | --- | --- |
| 普通工具错误或未知名字 | 记录error观察，在剩余轮次内继续 | 不代表工具成功，也不赋予新权限 |
| 模型调用失败 | 包装并返回原错误 | 不自动重试，保留原因供调用方分类 |
| 空模型响应 | 返回ErrEmptyModelReply | 协议失败，不是空答案成功 |
| 轮次耗尽 | 返回ErrMaxStepsExceeded | 可能已经执行工具，没有最终回答 |
| context取消或超时 | 直接返回对应错误 | 不再要求模型判断是否继续 |

实验中的错误来自本地固定数据，直接用`err.Error()`回填。真实系统需使用稳定错误码，区分可恢复错误，限制修复次数，并在回填前脱敏，避免泄漏内部地址、身份或秘密。工具结果中的指令也不能改变注册权限。

以下测试覆盖空响应、模型错误、取消和超时：

```bash
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -run 'TestEmptyResponseIsProtocolFailure|TestModelErrorKeepsCause|TestCanceledFinalResponseIsNotCommitted|TestToolCancellationStopsBeforeNextModelTurn|TestRunTimeout|TestRunCancel' -v -count=1
```

测试应全部PASS。模型返回Final前触发取消时，Run返回`context.Canceled`，Session只保留用户输入，不追加迟到答案。工具返回包装过的context错误时，即使外层context尚未取消，Loop也会终止，不再调用模型。

`err == nil`只说明循环得到了非空Final。工具失败后，模型仍可能生成看似正常的回答；任务完成度需由Eval评估，答案的资料依据需由RAG与引用校验检查。

<a id="book-ch23-tool-calling-最佳实践"></a>

## 最佳实践

## 最佳实践

### 保持明确的部件边界

让Model适配协议、Tool执行能力、Loop推进决策、Runtime管理运行。Session保存的历史与Context选择的模型输入分别维护。不要把业务校验、Provider字段转换、消息裁剪和HTTP生命周期全部塞进一个循环函数。

一次Run使用独立的状态和取消范围。消息快照要防止共享map被外部修改；配套实现复制JSON形状的数据，不承诺任意Go对象的深拷贝。无并发同步的Session须由单一调用方持有。

### 从确定性链路走向真实模型

先固定模型提议与工具结果，验证以下不变量，再接Provider：

1. **每个已完成工具结果都能对应到提议**，包括失败与拒绝的结果。
2. **下一轮能看到本轮观察**，且调用方不能通过共享对象篡改已保存历史。
3. **取消、超时与超限阻止后续推进**，未完成运行不会被保存为成功。
4. **未知名称和非法参数无法触发工具执行**，错误文本不会隐含扩大权限。

接真实模型后，用任务集评估完成度、工具选择、延迟与成本。脚本模型的最终答案预先写好，只能帮助检查执行行为，不能证明模型质量。测试和任务评估要分别保留证据。

### 记录足以还原决策的事件

每轮记录Run关联标识、轮次、模型提议、工具调用ID、执行状态、耗时与终止原因。失败发生在工具调用前、调用中还是结果提交后，处理方式可能不同。记录应脱敏，并限制大结果的保存方式。

排查问题需要这些可观察信息，不需要保存模型内部思维链。完整Trace与Replay会在后续章节展开；Loop先保证关键动作能够被关联和解释。

### 分开续聊与恢复

保存Session以后，可以把新用户输入接到原会话，启动新的Run；这次运行的轮次重新计数。要恢复原来的Run，还需保存执行游标、待处理调用、已完成结果和预算。

若进程在工具完成后、观察写入前退出，只重放消息历史可能重复调用工具。Checkpoint必须处理这个提交窗口，带副作用的动作还需要幂等或其他明确的恢复策略。本课程实验保持只读，不把保存历史当成执行恢复。


<a id="book-ch23-tool-calling-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 模型反复调用同一个工具

**先检查是否获得新信息。** 对照相邻两轮的模型输入、工具参数和观察：

1. 工具结果未回填或下一轮Context遗漏它：修复消息追加或输入选择。
2. 错误观察只有模糊的“失败”：返回可用的错误码与缺失条件。
3. 参数和观察均未变化：检查任务本身是否无法继续，按策略重规划或终止。

修复后，用同样输入确认下一轮收到新的观察，并能改变动作或明确结束。最后才根据任务评估考虑增加轮次。

### 工具成功了，模型却说没有结果

先直接调用工具，确认结果；再检查assistant提议和tool观察的角色、调用ID及内容；最后核对Context和Provider adapter有没有裁剪、遗漏或错误转换。若输入链路正确而模型仍误读，再评估模型能力与指令。

如果同一轮有多个结果，检查是否用了工具名或完成顺序配对。修复后，逐个核对调用ID；不要仅用最终答案文本判断消息协议已经正确。

### 用户取消后仍收到答案

检查模型返回后是否重新读取context，以及结果提交是否尊重运行状态。若工具长期不返回，继续检查下游是否接收context、是否支持取消。

确定性复现可以在模型返回Final前触发取消：Loop应返回取消错误，不追加迟到答案。严格的提交竞争还需Runtime状态规则处理，增加一次context检查无法消除所有竞争。

### 达到上限时，工具已经执行

先确认上限计量的是模型轮次还是工具次数。模型轮次上限不限制单轮工具数量；要限制工具调用，在每次执行前检查独立计数。

超限后应报告运行未完成，并保留已执行记录。若恢复需求涉及副作用，先检查提交与幂等信息，不能简单重新运行任务。

### 工具失败，Run却返回成功

检查成功指的是“模型给出了Final”，还是“任务通过验收”。普通工具错误回填后，模型仍可能输出非空答案，导致循环正常结束。

工具执行状态应单独记录；关键动作失败时，按任务规则拒绝交付或返回未完成。修复后同时验证工具状态、终止原因与任务评分，不能只检查`err == nil`。
