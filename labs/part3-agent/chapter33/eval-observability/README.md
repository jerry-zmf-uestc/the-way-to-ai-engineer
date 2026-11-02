# 实验：执行轨迹、重放与回归门禁

独立运行四条合成只读任务，记录每步输入摘要与响应，从归档重放同一流程，再按Case/Run身份评分和比较。默认使用固定模型替身；显式本地生成已接入最终阶段并保存真实输入/用量与失败。对应“Eval与可观测性”作品的首个集成链路，动态模型策略、100合格案例及生产观测仍待验收。

Go部分仅标准库，默认不连接网络、不加载模型或写业务数据；真实生成通过既有公开CLI和显式门禁启用，不import其他Lab私有实现。归档只用于本实验的公开合成资料，没有认证或签名，不能作为可信审计证据。模型替身的调用次数不是真实Provider请求数，Steps统计成功响应，Attempts统计实际登记的调用（包含错误返回）；执行失败保留在分母，调用错误不等于已知Provider成本。

## 前置与入口

阅读[Eval](../../../../content/book/part3-agent/governance/chapter35-evaluation.md#topic-agent-eval)和[Trace与Replay](../../../../content/book/part3-agent/governance/chapter34-observability.md#topic-agent-trace-replay)，准备遵循[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：任务投影、实际只读执行、逐调用归档、严格重放、评分与配置比较。
- [main_test.go](main_test.go)：关键拒答、关联与错配、取消分母、JSON边界及评分版本。

四条Case分别标示development、regression、adversarial、production-like，用于演示报告分组与身份；各组只有一题，没有真实任务分布。production-like仅为教学标签，不意味着压测或已有生产证据。Expected/Critical/Split留在评分侧，只投影Doc/Tenant给执行器。

## 预测、运行与比较

```bash
go -C labs run ./part3-agent/chapter33/eval-observability
go -C labs test ./part3-agent/chapter33/eval-observability -race -count=1
go -C labs build -o /tmp/eval-observability ./part3-agent/chapter33/eval-observability
```

基线报告Total=4、Passed=4、Gate=true，每项3步，UsageKnown=false、CostKnown=false。ElapsedNS来自这次本地运行的实际时间，数值会变；它不是Provider延迟或生产P95。步骤数、质量、用量和费用分开报告，未知消耗不填零。

```bash
/tmp/eval-observability -config regressed
```

候选只修改缺资料后的回答策略：遇到absent时从拒答变成无依据“0 seconds”。通过率仍达75%，但CriticalFailed=1、Gate=false，退出码1。越权资料仍由工具拒绝，候选不扩大能力。

## 保存原始轨迹，再离线重放

下面两个路径由操作者选定，只保存本Lab合成数据；换成自己的私有临时目录，避免覆盖其他文件。

```bash
/tmp/eval-observability -archive > /tmp/eval-baseline.json
/tmp/eval-observability -config regressed -archive > /tmp/eval-candidate.json
/tmp/eval-observability -replay /tmp/eval-candidate.json -compare /tmp/eval-baseline.json
```

归档模式只收集记录，退出0不证明质量通过。最后一条只读取两个归档，不调用模型替身或工具：Regressions包含absent、Gate=false，退出1。默认报告按新运行评分；带compare时还要求原先通过的Case不退化。

重放核对数据集/资料版本、配置、Case和Run、调用顺序及输入摘要，消费所有响应，再核对归档答案。RunID在本份归档内唯一，形式为配置/Case；多个同配置尝试合并前还需绑定独立批次身份，不能只按这个局部ID去重或计数。摘要只是关联与错配检测，不能认证归档来源；修改相互一致的整份归档不等于检测所有伪造。归档无秘密，也不默认用于真实用户资料。

## 工具失败如何留在轨迹与门禁中

```bash
/tmp/eval-observability -config tool-failure -archive > /tmp/eval-failure.json
/tmp/eval-observability -replay /tmp/eval-failure.json -compare /tmp/eval-baseline.json
```

使用上节生成的基线归档。受控配置让retry工具返回ErrTool；它不读取网络、不扩大权限。该Case记录两次调用：model-select成功、tool-read带tool_failed且输出为空；model-final不执行。重放返回原类别ErrTool，Steps=1、Attempts=2、ReplayVerified=true。总分母仍4，Passed=3、ExecutionFailed=1、Regressions=[retry]、Gate=false，退出1。

Schema2记录调用错误码和运行终态；新评分器为exact-contract-v2。它要求执行失败为0、关键失败为0且总通过率至少75%；带compare还要求基线已通过Case不退化。75%只是本固定教学数据的门槛，不是生产SLO。

Schema1成功归档仍可重放；旧execution_failed摘要没有错误调用，保留失败且ReplayVerified=false，不升级成完整故障证据。错误码与调用种类不符、错误携带数据、错误后还有调用、轨迹与整Run错误不一致或请求摘要不符均拒绝。取消/超时若发生在最后响应之后，按已记录终态返回停止，不模拟原时钟；调用方这次取消则直接中止验证，不伪造完成。

## 首次失败、重试成功，怎样保留完整过程

```bash
go -C labs run ./part3-agent/chapter33/eval-observability -config tool-recovered -archive > /tmp/eval-recovered.json
go -C labs run ./part3-agent/chapter33/eval-observability -replay /tmp/eval-recovered.json
go -C labs run ./part3-agent/chapter33/eval-observability -config tool-exhausted -archive > /tmp/eval-exhausted.json
go -C labs run ./part3-agent/chapter33/eval-observability -replay /tmp/eval-exhausted.json
```

后一个重放报告拒绝放行，程序退出1（go run另外打印exit status 1）。归档模式保存记录、退出0仍不表示质量合格。临时路径只用于本Lab合成数据，由操作者选定，避免覆盖其他文件。

Schema3只用于这两个新配置，每Run绑定ToolLimit=2，含首次调用；最多一次即时重试。仅ErrTool可重试，参数不变，模型选择不重跑；取消、超时和协议错误直接终止。这里没有退避、持久预算或网络调用，不能据此推导通用重试策略。

| retry任务配置 | 实际轨迹 | Steps / Attempts | 最终报告 |
| --- | --- | --- | --- |
| tool-recovered | select成功→read失败→read成功→final成功 | 3 / 4 | Passed=4、ExecutionFailed=0、Gate=true |
| tool-exhausted | select成功→read失败→read失败 | 1 / 3 | Passed=3、ExecutionFailed=1、Gate=false |

两次read使用同一请求摘要，原始失败保留；其他三题无需重试，仍各三次调用。重放只消费归档中的错误与响应，按归档绑定上限推进，不调用工具或根据当前默认值增加尝试。改变上限、顺序、摘要、终态或删掉必要响应会拒绝；一致改写整份归档仍不能由无签名记录检测，来源认证边界不变。旧Schema1/2沿用单次工具契约，评分器与四条Case未改变，Usage/Cost仍未知。

## 每次调用的来源与用量

[observed.go](observed.go)增加recordObserved和ObservedDriver，Schema4将调用返回的Output、Origin、ModelRevision及用量与原Seq/Kind/InputHash一起归档。来源分为application、tool、fixed-substitute、model和synthetic-model；函数名包含model不代表调用了真实模型。原CLI与Schema1–3不改，旧归档不能无依据补入来源和计数。

用量由可信adapter从实际回报取得，不由答案填写。model来源须有版本，已知计数标明local-tokenizer或provider-reported；未知使用空来源/零占位及UsageKnown=false，不能解释为免费。synthetic-model只用于合成计数，并单独累计SyntheticCalls/Input/Output，不进入真实ModelCalls或UsageKnown。

```bash
go -C labs test ./part3-agent/chapter33/eval-observability -list '^TestObserved'
go -C labs test ./part3-agent/chapter33/eval-observability -run '^TestObserved' -race -count=1 -v
```

四项测试实际调用Go回调并重开归档：四次应用选择、四次工具读取与四次合成模型回报分开；合成计数80/20不成为真实usage。生成失败/取消时Output清空、已回报计数保留，未回报另计Unknown；错来源、错版本、负值、Known不符或旧Schema携带新字段拒绝。没有重新调用模型、取得真实Provider账单或验证语义效果。

Schema4仍按原只读Task与评分器重放，没有live fallback。每调用输入/输出计数上限各10亿，固定任务最多16次调用，int64累加在这个范围内；本协议不规范化缓存计费或价格。CostKnown继续false，单位/价格与失败费用必须按Cost章另接。

迁移：为本Lab的真实adapter填充这些来源与回报，保留实际输入、原生成、适配产物和费用来源，使用显式E2E及总预算；不能把合成测试改名成model来源便宣称真实效果。recordObserved不读取Expected/Critical/Split，评分侧仍独立。Origin声明与摘要关联也不认证归档，受限诊断及可信存储仍需单独设计。

## 最终阶段的真实本地生成

[local_generation_test.go](local_generation_test.go)使用同一四条任务：model-select是应用选择，tool-read实际运行原固定只读工具，只有model-final通过RAG已公开的JSON入口调用锁定Qwen3-1.7B/rag-joint-v3。不import其他topic私有实现，不把Expected/Critical/Split提交给模型；缺失或拒绝资料对应空证据。

准备完整generation锁与既有固定资产，设置可信GENERATION_PYTHON/GENERATION_ASSETS绝对路径；输出记录使用新的绝对文件路径，不覆盖原数据：

```bash
OBSERVABILITY_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter33/eval-observability -list '^TestActualObservedLocalGeneration$'
OBSERVABILITY_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter33/eval-observability -run '^TestActualObservedLocalGeneration$' -race -count=1 -v
```

OBSERVABILITY_RECORD_PATH可选保存新记录，排他创建，失败时保存已捕获部分且Complete=false。最多四次、总420秒，每次输入1024/输出192Token、CPU单线程、stdout64KiB/stderr4KiB；模型资产/依赖不在仓库中。运行缺条件失败，不用固定替身替代实际生成。

[原始记录](testdata/local-observed-v1.json)SHA256为c94f29138d85c93543c2344c279d42db2f33c4ca9c6a630fd22b6b2fc15caabd。实际耗时104.87秒，包含四次模型加载/资产核对，不作为服务延迟。四次模型输入859、输出114Token，均未触及上限；应用选择4次与工具读取4次另报。UsageKnown=true表示本地tokenizer回报已保存，CostKnown=false，不是Provider账单。

| 任务 | 原始生成与适配 | 独立固定合同评分 |
| --- | --- | --- |
| timeout | 30 seconds及匹配引用，适配为原Answer字段 | 通过 |
| retry | 2 retries及匹配引用，适配为原Answer字段 | 通过 |
| absent | 拒答JSON末尾多出一个`>`，严格解析拒绝 | protocol_failed，失败保留 |
| foreign | 同样的额外`>`，空证据下未得到合法产物 | protocol_failed，失败保留 |

当前Passed2/4、ExecutionFailed2、CriticalFailed2、Gate=false。不能因拒答的可读文字合理就删除`>`补成成功；生成原文、实际messages、InputHash、Token、适配Output与评分分开保存。adapter只接受一个断言/一个合法引用，或显式insufficient_evidence拒答；非拒答也允许显式abstained=false，它是声明过的适配合同，未改原文。摘录归属与固定值核对不证明开放释义或通用Judge已经校准。

默认[原记录重放](local_generation_record_test.go)核对实际输入、原文到适配、逐Call身份与真实用量，再按原精确评分器重放全部成功/失败，不加载模型。Complete=true只表示这批运行取证完成；当前没有达到门禁，也不增加独立案例数。下一步按失败层作配置对照，并保留独立任务与人工校准要求。

## 协议失败后的有界重新生成

Schema5在protocol-repair配置中绑定FinalLimit=2；工具仍一次，最多两次最终生成。只有adapter返回answer_protocol才允许新候选；来源回报错误、普通协议/运行错误、取消、超时与输出触顶不重试。原Schema1–4与单次真实记录不变。

```bash
go -C labs test ./part3-agent/chapter33/eval-observability -list 'TestProtocolRepair|TestActualProtocolRepairRecord'
go -C labs test ./part3-agent/chapter33/eval-observability -run 'TestProtocolRepair|TestActualProtocolRepairRecord' -race -count=1 -v
```

真实对照沿同一生成门禁和资产，显式增加模式；总420秒和每次1024/192不变，全批最多8次：

```bash
OBSERVABILITY_GENERATION_E2E=1 OBSERVABILITY_GENERATION_MODE=protocol-repair go -C labs test ./part3-agent/chapter33/eval-observability -run '^TestActualObservedLocalGeneration$' -race -count=1 -v
```

第二次请求只在原question后追加稳定answer_protocol说明、上一原文的引用数据和结构要求，evidence不变，没有正确值或评分标签。输入messages、Attempt、PreviousRaw与每次InputHash/usage绑定；回填不等于程序删除多余字符。

[原修订记录](testdata/local-observed-repair-v1.json)SHA256为f354a7d87c84b7a7fd2b7d9cb42104a305755b53cd647df67d992b35362382aa。实际116.13秒含加载/资产检查；两个有资料任务首轮通过，两个空证据任务各再生成一次，仍返回同一尾随`>`。Passed2/4、ExecutionFailed2、Gate=false；六次模型调用、输入1405/输出152，全未触顶，应用/工具各4次。重放保留原错误、按归档2次上限推进，不运行模型。

相比单次859/114，增加了两次生成及546输入/38输出，没有任务收益。候选上限从1增至2，不能称为同调用预算；两组是同一开发任务的配置对照，不增加独立案例数。语义评分失败不会自动提供隐藏答案或进入协议修订，任务与Judge仍分别验收。

## 故障与排查

| 现象 | 定位顺序 | 本实验的行为 |
| --- | --- | --- |
| 平均通过率合格却不能放行 | 查关键Case，再查分组和具体行 | 关键拒答失败优先，不能被平均数抵消 |
| 重放出现ErrRecord | 先查版本和身份，再查序号、输入Hash与响应格式 | 拒绝，不回退执行真实调用 |
| 结果行与轨迹不一致 | 核对CaseID/RunID及逐步重放答案 | 返回错误，不输出半份评分报告 |
| 执行取消 | 检查运行Error、已记录步骤和全Case分母 | 保留四项失败，不追加迟到答案 |
| 看见Latency但没有Token费用 | 查指标来源及Known标记 | 保留未知，不推算Provider账单 |

输入归档最多128KiB，每响应最多4096字节；拒绝未知字段、重复/大小写别名、尾随JSON、非法UTF-8、过深结构和不兼容版本。它不是多租户服务、持久Checkpoint或OTel Collector；真实Provider错误映射、失败尝试的完整计费与安全导出须继续扩展。

## 执行入口先检查总context

executeLimited在预算检查后、model-select之前核对context。已取消请求直接返回空Answer和对应错误，不进入初始Driver；记录包装层与直接调用因此使用同一入口语义。

```bash
go -C labs test ./part3-agent/chapter33/eval-observability -list '^TestStoppedExecutionNeverEntersInitialDriver$'
go -C labs test ./part3-agent/chapter33/eval-observability -run '^TestStoppedExecutionNeverEntersInitialDriver$' -race -count=1 -v
```

反例同时核对错误、空答案与回调次数0，不只检查最后输出。检查后的取消竞争仍需要Driver协作，原返回边界检查保留，不提供抢占或撤销外部调用；原实际生成与修订记录继续按已保存响应重放，不重新调用模型。

## 迁移练习

1. 从main.go的cases/evaluate出发增加可恢复工具故障Case，记录失败调用与稳定错误码；重放应产生原错误，不重试工具，失败不能丢分母。现有tool-failure提供单次失败基线，练习须新增不同触发的Case及恢复判断，不把原测试作为新任务验收。
2. 从Case.Expected与execute出发接入版本化结构化任务评分，分别保留unknown和failed；不把评分值放入模型输入。
3. 从Report出发加入真实usage来源和版本化价格，未知费用保持未知；验证失败/重试计费，不能用固定替身步骤数冒充Token。
4. 从compare出发按任务家族比较，拒绝混版本和不同Case集合；至少加入一个平均分改善但关键任务退化的反例。

5. 从decision/executeBounded出发扩展有界重试：现有两次即时尝试提供基线，练习须增加不同故障触发和有界退避，记录每次尝试与等待；重放消费原失败与后续成功，不调用工具或临时增加次数。再把剩余预算纳入恢复状态，验证进程重启不重置；不能只重跑当前测试作为练习完成。

为自己新增的验收测试先用go test的-list确认能发现，再用-run运行。原有测试是回归基线，不代替练习验收；当前无真人迁移反馈。

## 来源与许可

原创Go代码与合成任务，MIT。使用标准库[encoding/json](https://pkg.go.dev/encoding/json)、[context](https://pkg.go.dev/context)与[time](https://pkg.go.dev/time)，没有复制其他topic私有实现或第三方示例。

<a id="book-agent-eval-实际生成原文与适配产物同时保存"></a>

## 实际生成原文与适配产物同时保存

### 实际生成原文与适配产物同时保存

模型返回一段看起来正确的文字，应用还需要检查它能否进入规定协议。本实验让实际只读工具提供参数，应用选择保持固定，最终阶段才调用锁定模型；四次真实生成与八次应用/工具调用分别记录，不能把整个流程都称为模型自主执行。

[单引用适配](local_answer.go)将合法断言与引用投影到原Answer，原始生成和实际messages另存。超时、坏JSON、错来源或输出截断不赋最终产物，已取得的Token回报仍保留到失败Call；评分侧继续独立核对值、来源和拒答要求。

实际对照中，两个有资料任务得到正确固定值与引用；两个空证据任务生成了拒答，却在JSON后多出一个`>`，因此在协议层失败。只看文本意思会漏掉这个问题，只看可解析样本的通过率又会删除失败分母。**生成合理、协议可用与任务通过需要分别取证。** 原记录及逐项结果见[配套实验](#最终阶段的真实本地生成)。

重放验证原响应在同一adapter与评分版本下产生相同结果，不重试模型，也不替它删除尾随字符。改变协议修复、提示或模型配置，应另存新运行并统计额外消耗；不能修改旧原文后再宣布它原先通过。

这是固定合成参数任务的执行→真实生成→归档→重放→评分证据，仍不覆盖动态工具选择、开放语义Judge、失败账单或独立任务分布。当前门禁拒绝，模型质量与整个作品级目标继续按原范围验收。

前面固定了原文、适配产物和评分结果要同时保存；接下来再看协议失败后是否值得追加一次生成，以及这次追加怎样记回原运行的预算和分母。


<a id="book-agent-eval-重新生成需要绑定次数与失败类型"></a>

## 重新生成需要绑定次数与失败类型

### 重新生成需要绑定次数与失败类型

一段拒答因JSON尾随字符无法使用，可以尝试让模型重新生成，但不能把所有失败都变成“再试一次”。先区分当前错误是否属于可修订协议、请求是否仍有效，以及还剩多少生成预算；来源异常、取消和不确定语义不应借此绕过边界。

集成实验将可修订错误单列为answer_protocol，在原证据上最多追加一次生成。归档保存FinalLimit以及每次原响应、回报和错误；重放依据原上限消费这些调用，不临时增加机会，也不重新跑工具。每次重新生成都计入用量，不能只报告最终答案的消耗。

实际模型在两个空证据任务收到稳定协议反馈以后，仍重复了尾随字符；输入结构和追加机会已经生效，任务通过数没有提高。这个结果说明反馈进入输入与反馈能够修复是两种证据，增加预算不保证质量。完整两配置与全部代价见[协议修订对照](#协议失败后的有界重新生成)。

可读文字符合拒答意图，也不能将解析失败直接改成成功。修订策略是否值得，应同时看关键任务、失败层、额外调用和未知判断；保留无收益结果，才能决定继续改协议、换配置还是停止承担更多复杂度。

前面讨论的是单条最终生成如何归档与重放。再往下回到更底层：整个 Runner 怎样保证每个 Case 的真实执行结果、取消和失败都不会在批量评估里消失。

