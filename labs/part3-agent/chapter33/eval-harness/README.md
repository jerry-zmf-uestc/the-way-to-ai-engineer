# 实验：版本化用例、支持标签与回归比较

用版本化教学夹具验证评估管道：Case与运行结果分别登记，支持标签不由模型自报；执行失败、缺失结果、unknown标签和未知usage不能变成成功或从分母消失。

默认离线，仅依赖标准库；不包含真实模型基准、人工校准或生产延迟/费用Benchmark。RequiredClaims采用固定文本规则，不能判断任意释义。

## 前置与入口

先读[Eval正文](../../../../book/part3-agent/governance/chapter35-evaluation.md#topic-agent-eval)与RAG。Go准备见[运行约定](../../../../book/introduction.md#running-examples)。

- [fixtures.json](fixtures.json)：三个教学Case、独立支持标签、baseline/candidate运行记录。
- [main.go](main.go)：有界JSON载入、受限Runner、评分、分母保留、支持覆盖、Wilson示意与配对退化。
- [main_test.go](main_test.go)：未知标签、工具失败、缺失与重复记录、数据集不一致、取消和身份绑定。

## 预测与运行

Baseline有两个通过和一个待复核。Candidate对其中一个Case返回工具失败，另一个仍只有unknown支持标签。算通过率时能否删除失败和待复核？

```bash
go -C labs run ./part3-agent/chapter33/eval-harness
go -C labs test ./part3-agent/chapter33/eval-harness -race -count=1
go -C labs test ./part3-agent/chapter33/eval-harness -cover
go -C labs build -o /tmp/eval-harness ./part3-agent/chapter33/eval-harness
```

预期通过数量：baseline=2/3，candidate=1/3；candidate有一个failed和一个review，退化Case为timeout。支持判断没有已知标签时显示unscored，不能解释成已判断支持率0%。

## 观察评分边界

- 工具失败，即使回答包含正确句子，Case仍失败；已有usage保留。
- 引用即使在允许范围内，也须出现在可信Runner的Evidence记录中；召回候选不等于实际输入。
- 缺失结果生成失败行，总Case数仍为3；重复Case/Run ID或未知Case拒绝。
- 支持率与覆盖率共同报告；unknown不能当作supported，review不通过回归门禁。
- 关系与任务分母分开：四条关系只有一条确定supported时，支持率100%、覆盖率25%，不代表任务通过。Links=0、KnownLinks=0、RecallCases=0分别表示覆盖率、支持率或平均Recall未定义；不能把报告初始零值当成实测0%。
- Recall对资料ID去重，案例无相关性标签时不补零召回。
- Runner绑定CaseID和RunID，仅向Task提供TaskInput（ID/Input/AllowedSources/AllowedTools）并复制允许能力；相关性、预期断言、拒答和Split不交给回调；取消后迟到结果记为失败，未启动任务也保留记录。
- Wilson95是二项比例假设下的示意；三个混合划分Case不是模型效果基准，不能用于证明统计显著提升。

回归比较拒绝任一侧重复/空Case ID、不一致集合和非法状态；已知usage累加检查整数溢出。TestRunnerInputDoesNotExposeGoldCriteria用合成canary验证答案不进入执行输入，允许能力副本不污染用例。这不是Go回调沙箱，执行回调不得读取评分侧全局夹具。

报告Rows保留Split，接受development/regression/adversarial/production-like四类；未知划分仍拒绝。现有三条夹具没有新增production-like效果记录，该类别的支持不等于已经完成容量、成本或生产验证。实际评估按划分及任务类别输出。Config是运行配置身份，实际项目还需保存该身份对应的模型、资料、工具、预算与评分环境。

## 参照与Judge的分组诊断

[calibration.go](calibration.go)接收版本化的参照与Judge标签，按supported/unsupported/unknown形成交叉计数。参照unknown单独计数，不拿它确认Judge正确；Judge unknown留在参照已知的分母中，降低判定覆盖。没有相应类别样本的误判率标记Known=false。

```bash
go -C labs run ./part3-agent/chapter33/eval-harness -calibration
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestCalibration' -race -count=1 -v
```

固定六项由作者构造，无人工标注或真实模型评分：三条正常题都判断一致；两条风险题的参照为unsupported，Judge分别给supported和unknown；一条歧义题参照unknown，Judge给supported。

预期总表：reference_known=5、reference_unknown=1、agreement=.60、coverage=.80、false_support=.50。风险组误支持率1/2，判定覆盖1/2；正常组没有unsupported参照，误支持率未定义。不能把正常组的100%一致性或缺少分母的0值解释成评分器已可靠。

Matrix行是参照，列是Judge，顺序都为supported、unsupported、unknown。它保存所有六项；未知不会变成正确支持或退出报告。测试覆盖未知分母、风险分组、重复ID、非法标签与缺少参照版本。

迁移任务：按正文Rubric为独立公开样本先建立参照，再接入实际Judge输出，保留标注者分歧与裁决记录。Calibrate只分析传入标签，不验证标注者身份、资料真实性或标签正确性；合成统计通过不计真人校准。门禁标准在看结果前确定，严重误支持不被普通样本平均分抵消。

## 必要证据组：候选覆盖与输入保留

[evidence_coverage.go](evidence_coverage.go)的ScoreEvidenceCoverage接收数据集/标注版本、Case、独立EvidenceRequirement和CoverageRun。每组Alternatives是复核后的可替代来源，全部组被覆盖才算前提齐备。标注只在评分侧使用，原TaskInput与fixtures.json未改，模型没有新增隐藏来源清单。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestEvidenceGroups'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestEvidenceGroups' -race -count=1 -v
```

固定三任务、两任务有组标注，一任务实际有运行：Total3/Annotated2/Unannotated1/MissingRuns2。完整候选只覆盖一个已观测任务，CandidatesComplete=1对应全标注分母1/2，不是删除缺失任务后的100%；InputObserved另报。未标注任务不进入组完备分母，但留在Total和Unannotated；这不是任务成功率。

反例对照：重复rule片段仍缺state；候选rule+state齐，但Input仅rule时，候选完备/输入不完备；已标注的等价rule来源与state组合可覆盖。CandidatesKnown=false与明确已观测空列表区分，未知阶段不会被填成成功或列出猜测的缺组。

评分检查Case/Run/组身份唯一，非法来源、同组重复项、未出现在已知候选快照中的Input拒绝。两层对应同一次候选→输入交接；多轮或直接提供背景资料时，先定义完整快照并准确设置Known，不把这项子集约束误用于任意历史。报告同时保存版本、逐Case缺组与观测覆盖。

这是合成集合契约，不证明人类标注可靠、来源真实、当前授权或模型实际使用了前提。迁移任务：在两个公开版本任务上建立必要组及替代依据，再导入实际候选/请求来源记录；先检验覆盖与未观测，最后结合引用/任务判断。原评分夹具与原练习保留，新测试不计作100合格案例。

## JSON覆盖评分入口

使用已准备的本地Schema1文件运行，输入只用于独立评分，不调用模型或工具。协议字段为Schema、DatasetVersion、RequirementVersion、Cases、Requirements、Runs；原Case中的Input与Split必须提供，来源清单与组标注有明确版本。

```json
{
  "Schema": 1,
  "DatasetVersion": "tasks-v1",
  "RequirementVersion": "groups-v1",
  "Cases": [{"ID": "conditional", "Split": "development", "Input": "What is the current timeout?", "AllowedSources": ["rule-v2", "state-v2"]}],
  "Requirements": [{"CaseID": "conditional", "Groups": [{"ID": "rule", "Alternatives": ["rule-v2"]}, {"ID": "state", "Alternatives": ["state-v2"]}]}],
  "Runs": [{"CaseID": "conditional", "RunID": "r1", "CandidatesKnown": true, "InputKnown": true, "Candidates": ["rule-v2", "state-v2"], "Input": ["rule-v2"]}]
}
```

将这个合成示意保存在自己临时目录的coverage-input.json；它不是新增独立模型案例。仓库根目录构建后传入文件路径：

```bash
go -C labs build -o /tmp/eval-coverage-cli ./part3-agent/chapter33/eval-harness
/tmp/eval-coverage-cli -coverage /path/to/private/coverage-input.json
```

响应为Schema1加Report：本示意Total1/Annotated1/CandidatesComplete1/InputComplete0，Input缺state组。输入最多256KiB，拒绝重复/大小写别名键、非法UTF-8、未知/尾随字段与递归深度超过8（根节点0）；输出最多512KiB，包含标注版本、观测覆盖和每任务缺组。

退出0只表示生成报告，InputComplete0仍需要排查，不能把命令成功作为质量门禁。输入/评分/参数错误退出2，不输出半份报告；-coverage与-calibration互斥，空coverage值与位置参数拒绝。无参数和-calibration仍分别运行原演示，原fixtures.json未改。

[coverage_json.go](coverage_json.go)定义公开文件协议，后续实验可按同一版本导出记录。格式校验不验证标注或运行记录来源；实际案例仍须独立标注、请求证据和误判复核。

## 跨Lab定义登记：数量与资格分别报告

[case-registry.json](case-registry.json)固定登记18份公开JSON的路径、字节SHA256、版本、案例ID、读取布局、类型与任务家族；不复制原输入、标签或语料，也不改原文件。[case_registry.py](case_registry.py)从脚本位置确定仓库根，只读已登记的public labs/part3-agent来源，与当前工作目录无关。

```bash
python3 labs/part3-agent/chapter33/eval-harness/case_registry.py > /tmp/course-case-registry-report.json
python3 labs/part3-agent/chapter33/eval-harness/case_registry_test.py -v
```

当前报告dataset_count=18、definition_count=101：model-input为84、contract-case为14、scoring-fixture为3。kind与family是登记声明，不是程序证明的语义分类；definition_count数定义，不数模型调用、提示版本或结果行。压缩任务是一份history定义，逻辑ID由登记指定，不将每条历史消息算成案例。

Schema2/course-case-registry-v2同时保留source_split_counts与current_use/qualification。三组策略数据independent-v1、holdout-v3、holdout-v4及Skills的holdout，共24条定义已绑定公开原记录并标为regression；其余77条用途仍为review_pending。use_evidence登记记录路径、SHA及Rows.CaseID/rows.case_id布局，完整ID集合须匹配原定义，多策略重复行不增加案例数；Skills原记录还核对case_sha256。缺记录、改摘要、漏ID或跨来源ID均拒绝。来源声明holdout或没有Split仍照实展示，不将文件名当当前用途。资格仍全部review_pending，qualified_model_case_count为null；退出0不表示101条都合格，也不满足100合格案例目标。

读取前拒绝绝对路径、越界路径与符号链接逃逸；每文件512KiB、最多20份登记及128条案例。哈希、版本、案例ID、重复字段/来源/版本、读取布局漂移均失败，不输出部分报告；错误输出不包含原语料。路径检查与字节固定不提供来源认证，也不是未知代码沙箱。

迁移时选择一个现有任务家族，先核对公开原记录与使用历史，再分别写明独立性、评分依据和当前用途证据。用途转为regression须按Schema2绑定公开记录并核对重放；扩展资格状态需另定义版本与审查合同，不能直接把qualification改为qualified或重命名heldout文件。原数据集与失败记录保留，新登记不增加案例数量。

## 故障与迁移练习

在副本中新增任务或适配，保留原夹具和原始运行。新增代码需要自己的测试；真实生成、独立标注与默认契约分别验收。

| 任务 | 起点与改动 | 反例与完成证据 |
| --- | --- | --- |
| 1. 检查评分器 | 从fixtures.json、score/Evaluate和main_test.go起步，在新夹具中加入未知来源、越权工具、应拒答却回答及无理由放弃四类记录 | 每条有看似正常文本时仍保留对应失败；其他条件满足而缺支持标签时保持review，已经明确失败的项不降为review。新增断言核对状态、失败分母及usage，不只看总体通过率 |
| 2. 校准支持判断 | 从calibration.go的Assessment/Calibrate及现有审阅材料起步，冻结任务、证据、量规和评分单位；为unknown建立独立复核记录 | 先隐藏机器结果和预期标签，再记录支持/反驳依据、争议与参照版本；未获依据仍unknown。人工意见和Judge结果分开，固定标签构造不算真人校准 |
| 3. 接入实际任务 | 在RunCases的Task适配边界接入只读运行，绑定Case/Run身份，捕获实际输入、工具事件、错误与usage；每Case和整批都有预算 | 新用例包含超时、取消和工具错误后正常文本；运行事实由适配器记录，模型不能填写成功状态。评分字段不进入输入，未知消耗不填零；默认使用固定回调，真实模型显式启用 |
| 4. 分组和配对比较 | 从Report.Rows及Regressions起步，按Split/任务类型报告，再为同Case重复运行登记独立Run身份 | 输入顺序不同仍配对到相同Case；缺失、重复与版本不匹配可见。分别报告Case数、运行数、判定覆盖和逐项退化，不能将重复试验计为新任务；改标签后重新判断可比性 |
| 5. 定义放行门禁 | 保留原Regressions基线，在应用层登记关键Case、质量下限及资源约束，并规定review如何处理 | 构造总分上升但关键Case失败/待复核的报告，仍应拒绝；关键ID不存在或重复不能略过。阈值在看结果前冻结，费用来源与完整性另核对，不从Token直接声称账单节省 |
| 6. 登记跨Lab案例 | 从两个已有公开任务文件起步，记录各自版本、Case身份、任务家族、用途历史、环境与评分依据，再关联运行配置和原记录 | 同任务多提示/多模型仍归同一任务；契约夹具、Judge参照和效果任务分别统计。无法确认划分或独立性时标为待核对，不凭文件名heldout或新ID凑合格案例数量 |

先确认已有基线测试名称，再执行：

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list 'TestToolFailureOverridesSuccessfulTextAndPairedGate|TestRunnerBindsIdentityAndRetainsCanceledCases|TestCalibrationPreservesUnknownAndHighRiskFailures'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^(TestToolFailureOverridesSuccessfulTextAndPairedGate|TestRunnerBindsIdentityAndRetainsCanceledCases|TestCalibrationPreservesUnknownAndHighRiskFailures)$' -race -count=1 -v
```

### 迁移：总体提高，关键用例仍然退化

从main.go的Report/Row、Evaluate及Regressions起步。现有门禁报告所有原先passed、现在不再passed的Case；在副本增加自己的应用放行策略，明确哪些非关键变化允许复核、哪些关键条件禁止放行，不改变原评分事实。

先冻结一个新任务集：其中一项必须在证据不足时拒答，其他项是普通问答。构造候选修复多个普通任务、却在这项任务上编造数值的配对结果。总体通过率可能上升，关键失败仍应阻断；这是固定门禁反例，不是新模型效果结果。

自行编写`TestExerciseCriticalCaseGate`，验证关键项failed或review均拒绝；关键项全部passed以后，继续检查原定质量与资源约束；未知、空白或重复关键ID明确拒绝；Case顺序改变不影响结果。保存被拒绝的具体Case、评分版本与理由，不删除失败行，也不在看到结果后降低阈值。

新测试写好后，先用-list确认存在，再运行它及原回归：

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestExerciseCriticalCaseGate$'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^(TestExerciseCriticalCaseGate|TestToolFailureOverridesSuccessfulTextAndPairedGate)$' -race -count=1 -v
```

当前仓库没有预先实现该练习测试，no tests to run或只执行原基线不能算完成。闭卷说明：评分器负责哪些事实，放行策略作出什么选择，为什么“通过率更高”仍可能被拒绝？人工校准、真实消耗与独立任务效果需要各自证据。

## 支持率满分与任务完整性

[completeness_test.go](completeness_test.go)的合成combined-config标签同时允许条件句与当前结论。仅输出条件句时，支持率/覆盖均1.00，RequiredClaims缺少当前结论，任务仍failed；补齐必需结论后才passed。它是固定评分契约反例，不是真实模型或人工语义校准，也不计新的独立Eval任务。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestFullySupportedPartialAnswerStillFailsTask$'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestFullySupportedPartialAnswerStillFailsTask$' -race -count=1 -v
```

迁移：给“当前配置”“历史规则”和“比较两版本”分别写完整要求，再用同一正确条件句检验哪些任务仍未完成；自行补释义/条件/单位的语义依据，不通过批量添加supported标签让所有回答通过。旧Judge的unknown、事后参照与原运行拒绝分别保留，不能回填旧Run改变其结果。

## 有界运行时验收

运行时验收入口见[acceptance.go](acceptance.go)：RunAcceptance固定应用标准集合/版本，生成器只接TaskInput与已验证的失败差距；检查器对当前候选摘要逐项判断。只在全部pass时赋Output，unknown转review；错误、协议失败和候选上限不交付草稿。默认CLI及离线Case/Result协议保持原样。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestAcceptance'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestAcceptance' -race -count=1 -v
```

原四个控制测试及新增绑定测试应实际执行。首候选timeout=30缺retry，检查器返回missing retry，生成器确认收到实际反馈才返回第二份文本，再检查后passed；原候选/失败判断仍保留。其余覆盖连续失败耗尽、unknown、漏项/重复/额外标准、错摘要、无证据/无差距、超限、取消、错误链和输入副本。

固定检查器仅检查字段出现，不验证数值和来源；它的passed不能证明报告正确。接口最多8标准/4候选，文本4096字节，证据/差距各1024；同步callbacks必须协作context。GenerationCalls与ReviewCalls是入口调用数，内部模型/工具消耗、usage和真实任务效果未测，不新增合格Eval案例。

迁移任务：在独立副本加入单位、值、来源和条件标准，为“字段齐全但错误”“资料版本改变”“评分器异常”创建验收测试；错误或unknown不得通过补写分数放行。再替换真实生成器与检查器，记录全部调用和候选，在相同任务/总预算下与无修订基线比较；既有固定测试只作控制回归，不提供语义评分答案。

### 判定必须匹配当前任务与轮次

[acceptance_binding_test.go](acceptance_binding_test.go)先取得一份实际通过的Verdict，再在候选文本相同但任务要求已变时提交旧判定。控制器核对Binding、Attempt和Digest，拒绝旧上下文；另覆盖任务ID、允许来源/工具、标准版本/顺序、候选上限改变与旧轮次。调用前冻结TaskInput和Policy，判断不能只关联文本。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestAcceptance.*(DifferentTask|Binding|EarlierAttempt)'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestAcceptance.*(DifferentTask|Binding|EarlierAttempt)' -race -count=1 -v
```

三个测试应实际执行。完全相同上下文产生相同Binding，测试明确保留这个边界：它不是唯一Run ID、身份凭证或资料真实性证明。资料实际改变但输入/政策版本不变时，当前哈希不会发现；真实接入仍须记录证据快照与运行身份，不从一份旧评分推断当前有效。

## 真实本地生成的反馈交接

[acceptance_local_test.go](acceptance_local_test.go)使用RAG的[公开JSON生成入口](../../chapter28/evidence-answer/local_generator.py)，固定Qwen3-1.7B/revision、rag-joint-v3、输入1024/输出192Token与整个Run180秒；独立Python及资产路径由可信应用设置。先核对完整[generation-requirements.lock](../../chapter28/evidence-answer/generation-requirements.lock)，模型文件仍由原CLI校验，不下载或加载远程代码。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestActualAcceptanceLocalGeneration$'
RAG_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestActualAcceptanceLocalGeneration$' -race -count=1 -v
```

沿RAG约定配置GENERATION_PYTHON与GENERATION_ASSETS绝对路径。可选ACCEPTANCE_RECORD_PATH为新的绝对输出文件；使用排他创建，已有文件拒绝覆盖。默认跳过模型，默认检查重放[原始记录](testdata/acceptance-generation-v1.json)，不修复输出。

固定Harbor两份证据经应用允许集合检查后进入请求。评分器只检查当前严格JSON、两份完整引用及固定联合结论；其他措辞保留unknown。失败差距作为普通数据追加到question，证据不改、预期答案不附入请求。实际messages逐项匹配，并拒绝隐藏字段、变更事实、问题及重复键。

| 实际候选 | 输入/输出Token | 截断 | 原始结果与检查 |
| --- | --- | --- | --- |
| 第一份 | 233 / 59 | 否 | 包含abstained=false，不符合回答协议，反馈指出结构与必要前提 |
| 反馈后第二份 | 355 / 84 | 否 | 协议与两份完整引用通过，当前结论却为6秒；固定规则拒绝 |

总输入588/输出143Token，两次生成、两次检查，整个显式测试76.15秒；包括资产校验与两次独立模型加载，不是生成服务延迟或P95。最终status=failed、Output为空，候选上限耗尽。测试PASS表示输入/控制契约成立，不表示任务成功；原记录SHA256为4335101352ef9d45bdafa58bf941d4b003db745f01cf75578bf48454924039e6。

这是已有both-premises任务的回归对照，不计新独立Case。无反馈基线见下节；没有真实Provider账单、动态存储取证或开放语义评分。迁移时保留所有失败尝试，用相同任务与总预算比较，而非只展示第二轮结构改善。

### 同候选预算的无反馈基线

ACCEPTANCE_GENERATION_MODE只接受feedback（默认）或no-feedback；未登记值在模型执行前拒绝。无反馈模式保留同一验收标准与两候选上限，生成请求不附前轮差距，其他证据、系统提示、模型和Token上限相同。

```bash
RAG_GENERATION_E2E=1 ACCEPTANCE_GENERATION_MODE=no-feedback go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestActualAcceptanceLocalGeneration$' -race -count=1 -v
```

实际基线的两份候选都由模型生成，未复制第一轮输出。记录见[无反馈原文](testdata/acceptance-no-feedback-v1.json)，SHA256为8469b04b3230ee2a4541229bf69e85401148ca2065e6a9d837de52e28d3a6c07。默认重放两组原文，分别核对输入模式、请求/messages、判断和终态；旧反馈记录没有Mode字段时按当时的feedback模式读取，不改原文件。

| 同一已见任务 | 候选/检查次数 | 总输入/输出Token | 第二轮检查 | 最终交付 |
| --- | --- | --- | --- | --- |
| 无反馈 | 2 / 2 | 466 / 118 | 协议仍失败，未进入有效引用/结论验收 | 无，候选预算耗尽 |
| 有反馈 | 2 / 2 | 588 / 143 | 协议与双引用通过，当前结论错误 | 无，候选预算耗尽 |

两组有相同预算上限，实际消耗不同：反馈多用122输入/25输出Token。基线显式测试72.44秒、反馈76.15秒，均含两次模型加载和资产校验；先后执行且系统负载未控制，不作为可靠延迟收益对照。贪心解码下，相同输入两次得到相同原文；这不是独立采样或稳定性评估。

本例能观察反馈后的结构变化，却没有任务完成增益。它不证明反馈普遍无效，也不支持保留复杂度的普遍优势；更广任务、独立评分和重复试验仍须补齐。

## 运行诊断开发任务

[diagnosis-cases.json](diagnosis-cases.json)登记runtime-diagnosis-development-v1的8份原创合成场景，development用途在模型运行前明确；它们不是实际Trace或production-like证据。机制覆盖接纳容量、Owner/Revision、观察关联、父目标、提交窗口、撤销、来源版本冲突与取消收齐。

[diagnosis.go](diagnosis.go)读取64KiB以内定义，校验唯一Case/事件/动作、期望动作属于建议范围、必要引用存在且不重复。BuildDiagnosisInput只投影question/events/allowed_actions，AllowedSources为事件ID，工具为空；expected/source/scenario保持评分侧。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestDiagnosis'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestDiagnosis' -race -count=1 -v
```

两个测试实际发现并执行，核对8机制、契约文件存在、标签变更不影响输入，并拒绝坏引用、重复ID/动作/事件、超限与错划分。它们验证定义关系，不证明预期原因语义已校准或模型已诊断正确。

迁移：先独立审阅事件是否足够支持预期原因，记录有歧义的解释与裁决；再把TaskInput交给受限诊断生成器，保留原始结果/引用/动作和未知判断。在新数据版本中加真实脱敏事件并逐条核对授权，不把当前合成定义直接改成生产样本。建议动作不执行；实际工具接入仍需应用注册、参数及权限检查。当前真实开发对照见下节，仍未计作已完成的合格100案例。

### 诊断回答的分层检查

[diagnosis_score.go](diagnosis_score.go)解析严格JSON cause/action/refs（4096字节、自由原因1024字节、引用最多8个且唯一），不接受模型附上的score。ScoreDiagnosis分别检查允许/预期建议及必要事件引用；建议不执行。原因由可选DiagnosisJudgment在评分侧判断，绑定dataset/rubric、Case和当前输入/答案摘要。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestDiagnosis(Evidence|Judgment|Cause)'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestDiagnosis(Evidence|Judgment|Cause)' -race -count=1 -v
```

三个测试验证：动作/引用齐备但没有判断仍review，supported/unsupported/unknown分别影响结果；旧任务/原文/量规判定拒绝；原因支持不能抵消错误建议或缺失引用。测试中的Judgment是合成控制夹具，不是真人评分，也不将它计为已校准案例。隐藏原因代码原样输出仍不会自动passed。

迁移：独立审阅模型的自由原因解释，记录版本、分歧与裁决依据，再生成绑定当前答案的判断；不要从模型自报supported创建Judgment。变化后的答案重新检查动作、引用和原因，缺少语义依据保持unknown。真实诊断模型已有下述开发记录，人工校准仍未完成，当前接口只建立评分交接。

## 真实运行诊断生成

公开JSON生成入口显式登记runtime-diagnosis-v1，诊断视图仅question/events/allowed_actions；评分字段拒绝。使用既有锁定Qwen3-1.7B与完整generation-requirements.lock、原资产哈希，CPU单线程、不下载/远程代码。原RAG默认提示和Go适配器不接受这个新诊断模式，原生成记录保持。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestActualDiagnosisLocalGeneration$'
DIAGNOSIS_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestActualDiagnosisLocalGeneration$' -race -count=1 -v
```

沿生成约定配置GENERATION_PYTHON/GENERATION_ASSETS绝对路径。固定8题分两批各4，整个流程360秒，每题1024输入/192输出Token；DIAGNOSIS_RECORD_PATH可选新的绝对记录路径，排他创建不覆盖。默认只纯投影/原始重放，不加载模型。

[原始记录](testdata/diagnosis-generation-v1.json)实际生成耗时99.11秒，总输入1528/输出404，8题均未截断；耗时含两次模型加载/资产校验，不作为服务P95。SHA256为4038fb5f7dba3fc50c4dd07f40e6eb98469b69c871c3fa7311df2875549d398a。

| 检查 | 本次开发结果 | 能说明什么 |
| --- | --- | --- |
| 协议 | 8/8可解析 | 不证明语义诊断成立 |
| 建议匹配 | 6/8 | 按当前开发参照检查，建议不执行 |
| 必要事件引用 | 2/8 | 只是覆盖，不代表原因得到支持 |
| 独立原因判断 | 0/8已知 | 没有评分者判断，不能补supported |
| 综合状态 | review2 / failed6 / passed0 | 保留缺项与错误，不作为已校准成功率 |

例如d05原因说不能宣称角色完成，action却是report_role_completed；d06原因说撤销但只引用e1检索命中，缺e2授权变化。原文未修复。TestDiagnosisGeneratedRecordPreservesUnknownCauses默认重放实际输入/结果/评分，核对全部CauseKnown=false及这些错误；测试PASS不是任务通过。

这8题现在已进入开发分析，不再当未见验收。独立语义判断/争议复核、新任务家族和production-like资料仍待完成，运行次数与单题诊断记录不增加案例数。没有提供真实账单或泛化结论。

## 独立原因审阅起点

[diagnosis-review.md](diagnosis-review.md)汇集问题、事件、建议选项与模型原文，不显示Expected/机器Score；[判断模板](testdata/diagnosis-review-template.json)只预填绑定信息和unknown，Reviewer/ReferenceVersion/Rationale留空。它是审阅起点，不是已完成标注。

[prepare_diagnosis_review.py](prepare_diagnosis_review.py)只读固定定义和原始记录，核对版本、Case集合、任务/messages和预算后输出，不加载模型/执行建议或写标注：

```bash
python3 labs/part3-agent/chapter33/eval-harness/prepare_diagnosis_review.py --format md
python3 labs/part3-agent/chapter33/eval-harness/prepare_diagnosis_review.py --format json
python3 labs/part3-agent/chapter33/eval-harness/diagnosis_review_test.py
```

先将模板复制到新的个人文件，保留生成模板。评审者各自阅读事件与原答案，填写supported/unsupported/unknown、使用的事件与理由、参照版本及未解决问题，再比较分歧；不要用Expected.Cause或机器动作命中率直接给原因打分。原因判断与动作/引用检查分别维护，所有输入/答案改变后重新绑定。

纯测试核对八题、没有预期/机器分数投影、原文保持及输入错配拒绝；判断模板没有任何真人结果。未完成独立审阅时，ScoreDiagnosis仍以nil/unknown进行，不把这些材料计为合格案例或读者试学。身份、标注质量和争议裁决由可信评审流程建立，不由Hash或脚本证明。

### 先复核证据必要性，再看模型引用

[diagnosis-reference-review.md](diagnosis-reference-review.md)与[未审阅模板](testdata/diagnosis-reference-review-template.json)只展示定义视图，不读取模型记录、Expected或机器Score。每题可分别记录多套最小充分事件集合、补充事件、理由/歧义和评审者/参照版本；Reviewed=false、空集合保持未知。

```bash
python3 labs/part3-agent/chapter33/eval-harness/prepare_diagnosis_review.py --scope reference --format md
python3 labs/part3-agent/chapter33/eval-harness/prepare_diagnosis_review.py --scope reference --format json
```

先复制到新的个人文件独立填写，再比较分歧并看模型答案。supported原因与必要引用覆盖分别判断；两套证据各自充分时，不强制模型引用所有路径。修改参照必须新版本重新评分，旧record及开发划分不覆盖；现有ScoreDiagnosis仍按原Expected.Refs运行，没有从空模板生成新标签。

TaskSHA256绑定本审阅视图（本脚本排序键JSON），不是Go输入摘要或身份认证。默认四个review纯测试核对答案/标签不投影、未知状态、视图变化和两份起点重建；这些测试不补标注事实。本材料尚未产生真人结果，也不计合格案例或学习试验完成。

## 资源核算开发任务

[accounting-cases.json](accounting-cases.json)新增12份resource-accounting-development-v1定义，覆盖预留/未知费用、失败重试、取消前执行、共享总账、互斥usage、窗口余量、取消占槽、整体批接纳、恢复额度、关键路径与未知Tokenizer。整数费率和时长是假设，未提供真实支付、报价或生产账单。

[accounting.go](accounting.go)使用有界整数oracle核对Expected的known/value/unit，每题1–8唯一事实；输入仅question/facts，源/规则/期望留评分侧。未知value=null，零不代替未观测。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestAccounting'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestAccounting' -race -count=1 -v
```

两个测试实际执行，核对12不同机制、源契约文件、输入隔离与2个未知任务，并拒绝错单位、unknown带0、错/重复事实、超界/负余量及重复Case。oracle计算最大四项有界乘积，使用int64，不把浮点近似当账本。

迁移：先独立审阅题面是否有明确单位与适用条件，结合实际模型记录保存原输出与引用；未知金额不得从最终成功推定免费。新任务仍development、真实模型结果见下节，真人校准仍无，不计已完成合格案例。跨Lab当前101定义仅是登记数，原16/17份记录和运行变体不重复计数。

### 资源核算答案检查

[accounting_score.go](accounting_score.go)解析严格known/value/unit/refs。known只能是JSON布尔；false须value=null，true须非负int64≤4e12，引用最多8且唯一；正文数字/单位与合成oracle分别比较，未引用输入中的ID拒绝，不执行引用或建议。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestAccounting(Contract|Parser)'
go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestAccounting(Contract|Parser)' -race -count=1 -v
```

两个测试对12定义的合成正确答案检查numeric_contract_passed，并覆盖错数值/单位/未知引用、unknown带0、known带null、布尔null、非整数、重复键和模型附score。a04已确认零调用可得到已知0；a02缺usage必须未知null，不混淆。

程序通过不是模型结果，refs合法不证明充分性；题面/来源/解释仍需独立评审。迁移时保存原始生成和本地Token数，先按known/value/unit分别报告，再分析未知处理与条件遗漏，不把整个任务写为passed。

## 真实资源核算生成

公开JSON生成入口登记resource-accounting-v1，仅锁定Qwen3-1.7B与完整generation lock/原资产hash；视图为question/facts，Oracle规则与Expected不附入请求。系统声明通用输出/单位约定，不给每题正确值。原RAG/诊断提示及记录保持。

```bash
go -C labs test ./part3-agent/chapter33/eval-harness -list '^TestActualAccountingLocalGeneration$'
ACCOUNTING_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter33/eval-harness -run '^TestActualAccountingLocalGeneration$' -race -count=1 -v
```

配置GENERATION_PYTHON/GENERATION_ASSETS绝对路径；12题三批各4、360秒总时限、每题1024/192Token、CPU单线程。ACCOUNTING_RECORD_PATH为新的绝对记录文件，排他创建并保留已生成部分记录；默认不加载模型。

[原始记录](testdata/accounting-generation-v1.json)实际耗时118.49秒，含三次模型加载/资产校验；总输入2365/输出480，12题均未截断。SHA256为93415750e86fc3579db69104a181a4bde74c47cb7cf7a3b2ed6f455e3abd7e8c。

| 检查层 | 当前开发结果 | 解释范围 |
| --- | --- | --- |
| 严格协议 | 1/12可解析 | 其余围栏或嵌套错误，未改写 |
| 数值合同 | 0/12通过 | 唯一可解析题a06的value=10/unit=token，与合成费用参照不符 |
| 截断 | 0/12 | 没有因输出上限截断交付 |

a01局部算出50却给unit并附围栏，a03只报最后5而未累计15，a11给6000而假设关键路径为5000；这些是原文诊断例子，协议失败项不补记已完成数值评分。a02/a12的known对象嵌套错误也保留。TestAccountingGeneratedRecordIsNotRepaired默认重放输入/messages、用量、原答与分层评分，拒绝修复成通过。

本次全部failed，但这是已见合成开发任务，不证明更广模型能力。格式约束/修订/更强模型需要独立版本与同预算基线，不能用人工去围栏、改单位或改Expected制造收益。没有真实账单、外部动作、人工资格或production-like证据，12次运行不增加定义数。

## 来源与许可

原创代码和合成教学夹具，MIT。[NIST比例区间](https://www.itl.nist.gov/div898/handbook/prc/section2/prc241.htm)用于核对Wilson方法，未复制第三方数据。

<a id="book-agent-eval-来源登记与案例资格分开验证"></a>

## 来源登记与案例资格分开验证

### 来源登记与案例资格分开验证

多个实验各有自己的任务文件，汇总时先登记文件、版本、案例身份与读取布局，再核对实际来源。任务文件更新但登记未更新，应明确拒绝；不能读到一部分就继续公布一个看似完整的总数。

[公开登记实验](case_registry.py)将原始字节SHA、来源版本及案例ID分别核对。它使用应用登记的读取布局，支持小写cases、大写Cases、Input.ID、scenario和单份history，不从文件名猜任务结构。最后核对的是：

```python
        valid_ids = all(isinstance(i, str) and i and len(i) <= 128 for i in ids)
        valid_splits = all(s is None or isinstance(s, str) and s and len(s) <= 32 for s in splits)
        if not valid_ids or len(set(ids)) != len(ids) or ids != entry['case_ids'] or not valid_splits:
            raise ValueError('case_identity')
```

这些检查只能证明“读的是登记的那份定义”。来源声明的split、登记的任务家族、已知运行和独立评审仍是不同证据。压缩任务中的多条消息不是多个任务；同一题跑十次也不增加定义数。资格尚未复核时保持未知，不从通过字节校验推导已完成独立验收。

来源真实性、标签是否有效及任务是否独立不能由一个哈希解决。原案例被用于修改提示后，即使文件名与字节没变，用途也已变化；实际使用历史需要另行记录，不能通过重新登记来抹掉。


<a id="book-agent-eval-已经看过的验收题怎样转为回归"></a>

## 已经看过的验收题，怎样转为回归

### 已经看过的验收题，怎样转为回归

一组任务可以在首次冻结配置后用于验收。看到结果以后，它还能帮助定位缺陷和验证修复，但已经失去“作者尚未见过结果”的条件。保留原来的holdout文件名与来源split可以记录历史；当前用途需要单独维护，不能靠重命名或换模型重新变成未见题。

用途登记按以下顺序处理：

1. **定位原定义**：核对资料版本、字节摘要和案例ID，不改旧标签或失败结果。
2. **关联已观察记录**：保存公开原记录的路径与摘要，确认记录涵盖哪些原Case；同一题的多策略、多尝试行归到同一ID。
3. **更新当前用途**：已有结果用于日常重放时标为回归，来源split仍原样展示；需要新的未见验收集时另行冻结，不复用已调优数据。
4. **继续审查资格**：回归用途不证明标签可靠、任务独立或有足够覆盖；资格证据不足时仍保持未知。

公开登记的Schema2将use_evidence与current_use绑定，回归项必须有完整案例关联的原记录；缺记录、摘要变化或漏掉Case时拒绝整份报告。记录哈希只能定位已审查材料，不能证明某次推理真的发生，更不能从ID相同推导两个任务语义相同；登记时仍需核对原执行与对应重放测试。

三组策略任务和一组Skills任务已有这样的公开记录，已按当前用途转为回归。其他资料继续保留待复核，不因批量运行或总数达到要求而自动升级。具体来源与数量留在配套Lab，正文关注来源split、使用历史和案例资格如何分别判断。


<a id="book-agent-eval-从失败机制扩充任务而不是批量改参数"></a>

## 从失败机制扩充任务，而不是批量改参数

### 从失败机制扩充任务，而不是批量改参数

一份配置问答换十个服务名，可能仍只测试相同检索路径。扩充评估集时，先确定系统新增了哪种必须判断的问题：容量拒绝与超时不同，旧角色版本与权限不足不同，工具返回与角色提交也不同；每一种失败都应有可观察事件和可检查的下一步。

配套[运行诊断开发任务](diagnosis-cases.json)提供八种机制的合成事件。以“工具返回但角色未提交”为例，输入同时保留executing、已保存Raw、空RoleResults和原进程终止；期望是先核查提交与权限，不能重新创建预算或宣称角色完成。答案需要引用能够支持这个判断的事件，不能仅复述一个错误码。

这些是人工编写的开发定义，不是实际生产Trace。每题分别登记问题、事件、允许的建议动作、预期原因/动作/必要引用及契约出处；它们帮助暴露不同判断，仍需真实模型运行、独立评分与任务家族审查，才能决定哪些可作为效果验收。

[BuildDiagnosisInput](diagnosis.go)只序列化任务视图，评分元数据不进入生成输入：

```go
	data, err := json.Marshal(struct {
		Question       string           `json:"question"`
		Events         []DiagnosisEvent `json:"events"`
		AllowedActions []string         `json:"allowed_actions"`
	}{c.Question, c.Events, c.AllowedActions})
```

allowed_actions是本题的建议选项，不是已经授权的工具，不能由回答中的动作名直接执行。Expected与source用于评分和审查；修改隐藏标签不应改变任务视图。开发阶段可以反复分析这些问题，但不能事后把文件改称holdout来证明独立验收。

前面确定了要评什么、样本从哪里来、哪些题目还能算独立证据；接下来把这些定义绑回一次真实运行：失败到底发生在哪个 Run、哪条记录能证明它、以及为什么它不能从分母里悄悄消失。


<a id="book-agent-eval-动作引用与原因解释分别评分"></a>

## 动作、引用与原因解释分别评分

### 动作、引用与原因解释分别评分

诊断回答可以选择了允许的下一步，也引用了全部必要事件，却把根因解释错。反过来，原因正确也不能支持“忽略权限继续执行”这样的下一步。评分时分开保留动作是否允许、建议是否符合本题标准、必要引用是否齐全，以及原因解释的独立判断。

[ScoreDiagnosis](diagnosis_score.go)按这些条件决定状态。原因没有判断时保留review，不能用Expected.Cause字符串匹配代替语义支持：

```go
	if !score.ActionAllowed || !score.ActionCorrect || !score.EvidenceComplete {
		return score, nil
	}
	if !score.CauseKnown {
		score.Status = "review"
	} else if score.CauseSupported {
		score.Status = "passed"
	}
```

原因判断在评分侧绑定数据集/量规版本、Case ID、任务视图与答案原文摘要。任务改变、答案改变或评分版本改变时，旧判定拒绝；摘要只关联判定对象，不认证评审者或证明判断可靠。

回答的cause是一段自由解释，不要求模型猜隐藏的原因代码。refs表示它使用哪些输入事件，合法引用只证明事件存在、必要引用齐备只证明覆盖条件满足；原因是否由这些事件支持仍需独立取证或人工复核。定义中的预期建议也要经过审查，有歧义的任务保持开发用途，不把全部结构检查通过视为已经校准。


<a id="book-agent-eval-检查真实回答里的指标分歧"></a>

## 检查真实回答里的指标分歧

### 检查真实回答里的指标分歧

真实诊断生成可能在不同维度上出现分歧。以配套开发对照为例，模型解释原角色未提交，却选择report_role_completed；另一个回答解释资料已撤销，引用的却只有检索命中事件。正文流畅或动作名称合法，都不能代替引用与下一步的核对。

当前记录中，8题都有结构化输出，6题建议符合开发标准，2题必要事件引用齐备；没有独立原因判断，所以没有完整passed。两题进入review，六题因建议或引用要求失败。这个报告用于定位下一步应改输入、引用指导还是评分依据，不能把动作命中率当作诊断成功率。

输入捕获核对的是实际发送的question/events/allowed_actions；原文、截断与本地Token数分别保存。CauseKnown保持false，人工复核应针对对应版本的具体答案建立参照，再回到校准分析。开发结果一经分析就继续按开发/回归用途使用，不改名为独立验收集。


<a id="book-agent-eval-数值参照也要保留单位与未知"></a>

## 数值参照也要保留单位与未知

### 数值参照也要保留单位与未知

系统任务不只要求语言解释，还包括剩余预算、在途容量和累计消耗的判断。把预留当实付、失败重试从费用中删掉、重启后重建全额预算，都会让一个数值看起来合理，却违反执行事实。

配套[资源核算开发任务](accounting-cases.json)把这些问题分别定义：总账已包含子任务时不重复相加；canceling尚未退出时继续占名额；未知usage不能由预留推断费用；UTF-8字节数不能代替Tokenizer。费率是虚构整数microcredit，时长是明确假设，不是现实报价或服务测量。

[AccountingOracle](accounting.go)独立按本题规则核对结果、known与unit。未知参照没有数值，不写成0；零值只有在事实确实允许确认零时成立。输入只展示问题与事实，不附oracle的规则编号和期望结果。

这个程序参照能发现数值关系错误，不能证明题面规则无歧义或模型理解了适用条件。案例资格仍需核对来源、任务家族、模型运行与独立判断；新增定义数达到目标数字，不等于合格案例完成。


<a id="book-agent-eval-核算输出先分清未知零和单位"></a>

## 核算输出先分清未知、零和单位

### 核算输出先分清未知、零和单位

一个数值可以和参照相等，输出却仍不可采用：费用未知时写0、microcredit写成USD、剩余容量用已经取消但尚未退出的任务计算，都改变了结论含义。核算答案应同时声明known、value、unit以及使用的输入引用。

[ParseAccountingAnswer与ScoreAccounting](accounting_score.go)要求未知对应null，已知对应非负整数，单位和数值分别核对；模型附上的score字段不接受。只有这些合同条件全部通过，才标记数值核算通过：

```go
	if score.KnownCorrect && score.ValueCorrect && score.UnitCorrect && score.RefsValid {
		score.Status = "numeric_contract_passed"
	}
```

这个状态名刻意限定为numeric_contract_passed。refs合法只说明它们属于本题输入，不证明推导完整；解释、引用充分性和来源可信度另行审查。测试中的正确答案由程序参照构造，只验证评分关系，不能报告成模型成功率。


<a id="book-agent-eval-实际生成先报告协议失败所在层"></a>

## 实际生成先报告协议失败所在层

### 实际生成先报告协议失败所在层

模型生成了正确的局部数字，仍可能给出错误单位或额外结构。配套12题真实核算对照中，11份输出带围栏或嵌套错误，未进入有效数值检查；唯一可直接解析的答案也把费用算成输入Token数量，并给出token单位。当前没有numeric_contract_passed。

这份结果应按层报告：协议是否成立、known/value/unit是否可比较、引用是否属于输入。协议失败时不能把尚未检查的数值维度一律解释成算术错误，也不能删除围栏后把修复结果替换成模型原答。若研究格式约束或修复，应另存策略版本与实际代价，再作同预算对照。

原始输入、输出、截断与本地Token计数保留。费率和任务环境是合成开发规则，这次运行不能作为现实计费可靠性或独立泛化证据；未知费用与Tokenizer定义仍需按原口径判断，错误原文不补成正确答案。

前面仍是在单条答案内部区分协议、数值和单位；接下来再看跨多条证据的覆盖问题：候选找到了，不等于输入真的齐了。


<a id="book-agent-eval-独立复核先看事实与原答案"></a>

## 独立复核先看事实与原答案

### 独立复核先看事实与原答案

校准需要一份能够让评审者独立判断的材料，而不只是填写最终分数。先展示问题、事件与模型原文，暂时隐藏预期标签和机器结果，分别记录支持、反驳或无法判断的依据；独立意见完成后再比较，避免先见机器判断就跟着修改理由。

配套[审阅材料](diagnosis-review.md)保留八题的原始回答，未填任何通过标签；[判断模板](testdata/diagnosis-review-template.json)把数据集、量规、Case、输入与答案摘要对应起来。摘要绑定文本，不证明评审者真实独立，也不把空白模板变成校准证据。

评审时，supported需要指出事件如何支持原因和关键关系，unsupported需要指出明确矛盾或误认；事件不足、表达歧义或尚无依据则保留unknown。动作和引用另行检查，不能因为原因合理就接受危险建议。记录分歧、参照版本与裁决，不把两人意见相同自动当作事实。


<a id="book-agent-eval-必要引用的标准也要独立复核"></a>

## 必要引用的标准也要独立复核

### 必要引用的标准也要独立复核

引用覆盖依赖标注者定义哪些事件是必要的。如果一份回答用另一套充分证据解释了问题，固定要求原列表全部出现，可能把有效解释判成缺证据；反过来，事件数量很多也不代表关键前提存在。必要性和替代关系应先于模型结果复核。

配套[证据必要性审阅](diagnosis-reference-review.md)只展示问题、事件和建议选项，不读取模型答案或预期标签。评审者先记录最小充分事件集合：移除某条事件后，原因或下一步是否仍有依据；不同路径各自充分时，分别记录，不要求把所有替代路径合并成一个必引全集。

当前诊断的必要引用检查只表示符合这份开发参照，尚未由独立标注证明它是唯一充分关系。新的参照改变了必要性判断时，要保存新版本、分歧与依据，再重新评分；旧运行与旧口径保留，不能直接修改它们来制造模型提升。空白必要组表示尚未审阅，不是任务不需要证据。


<a id="book-agent-eval-看清三类结论的交叉关系"></a>

## 看清三类结论的交叉关系

### 看清三类结论的交叉关系

把参照放在行、Judge放在列，保留三种结论：

| 参照 \ Judge | supported | unsupported | unknown |
| --- | --- | --- | --- |
| supported | 判断一致 | 误否定 | Judge未完成判断 |
| unsupported | 误支持 | 判断一致 | Judge未完成判断 |
| unknown | 参照待复核 | 参照待复核 | 双方均未确定，不能记作正确支持 |

**双方都说unknown，也不能记作已确认的支持。** 参照不确定的项不能用来确认Judge判对；参照确定但Judge未知的项仍留在判定覆盖分母中。两种未知分开报告，才能知道应补资料与标注，还是应改评分器。

[CalibrationCounts.Rates](calibration.go)分别使用对应分母：

```go
func (c CalibrationCounts) Rates() CalibrationRates {
	return CalibrationRates{
		Agreement:    fraction(c.Agree, c.ReferenceKnown),
		Coverage:     fraction(c.JudgedKnown, c.ReferenceKnown),
		FalseSupport: fraction(c.FalseSupport, c.UnsupportedReference),
		FalseReject:  fraction(c.FalseReject, c.SupportedReference),
	}
}
```

Agreement是参照已确定的项中，Judge作出相同确定判断的比例；Coverage是同一分母下Judge实际作出确定判断的比例。误支持率的分母是参照unsupported的项，误否定率的分母是参照supported的项。没有相应样本时，fraction返回Known=false，不能补成0%错误率。

这两个误判率也不是“Judge说supported以后有多大概率正确”：后者需要以Judge判supported的项为分母，并受样本构成影响。报告名称、计数和分母必须一起保存，不能只传一个accuracy数字。


<a id="book-agent-eval-候选与判定必须对应同一份产物"></a>

## 候选与判定必须对应同一份产物

### 候选与判定必须对应同一份产物

评分器说“通过”，应用还要确认它检查的是哪份候选。候选已经修改，却沿用上一版的评分；或者评分只返回其中几个标准，应用把缺项默认为通过，都会破坏验收链路。

[RunAcceptance](acceptance.go)为每轮候选绑定尝试号、文本摘要与任务/政策Binding。后者从应用冻结的TaskInput与AcceptancePolicy计算，包含任务要求、允许来源/工具、标准版本/ID和候选上限。检查器必须返回同一Binding、尝试号、文本摘要及全部标准判断；其中任何一项不匹配都拒绝，不进入自动修订。

```go
type AcceptanceCandidate struct {
	Attempt               int
	Text, Digest, Binding string
}
type AcceptanceVerdict struct {
	Attempt         int
	Digest, Binding string
	Checks          []CriterionDecision
}
```

相同文本在不同要求下可能有不同结论：报告原本只需列参数，后来要求当前生产版本，原通过判断就不能直接沿用。Binding区分输入与政策，Digest区分文本，Attempt区分本次检查轮次；三者共同关联判断。标准ID顺序改变也会形成新Binding，当前实现保守地拒绝旧判定。

Binding不是唯一Run ID；完全相同上下文再次调用会得到同一个值。它也无法察觉输入未记录的资料变化，应用仍需绑定实际证据版本并管理Run身份。哈希不认证评分器身份，也不证明评分依据正确。通过项必须提供证据说明，失败项必须提供具体差距；这些文字仍需由可信检查器从实际取证形成。仅仅存在一个Evidence字符串，不能把模型自报结论变成独立验证。

控制器核对判断以后再选择交付、复核或修订：

```go
		var unknown bool
		feedback, unknown, err = checkAcceptanceVerdict(candidate, required, verdict)
		if err != nil {
			return report, err
		}
		report.Attempts[len(report.Attempts)-1].Checks = append([]CriterionDecision(nil), verdict.Checks...)
		if unknown {
			report.Status = "review"
			return report, ErrAcceptanceReview
		}
		if len(feedback) == 0 {
			report.Status, report.Output = "passed", text
			return report, nil
		}
```

unknown要求复核，不自动当成可修复的fail。明确失败才形成下一轮feedback；反馈只能说明差距，不能替换标准或扩大工具能力。每个新候选重新取证与检查，未改变的内容也不能仅凭上一轮整体通过标记跳过当前产物关联。

候选上限与模型轮次不是同一预算。一次候选生成或检查可能各自调用多次模型、工具或测试；本控制器最多4候选，每轮各调用一次生成器与检查器，不自动重试内部错误。真实适配器还要管理这些调用内部的Token、时间、工具与费用，所有失败尝试也计入消耗。

最终Output只在全部标准明确通过且返回边界未观察到取消时赋值；预算耗尽时保留有界候选记录和合法判断，Output为空。调用方若把Attempts中最后一份文本当最终答案，就绕过了这份交付契约。检查后的权限变化与客户端交付窗口仍需由服务入口复核，不把一次context检查当成原子授权。

配套固定检查器只检查报告字段是否出现，以便隔离反馈传递和控制分支；它不验证参数值或出处。因此，示例passed只对应这份窄标准。用于实际报告时，还需加入值、条件、来源与完整性判断，并按本章校准方法验证评分可靠性。


<a id="book-agent-eval-反馈修订怎样接入真实生成"></a>

## 反馈修订怎样接入真实生成

### 反馈修订怎样接入真实生成

有界控制器通过，还不能证明模型会使用反馈。真实接入要把每轮候选、实际模型输入与判定关联起来：先冻结本任务获准证据，生成器只接问题、证据与上轮具体差距；检查器依据自己的任务标准取证，不让模型生成自己的评分。

配套[Harbor检查器](acceptance_harbor.go)复用RAG的两份合成资料：部署给出fallback_enabled=false，政策分别规定启用与停用时的超时。协议、引用集合与当前结论是三个检查项，全部通过才交付；引用匹配不替代结论判断。

这个检查器只能识别固定规则中的少量结论。对其他表达保留unknown，交给独立复核，不因为不等于规范句式就宣布它语义错误：

```go
		if conclusion == "Harbor current timeout is 43 seconds." {
			checks[2] = CriterionDecision{ID: "conclusion", Status: "pass", Evidence: "current conclusion matches this frozen two-premise rule"}
		} else if conclusion != "Harbor current timeout is 6 seconds." && !strings.Contains(conclusion, " when ") {
			checks[2] = CriterionDecision{ID: "conclusion", Status: "unknown", Evidence: "wording outside this frozen rule; needs independent review"}
		}
```

实际生成通过RAG已公开的JSON入口调用锁定模型，反馈作为普通数据进入下一次question，原证据保持不变；没有从其他Lab导入私有实现。捕获的messages分别核对问题、反馈、证据ID/版本/文本，隐藏评分字段不得进入输入。

一次实际对照中，首候选包含协议不允许的字段，未能进入有效的引用与结论验收。反馈后，第二候选通过协议和双来源完整引用，却把停用条件下的当前值判断成启用条件的值；候选上限耗尽，控制器没有交付Output。**格式和引用齐全，仍不能证明组合结论正确。** 原始输入、输出、用量与判断见配套实验，不能用人工改写候选来补成成功。

配套实验另外实际运行了同一模型、资料与两候选上限的无反馈基线；它保留验收，但下一次生成不附差距。基线重复了协议失败，反馈组改善了结构却仍答错，两组都没有交付。相同上限不意味着消耗相同，反馈增加的输入和输出也要计入。

这次对照说明既有任务上的反馈进入了真实输入、结构发生变化且失败关闭生效；当前没有同预算任务完成增益，也不增加独立案例数量。开放语义支持、更多任务和人工评分仍需按前面的校准与效果对照方法完成。


<a id="book-agent-eval-最佳实践"></a>

## 最佳实践

## 最佳实践

### 先检查评分器，再用它调系统

加入“工具失败但回答正常”“引用真实但断言错误”“unknown标签”“结果缺失”“全部拒答”等反例。评分器能区分这些情况，才有资格为系统改动提供证据；规则测试通过也不能替代真实样本校准。

### 将失败变成可复现用例

从Trace确认执行原因，保留最小输入与环境，复核预期，再进入数据集。敏感数据脱敏，不能把线上原始消息和秘密直接复制到公开夹具。

### 持续改进先改一个明确变量

一次只比较清楚的策略变化，保留配置与结果。发现退化先回到失败Case，不连续堆叠提示词、模型和检索改动，导致无法解释收益来自哪里。


<a id="book-agent-eval-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 报告分数提高，关键任务却失败

检查分组、样本新增或删除、失败是否被过滤，以及评分器版本是否改变。重新按同Case配对比较，确认关键失败没有被平均值抵消。

### 模型自评很好，人工认为答错

先核对Rubric和证据，再检查模型评分是否偏好某种表达或受答案内指令影响。用独立人工样本校准；没有一致性证据时保留review，不把自评结果直接用于验收。

### 引用支持率很高，却大量没有标签

同时检查覆盖率与KnownLinks。补齐或复核标签，再判断支持率；未知不等于支持，缺少评分不能使一次改动通过门禁。

