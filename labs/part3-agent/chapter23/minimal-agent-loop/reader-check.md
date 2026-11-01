# Loop 环节试学：从运行到迁移

供已有编程经验、正在补Go的读者使用，预计45–75分钟，环境准备另计。它是第一阶段试学中的一个环节，不代替整个Part 3、Tool章或两个阶段的完整验收。默认只运行本地固定模型和只读工具，不需要模型Key。

## 准备一个自己的副本

使用包含当前书稿和实验的仓库副本，按[运行准备](../../../../book/introduction.md#running-examples)核对Go版本。在含book和labs的仓库根目录执行后续命令，不在共同工作区改练习。遇到环境错误先记录卡点；不要把“没有运行起来”直接记成不理解Loop。

阅读[Agent Loop正文](../../../../book/part3-agent/building/chapter24-decision-loop.md#topic-ch23-tool-calling)，暂时不打开main_test.go，也不让AI给出下面问题的答案。Go语法或环境准备可以求助，记录求助范围即可。

## 先预测，再观察

在运行前写下预测，每题说明依据，不要求先答对：

1. 第一轮模型提出add与get_time两个工具，第二轮给最终回答。steps、工具次数和消息条数分别怎样计数？每条工具结果怎样与提议配对？
2. 将MaxSteps设为1，第一轮的两个工具会不会执行？若没有最终答案，已执行的结果还在不在？
3. 工具返回普通错误，与返回包装过的context.Canceled，有什么不同？谁决定是否再次调用模型？
4. 保存全部消息后，进程在工具执行完、观察未提交时退出，能否仅凭History安全恢复？还缺哪些事实？

按[Lab说明](README.md)运行默认程序，再确认下面测试存在并执行：

```bash
go -C labs run ./part3-agent/chapter23/minimal-agent-loop
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -list 'TestRunCompletesWithToolCall|TestMaxStepsCountsModelRoundsAfterToolObservation|TestToolCancellationStopsBeforeNextModelTurn'
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -run '^(TestRunCompletesWithToolCall|TestMaxStepsCountsModelRoundsAfterToolObservation|TestToolCancellationStopsBeforeNextModelTurn)$' -race -count=1 -v
```

对照预测与实际结果，指出哪条预测需要修正，以及正文哪一节帮助你修正。PASS只说明这些原测试通过，不代表你已经完成迁移。

## 改成一个没在原测试中出现的条件

从main_test.go的TestToolCancellationStopsBeforeNextModelTurn开始，自行新增`TestExerciseSecondToolCancelsAfterFirstObservation`：同一轮提议两个只读工具，第一个成功，第二个返回包装过的context.Canceled。让第二轮模型响应明确可辨认，但不要让它替运行时判断取消。

先预测第一次工具结果是否保留、第二项是否作为普通错误观察回填、模型是否会进入第二轮；再写测试核对原错误链、实际工具调用次数、模型次数、消息角色顺序与ToolCallID。不要只断言最后一个字符串，也不要用sleep猜执行顺序。

新测试写好后，在自己的副本运行：

```bash
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -list '^TestExerciseSecondToolCancelsAfterFirstObservation$'
go -C labs test ./part3-agent/chapter23/minimal-agent-loop -run '^(TestExerciseSecondToolCancelsAfterFirstObservation|TestToolCancellationStopsBeforeNextModelTurn|TestRunCompletesWithToolCall)$' -race -count=1 -v
```

当前仓库没有预填这个测试。第一条没有列出新名称、后续只有原测试PASS或出现no tests to run，均不算完成迁移。若借助AI生成测试，请在关闭AI后逐项解释断言，再将第二个工具错误换成普通错误，预测哪些断言应改变；记录哪些部分仍不能独立解释。

## 关闭资料，解释一次完整过程

用自己的话说明“用户输入→模型提议→应用执行→观察回填→继续或终止”，并标明本次取消发生在哪里。回答：为什么取消不能撤销第一个已完成工具？为什么最后有文字不证明任务完成？为什么会话历史不等于恢复状态？

解释不清时可以回读再回答，但保留第一次卡点。试学用于修正教材，不要求把所有问题都填成成功。

## 反馈给审校者

反馈至少包含以下内容，可用文字和代码差异，不必制作报告：

| 项目 | 需要记录的事实 |
| --- | --- |
| 准备与运行 | 使用的书稿版本/日期、Go版本、实际执行的命令、是否遇到环境卡点 |
| 预测与修正 | 一条原预测、实际观察及修正原因；哪段正文帮助了理解 |
| 迁移结果 | 新测试名称、运行结果、你修改的断言或代码；保留失败也可以 |
| 独立解释 | 关闭资料后的说明，以及仍不能解释的问题 |
| 求助与阅读 | AI或他人帮助了什么、花费时间、具体读不懂或图/代码难定位的位置 |

读者代号即可，不需要提供个人身份或机器上的私有路径。审校者分别判断环境可运行、机制理解和新条件迁移，不以运行时间、预测满分或测试全绿代替学习证据。只有真实读者提交了可核对反馈，才记录该环节结果；作者与AI自检不能代替真人试学。
