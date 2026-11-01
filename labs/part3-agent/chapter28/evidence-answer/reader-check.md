# 知识链路试学：命中、支持与完成

预计60–90分钟，环境准备另计。先掌握Loop、Context和基本检索，再使用自己的仓库副本完成。该材料是知识阶段的一次试学，不代表完整作品或整个阶段已验收；只用合成资料、固定生成与评分，默认不启动模型、数据库或网络服务。

## 先把五种判断分开

阅读[Retrieval](../../../../book/part3-agent/building/chapter27-rag.md#topic-ch28-vector-retrieval)、[RAG](../../../../book/part3-agent/building/chapter27-rag.md#topic-ch28-rag-app)及[Eval](../../../../book/part3-agent/governance/chapter35-evaluation.md#topic-agent-eval)中关于证据、支持与任务完整性的段落。运行前为下面每种情况写下：在哪一层失败，还需要什么证据，是否可以交付完整答案。

| 情况 | 需要独立判断的问题 |
| --- | --- |
| top-k返回三份相似资料，但缺当前部署状态 | 排名第一能否证明当前超时，应该在哪一层寻找缺项？ |
| 候选已包含状态和规则，但实际输入只保留规则 | 应改召回还是Context，哪个记录能区分？ |
| 引用ID和版本真实，摘录删掉了原文中的否定条件 | 出处通过以后，还缺哪项检查？ |
| 所有已有断言都supported，却没有回答用户的一半问题 | 支持率与任务通过应分别是什么判断？ |
| Judge返回unknown，或者本次根本没观测输入 | 未确定、空结果和明确失败能否一起统计？ |

不要求先答对。AI可以辅助查术语，但先保留自己的预测，之后说明哪里发生了修正。

## 核对现有机制，再解释结果

在仓库根目录先列测试名，再运行。不要只复制PASS；记录一条与预测不同的结果及其原因。

```bash
go -C labs test ./part3-agent/chapter28/evidence-answer -list 'TestPartialQuoteCannotBypassSupportJudgment|TestEmptyConflictAndScopeBeforeModel'
go -C labs test ./part3-agent/chapter28/evidence-answer -run '^(TestPartialQuoteCannotBypassSupportJudgment|TestEmptyConflictAndScopeBeforeModel)$' -race -count=1 -v
go -C labs test ./part3-agent/chapter33/eval-harness -list 'TestFullySupportedPartialAnswerStillFailsTask|TestEvidenceGroupsDistinguishCandidateAndInputLoss'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^(TestFullySupportedPartialAnswerStillFailsTask|TestEvidenceGroupsDistinguishCandidateAndInputLoss)$' -race -count=1 -v
```

这些测试只验证给定资料、标签和执行合同。不能由固定Judge得出一般释义支持率，也不能从原测试通过宣称自己已学会必要证据标注。

## 迁移到一个新的两项要求

从eval-harness的completeness_test.go及main.go的Case、Result、Evaluate起步，自行新增`TestExerciseTwoRequirementsWithMissingAnswer`。不要导入evidence-answer的私有Go实现；两个实验保持独立。

新问题同时要求“当前超时”和“只读重试次数”。自己写两份合成来源、两项完成条件与对应支持标签，评分字段留在评估侧。先构造只回答一项、且这一项确实被来源支持的运行结果；再构造两项都有依据的结果。最后把其中一项支持判断改为unknown，或者将它的来源从实际Evidence中去掉。

运行前分别预测任务状态、支持比例、判断覆盖与失败分母，再自行写测试核对。必须能说明为什么少量已支持关系不等于完整任务，以及“未判断支持”与“引用根本没进入输入”如何区分。模型或结果不能自行填写自己的supported标签。

新测试写好后执行：

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestExerciseTwoRequirementsWithMissingAnswer$'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^(TestExerciseTwoRequirementsWithMissingAnswer|TestFullySupportedPartialAnswerStillFailsTask)$' -race -count=1 -v
```

当前仓库没有预填新测试；未列出新名称、只通过原回归或全跳过都不算迁移完成。若使用AI写代码，关闭资料后解释每个标签、来源关系和分母；不能因为代码能跑就把合成标签当作独立语义校准。

## 反馈与复核

保留书稿版本/日期、Go版本、实际命令、一条原预测与修正、新测试差异与结果、求助范围及具体卡点。读者代号即可，不需要私有身份或机器路径。

关闭资料后画出“查询→候选→输入→断言/引用→支持→任务判断”，标明自己的反例发生在哪里。审校者核对环境运行、证据层次解释和新任务迁移三类结果；失败或解释不清也是教材改进依据，不将反馈强行写成通过。

原Loop试学关注控制与取消，本次关注知识与评分；二者采用不同任务。只有真实反馈才能记录相应环节结果，不能用两份材料的存在、作者自检或重复运行同一题补成两个阶段已完成。
