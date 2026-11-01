# 实验：证据、引用与断言校验

以固定资料和注入的Model/Judge验证RAG交付边界：生成前检查身份与结构化事实冲突，生成后检查JSON、出处、版本、摘录和断言支持。默认离线、标准库。

本Lab的Run接收已给定证据，默认不检索、不调用真实LLM。显式本地生成对照加载锁定模型，跨Lab入口另接收neural-retrieval的真实编码/检索导出，再走同一回答验证路径。传输记录不能代替来源与身份授权。

固定释义Judge只验证接口，整段原文匹配只确认文本身份；通用自然语言矛盾/蕴含判断、资料真实性、任务完整性与人工校准仍分别验证。已有真实链路保留格式失败、未知支持与误拒答，不因接上模型和检索就视为效果达标。

完整Claim.Text、Quote与Evidence.Text三者相等时只确认整份文本身份。部分摘录即使Claim.Text==Quote，也必须进入Judge：它可能删掉“不”或“仅在批准后”等上下文。TestPartialQuoteCannotBypassSupportJudgment覆盖否定/条件摘除，无Judge返回ErrReview，显式Unsupported拒绝；TestSupportedPartialQuoteRequiresExplicitJudgeAndNoPartialPublish验证合法部分摘录需要明确支持及整份草稿的失败出口。原始生成记录保持不变，身份检查不成为通用支持判断。

## 前置与入口

阅读[RAG正文](../../../../book/part3-agent/building/chapter27-rag.md#topic-ch28-rag-app)、Retrieval、Context和Model。运行准备见[全书约定](../../../../book/introduction.md#running-examples)。

- [main.go](main.go)：Evidence、结构化Claim、严格Parse、支持校验与单一成功出口。
- [main_test.go](main_test.go)：伪来源、旧版本、改写摘录、错误释义、unknown、Judge失败、无证据、冲突与取消。
- [joint.go](joint.go)：有界多引用断言、整批归属预检和联合Judge入口。
- [joint_run.go](joint_run.go)：独立多引用JSON格式、ParseJoint与RunJoint回答出口；旧单引用协议保留。

## 多引用与联合支持入口

部署资料给出fallback关闭，策略资料给出开启/关闭分支；单引用策略无法证明当前超时为43秒。[joint_test.go](joint_test.go)用原始固定文本与受控规则Judge验证这项交接：

```bash
go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestJoint' -race -count=1 -v
```

三项测试应通过：两份必要证据共同支持43秒，只有策略时返回ErrReview；同样两份资料下6秒结论返回ErrUnsupported。错误/重复/旧版本/改写/超大引用在任何Judge调用前拒绝，后一个断言不合法也不先判断前一个。nil/unknown待复核，Judge错误保留cause，取消后的supported不释放；回调修改收到的引用或证据副本，不改变原输入及后续判断。

ValidateJoint的registry必须由可信应用先完成身份/目录/版本检查，模型不能提供它；函数不实现新身份授权或跨时刻撤销保证。边界为16个断言、每断言1–8引用、总断言/引用字段32KiB、registry最多8份授权证据。Judge只得到本断言引用的完整Evidence，不把无关记录一起交给它。调用方须在整批返回nil后才接受结果，Judge回调不承担提前发布。

固定规则只认识这组部署/策略文本及43/6分支；不能从接口通过推导通用自然语言判断。原Claim/Parse/Run和默认CLI、既有模型记录/分层结果保持单引用协议，不把历史生成结果自动改用新格式重评。

### 联合草稿的生成、解析与整批出口

RunJoint复用旧Run的证据注册与结构化冲突检查，非法范围在Model前拒绝；接着调用注入的Model，ParseJoint解析新JSON格式，ValidateJoint检查全部引用与联合支持，全部通过后才返回完整JointAnswer。

```bash
go -C labs test ./part3-agent/chapter28/evidence-answer \
  -run 'TestJointRun|TestJointParser|TestJointReferences|TestJointPreflight|TestJointJudge' -race -count=1 -v
```

[joint_run_test.go](joint_run_test.go)覆盖整个路径：正确双引用返回一条联合断言，后续错误分支使整份输出为空；模型回调修改输入副本不改变可信快照。无证据/结构化冲突不调用Model，越权拒绝，模型错误保留cause，未知支持和取消返回零答案，模型拒答不调用事实Judge。

新格式是`claims:[{text,citations:[{evidence_id,version,quote}]}]`；受控拒答仍为abstained=true、reason=insufficient_evidence且无claims。严格解析拒绝非法UTF-8、超32KiB、重复字段/大小写别名、额外或尾随JSON、混合分支、重复来源和过多引用。共享重复键检查也保护旧Parse，不允许abstained与Abstained覆盖；合法旧记录和旧测试继续验证。

这些测试使用固定Model草稿与规则Judge，不调用真实模型、检索或网络。它们证明生成后到库返回结果的验证流程；没有新增Live联合生成器/通用联合Judge、客户端传输或语义校准，也没有改变实际原模型的分层效果。

迁移任务：为相同结论引入另一版本的部署状态，分别验证缺状态、旧状态与规则不一致；先定义可信证据集合与联合判断依据，再设计多引用输出协议。保留unknown和整批失败，不拼文本后沿用一个来源ID，也不修改旧记录制造新效果。

## 真实联合生成：前提齐全与缺失

[joint_generation_test.go](joint_generation_test.go)把GenerateLocalVariant放进RunJoint的Model回调，先由应用验证证据范围，再投影问题与ID/version/text。沿用`RAG_GENERATION_E2E=1`、锁定runtime、资产哈希校验、单CPU线程、输入1024/输出192、整组180秒。默认离线只重放已保存记录，不加载模型；旧单引用入口与默认profile保持不变。

三个输入均来自已有Harbor合成案例：部署与策略齐全、只有策略、只有部署。它们是开发回归对照，不新增独立Eval案例数量；评分值和预期拒答不进入模型。

```bash
# 设置已准备并校验的GENERATION_PYTHON和对应GENERATION_ASSETS后运行。
RAG_GENERATION_E2E=1 RAG_JOINT_PROFILE=qwen3 RAG_JOINT_PROMPT=rag-joint-v1 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualJointGeneratedDrafts$' -race -count=1 -v
RAG_GENERATION_E2E=1 RAG_JOINT_PROFILE=qwen3 RAG_JOINT_PROMPT=rag-joint-v2 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualJointGeneratedDrafts$' -race -count=1 -v
# 此配置须将GENERATION_ASSETS切换到1.7B；身份见qwen-1.7b-generator-lock.json。
RAG_GENERATION_E2E=1 RAG_JOINT_PROFILE=qwen3-1.7b RAG_JOINT_PROMPT=rag-joint-v2 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualJointGeneratedDrafts$' -race -count=1 -v
```

| 配置 | 原始生成与准入结果 | 用量（输入/输出Token） |
| --- | --- | --- |
| 0.6B / joint-v1 | 三项格式失败；占位answer、代码围栏、错误字段或尾随文本 | 522 / 139 |
| 0.6B / joint-v2 | 三项格式失败；复述分支或生成无依据300秒 | 633 / 250 |
| 1.7B / joint-v2 | 齐全时生成43秒但错字段/漏前提；另两项进入Judge后unknown | 633 / 178 |

每项一次实际生成，无截断；没有剥围栏、映射字段、补来源或改答案。测试PASS只表示实验运行并完整记录，三组任务评分均0/3，不表示课程模型质量已通过。固定Judge不认识任意释义，也未完成真人校准。

原始记录为testdata/joint-generation-v1.json、joint-generation-v2.json与joint-generation-1.7b-v2.json。默认TestJointGeneratedRecordsReplayWithoutRepair逐项重放原文，核对版本、预算、结果与错误。TestJointFieldRepairCannotSupplyMissingPremise只在副本改id字段，证明解析成功后仍因缺部署引用返回ErrReview；这个故意变体不进入Live适配器，也不改变原记录。

### v3：输入引用键与实际消息

显式RAG_JOINT_PROMPT=rag-joint-v3沿用v2指导与锁定模型，模型资料视图将输入id投影为evidence_id，不改原文/资料数量、不修复输出。旧v1/v2不变。使用原TestActualJointGeneratedDrafts命令选择v3，预算仍输入1024/输出192、Run180秒；只在原RAG_GENERATION_E2E=1门禁运行。

真实1.7B三项记录在[testdata/joint-generation-1.7b-v3.json](testdata/joint-generation-1.7b-v3.json)，均未通过：两前提齐全仍只引用policy并复述条件结论（ErrReview），policy-only的citations为字符串数组（ErrDraft），deployment-only生成无依据60秒（ErrReview）。input/output总638/164，无截断，没有字段或引用补写。

每条GeneratedDraft的input_messages保存实际system/user消息；Go InputSHA256仍定位提交Python的载荷，不能把它当作模板后Token字节摘要。validateCitationInput逐项核对问题及evidence_id/version/text与原授权请求，拒绝额外评分字段、版本或问题漂移；Python投影测试检查标签/租户不进入消息。模板与分词由锁定配置处理，messages本身不证明模型正确采用。

默认TestJointGeneratedRecordsReplayWithoutRepair已包含v3，原错误、草稿、固定Judge与答案保持；TestCitationInputBindsOriginalQuestionAndEvidence检查输入记录/投影边界，不加载模型。新记录不增加独立Case，固定Judge与人工语义/任务校准仍分别验收。

## 实际提交输入与覆盖评分

旧记录没有实际发送载荷摘要，不根据今天的夹具补写成已捕获输入。新[joint-captured-1.7b-v2.json](testdata/joint-captured-1.7b-v2.json)来自独立显式运行：原三任务、原模型/提示/预算，Go适配器对实际cmd.Stdin记录InputSHA256，公开合成测试捕获投影并核对摘要。原输出、错误、Token和0/3任务结果均保留。

```bash
RAG_GENERATION_E2E=1 RAG_JOINT_PROFILE=qwen3-1.7b RAG_JOINT_PROMPT=rag-joint-v2 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualJointCapturedInputs$' -race -count=1 -v
go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestCapturedJointRecordsBindSubmittedInputWithoutRepair$' -race -count=1 -v
```

第一条沿用已准备的GENERATION_PYTHON/GENERATION_ASSETS，第二条默认离线。捕获只用于本组公开合成数据，适配器仅增加摘要，不默认记录任意私有prompt。摘要对应Go发送JSON，不证明模型内部处理或来源身份。

[投影脚本](project_captured_coverage.py)将固定新记录转换为[公开覆盖评分协议](../../chapter33/eval-harness/README.md#json覆盖评分入口)，状态/规则组在评分侧追加。它校验锁定源文件与摘要，仅对这组固定ASCII载荷重建字节，不提供通用JSON规范化。

```bash
python3 labs/part3-agent/chapter28/evidence-answer/capture_projection_test.py
python3 labs/part3-agent/chapter28/evidence-answer/project_captured_coverage.py > /tmp/joint-input-coverage.json
go -C labs build -o /tmp/eval-coverage-cli ./part3-agent/chapter33/eval-harness
/tmp/eval-coverage-cli -coverage /tmp/joint-input-coverage.json
```

Annotated3/InputObserved3/InputComplete1；policy-only缺current-state，deployment-only缺applicable-rule。本次直接提供证据，CandidatesKnown=false、CandidatesObserved0，不能写成Recall成绩。固定ID@version只用于此组来源，不是任意ID的通用拼接协议。

输入齐全的both-premises仍因原id字段与缺部署引用失败，另外两项支持unknown。0/3不变，未补字段或引用；同三任务的新运行不增加独立案例数，也不完成通用标注/Judge校准。

## 预测与运行

资料正文是“Request timeout is 30 seconds.”，模型草稿采用不同措辞。出处与摘录正确后，是否可以直接公开？

```bash
go -C labs run ./part3-agent/chapter28/evidence-answer
go -C labs test ./part3-agent/chapter28/evidence-answer -race -count=1
go -C labs test ./part3-agent/chapter28/evidence-answer -cover
go -C labs build -o /tmp/evidence-answer ./part3-agent/chapter28/evidence-answer
```

演示返回一个通过固定教学Judge的释义断言，source为timeout:0@v1。它没有真实模型调用，不能用这次成功证明自然语言支持判断有效。

## 失败与拒答

| 条件 | 结果 |
| --- | --- |
| 无资料 | Abstained=true，Reason=no_evidence，不调用Model |
| 同结构化FactKey不同值 | conflicting_evidence，不调用Model；不检测任意文本矛盾 |
| 外租户、重复ID或非法证据 | ErrEvidence，不传给Model |
| 坏JSON、重复键、额外字段、空回答/过多断言、混合分支或未知拒答原因、尾部负载 | ErrDraft |
| 模型提出合法证据不足拒答 | Abstained=true，Reason=model_insufficient_evidence；正确性另由Eval检查 |
| 不存在ID、版本不匹配、伪摘录 | ErrEvidence，不能公开部分答案 |
| 部分摘录或释义没有Judge、判断unknown | ErrReview |
| 明确不支持 | ErrUnsupported |
| Judge调用失败 | 包装原错误，不默认为supported |
| 模型返回前取消 | context.Canceled，不接受迟到答案 |

当前上限：8个证据、证据内容合计16 KiB、问题4 KiB、草稿32 KiB、16个断言。字符串返回后检查不能替代adapter的读取和生成上限，费用也需要由调用记录统计。

## 受控本地自由生成

只用公开合成资料与问题。独立实验不导入其他Lab私有实现；[generator-lock.json](generator-lock.json)固定SmolLM2-135M-Instruct revision及六个资产哈希，[generation-requirements.lock](generation-requirements.lock)固定Python3.12环境（PyTorch2.9.0、Transformers4.57.1、safetensors0.6.2）。准备隔离环境与[官方固定提交资产](https://huggingface.co/HuggingFaceTB/SmolLM2-135M-Instruct/tree/12fd25f77366fa6b3b4b768ec3050bf629380bac)，这是显式联网准备；运行只读本地safetensors，禁用远程代码与自动下载。模型/虚拟环境不入仓库。

GENERATION_PYTHON、GENERATION_ASSETS为可信应用配置的绝对路径。准备后从仓库根目录执行：

```bash
RAG_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualGeneratedDrafts$' -race -count=1 -v
```

默认明确跳过真实生成；显式启用且环境或资产错误时失败。每批最多6个请求，问题4KiB、最多8个证据，序列化输入64KiB；CPU单线程、1024输入Token、192新增Token、180秒子进程期限，stdout128KiB、stderr4KiB。这个批处理只用于已确认公开合法资料的离线评估，在线服务应在Run检查证据范围后调用模型。LocalDraftModel拒绝取消与触及输出上限的记录，剩余草稿仍须Parse/Validate；没有通过的部分答案不得公开。

实际开发诊断：v1六条生成复述输入，结构均不符合Draft；v2采用无关格式示例后，2条出处错误、2条草稿错误、2条输出达到上限，公开断言数均为0。原始生成、提示版本、Token与失败阶段完整记录，测试不要求最低模型成功率；通过只证明受控推理和拒绝链路运行，不证明RAG效果已合格。两版共享开发问题，不是独立泛化评测；v1记录仅用于解释诊断，当前入口固定v3；v1/v2结果保留为开发诊断。

v3修正输入投影：只向生成器发送id/version/text，Tenant、Source、FactKey/FactValue留在应用；TestLocalBoundary用合成canary检查内部字段不进入请求。格式示例与生成预算沿用，修改输入协议不等于约束模型输出。

独立验收入口：

```bash
RAG_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualHeldoutDrafts$' -race -count=1 -v
```

[testdata/heldout-v1.json](testdata/heldout-v1.json)含四个应答、两个应拒答任务。模型看不到expected_quote/should_abstain，空资料在Run前置分支直接拒答，不进入生成批次。实际answerable_accept=0/4、exact_expected_extract=0/4、correct_abstain=1/2；资料无关任务的协议错误不计正确拒答。开发集1条通过摘录校验，但生成了重复断言，不当作完整任务成功。冻结v3后这组验收未用于再调提示；后续使用其诊断时转为回归集，并另建验收版本。

Rubric是作者定义的严格摘录覆盖与拒答标签，不是通用语义Judge，更不是已经人工校准的评分器。失败、缺失和分母均保留，不能因任务失败就不报告。

## 同任务的锁定模型配置对照

保留smollm默认配置，另增qwen3白名单配置。资产身份见[qwen-generator-lock.json](qwen-generator-lock.json)，官方Qwen/Qwen3-0.6B revision c1899de289a04d12100db370d81485cdf75e47ca；七个文件大小/SHA256固定，safetensors哈希对照官方LFS OID。权重不分发；沿用已有固定Python环境，不升级依赖。Qwen使用本地CPU、关闭thinking，1024输入/192新增Token、180秒期限与输出体积限制不变；这是实验固定贪心配置，不是通用推荐参数。

GENERATION_ASSETS配置到对应模型资产目录，GENERATION_PYTHON保持隔离runtime。以下单引用对照只使用smollm或qwen3；联合协议另登记qwen3-1.7b，运行不接受任意仓库/脚本。先准备官方资产，再从根目录执行同一组问题：

```bash
RAG_GENERATION_E2E=1 RAG_GENERATION_PROFILE=smollm RAG_GENERATION_DATASET=heldout-v2 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualHeldoutDrafts$' -race -count=1 -v
RAG_GENERATION_E2E=1 RAG_GENERATION_PROFILE=qwen3 RAG_GENERATION_DATASET=heldout-v2 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualHeldoutDrafts$' -race -count=1 -v
```

运行两条命令前分别设置对应GENERATION_ASSETS，不能把同一个目录当成两模型资产。默认smollm仍用heldout-v1，qwen3默认heldout-v2；可显式选择这两个已登记数据集，没有任意文件路径加载。

无Judge时，四个应答任务的实际交付为smollm1/4、qwen3 2/4，预期摘录覆盖同值；应拒答均1/2，空资料不调用模型。Qwen的短Quote/完整Text进入待复核，Markdown围栏被Parse拒绝，无资料支持的问题也没形成合法拒答草稿。小样本与模型模板不同，不推导普遍优劣、延迟或成本收益。

同一生成记录另外送入ExactSourceJudge，只判断Claim与整段原文完全相等；前置ID/版本/Quote校验不变，改数值或一般释义保持unknown，围栏不自动删除。支持规则结果与无Judge结果分别记录，不与生成能力混算：同一生成记录的实际交付为smollm1/4、qwen3 3/4，增量来自确定性全文匹配，不是自然语言蕴含推理。原v3协议的有资料但不足路径缺口由下方v4拒答变体补齐；支持Judge仍不是人工校准的语义评分器。

现有nil Judge让释义停在ErrReview，逐字引用也不证明任务相关或资料真实。没有真人校准，不填写真实支持率。练习：保留这组开发失败，另建独立验收问题；更换经来源/许可核对的生成配置后，比较结构、归属、支持与任务完成的分层结果。不要自动删除示例污染或额外字段后计为原模型成功；若引入修复，单独记录修复次数、usage与失败。

## 显式拒答协议与新验收样本

回答草稿保持原有claims结构；拒答为abstained=true、reason=insufficient_evidence且没有claims。Parse拒绝混合分支、未知原因、重复键、额外字段和空回答。Run在模型调用前仍检查Scope/版本/冲突，拒答不能绕过它们；调用返回后仍检查取消。模型拒答返回model_insufficient_evidence，与应用no_evidence/conflicting_evidence分开。格式错误、unknown支持和超时不转成拒答。

```bash
go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestAbstention' -race -count=1 -v
RAG_GENERATION_E2E=1 RAG_GENERATION_PROFILE=qwen3 RAG_GENERATION_DATASET=heldout-v3 RAG_GENERATION_PROMPT=rag-draft-v4 go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestActualHeldoutDrafts$' -race -count=1 -v
```

使用此前准备的Qwen资产/隔离Python绝对路径。Prompt v3保留原请求，v4只追加互斥回答/拒答要求；数据集/提示都由应用白名单选择，不从模型输出加载文件或脚本。默认v1/v2继续使用提示v3，heldout-v3默认提示v4。

[heldout-v3.json](testdata/heldout-v3.json)在执行前冻结，四个应答/两个应拒答，期望字段不进入模型。实际Qwen交付4/4、预期摘录覆盖4/4、正确拒答2/2、误拒答0/4；空证据不调用模型。新数据集不同，不能将它与v2直接相减当成提示或协议增益。运行通过不证明广泛NLI、引用真实性或真人校准，已观察后转回归集。

测试还证明：合法模型拒答不调用事实Judge；对可答问题拒答虽为合法协议，仍是任务Eval失败；混合/未知/任意原因、迟到拒答、跨Scope及坏JSON分别拒绝。所有案例保留原始文本/Token和错误，不省略误拒答样本。

## 实验版本与结果记录

这些记录解释开发与验收中的失败，不能合并为一个模型成功率。每次更换数据集或协议，固定新版本再执行；观察后的任务转为回归资料。

| 数据与配置 | 实际结果 | 结论范围 |
| --- | --- | --- |
| 六条开发题、SmolLM2提示v1 | 六条复述输入JSON，均为草稿失败 | 语法有效不代表应用协议有效 |
| 相同开发题、提示v2 | 出处失败2、草稿失败2、输出上限2，交付0 | 新增格式示例混入“8秒租约”与example ID，也出现额外字段 |
| 输入投影修正、提示v3开发题 | 摘录交付1/6，但包含重复断言 | 交付契约通过不证明完整任务成功 |
| heldout-v1、SmolLM2提示v3 | 可答交付0/4、预期摘录0/4、正确拒答1/2 | 四条未交付；无关资料为协议失败，空资料在生成前拒答 |
| heldout-v2、SmolLM2提示v3 | 无Judge交付1/4、整段原文Judge交付1/4、正确拒答1/2 | 标签不进入模型，固定输入与生成上限 |
| heldout-v2、Qwen3提示v3、thinking关闭 | 无Judge交付2/4、整段原文Judge交付3/4、正确拒答1/2 | “13 seconds.”短Quote配完整Text获全文身份确认；围栏和拒答结构失败保留 |
| heldout-v3、Qwen3提示v4 | 应答4/4、预期摘录4/4、正确拒答2/2、误拒答0/4 | 空资料不调用模型，无关资料经真实模型拒答；任务集不同，不计算跨集增益 |

开发题包含超时、取消、重试、引用、版本变化及无资料支持的票价问题；v2/v3的四个可答问题与两个应拒答问题分别保留在testdata。Rubric只检查预期逐字摘录覆盖与明确拒答，不能评估任意释义蕴含、资料真实性或复杂答案完整性。模型及分词器/模板不同，配置对照不单独归因于参数量；完整模型资产、版本、处理与预算见前面的锁定说明。生成记录的出处、结构与未知支持失败均保留，未做真人语义Judge校准。

## 从真实检索候选到回答验证

两个独立Lab通过`retrieved-evidence-v1` JSON连接：MiniLM真实编码与精确top1检索 → 选中原文/ID/来源/版本 → Run前置校验 → Qwen3提示v4生成 → Parse/引用/整段原文支持检查。没有导入其他Lab的私有实现；生成发生在Run校验之后，数据集标签不进入模型。

沿用前文已核对的Python与模型资产，另外按[真实Embedding实验](../neural-retrieval/README.md)准备EMBEDDING_PYTHON与EMBEDDING_ASSETS。只用公开合成demo/v1资料；传入的身份声明不是服务认证或来源真实性证明。生产接入须从可信目录核对候选，不接受任意用户自报的租户、版本和文本。

从仓库根目录执行；第一条成功后再运行第二条。临时文件只含合成公开资料，可在复核后删除：

```bash
export EMBEDDING_E2E=1 RAG_GENERATION_E2E=1 RAG_GENERATION_PROFILE=qwen3
go -C labs run ./part3-agent/chapter28/neural-retrieval -export-evidence > /tmp/retrieved-evidence.json
go -C labs run ./part3-agent/chapter28/evidence-answer -retrieved < /tmp/retrieved-evidence.json
```

检索端在编码前移除旧版与外租户文档；选中原文或查询编码被截断时拒绝导出。接收端限制64KiB、六个任务、每任务至多一个候选与4096字节原文，核对协议/编码身份、重复或大小写歧义字段、固定范围与来源形状。参数缺失、错误资产或协议不一致不能用默认fake替代。ReadRetrieved中的fixture来源检查只确认约定形状，不是任意来源的密码学认证。

实际六个任务均保留；五个有相关性标签的问题命中对应原文，无资料支持的问题仍命中cancel近邻。回答层得到：

| 任务 | 命中 | 回答验证结果 | 输入/生成Token |
| --- | --- | --- | --- |
| duration | timeout | accepted_extract，逐字摘录交付 | 218/63 |
| retry | retry | support_unknown，不交付未经确认的释义 | 219/58 |
| cancel | cancel | accepted_extract，逐字摘录交付 | 217/63 |
| cache | cache | draft_failed，不修补后计为原模型成功 | 220/64 |
| citation | citation | abstained，可答问题误拒答 | 220/21 |
| unsupported | cancel | abstained，无资料支持问题正确拒答 | 219/21 |

五个有资料任务可交付摘录2/5，误拒答1/5；无资料支持任务正确拒答1/1。这些分母分别报告，不把格式错误或支持未知合并成拒答。逐字支持通过不等于完整任务成功，通用释义Judge与真人校准仍未完成；既有任务已经公开且用于诊断，不是新增独立泛化集。运行含逐任务加载模型，不能将耗时当作复用服务的延迟基准。

JSON报告保留模型revision、提示版本、每条状态、Token、生成截断标记和经过验证的Answer；失败不留下部分Claims。DiagnosticDraft仅作离线诊断，包含未经验证的原始草稿，不能当作业务回答交付。实际候选与全部原始草稿冻结在[testdata/retrieved-regression-v1.json](testdata/retrieved-regression-v1.json)；TestRetrievedRecordedOutcomes默认离线重放这些草稿的验证结果，不能计作新的真实模型执行。retry的释义需要通用支持判断；cache将断言字段拆进两个不完整对象，Parse直接拒绝。默认实验仍离线，无E2E开关时不能输出伪造的真实检索候选。练习：在独立语料中加入一个相近但不能回答的片段，先冻结相关性与应拒答标签，再比较top1/top-k对拒答、输入预算与支持未知的影响；标签只能进入评分侧。

## 知识链路试学

[试学材料](reader-check.md)串联Retrieval/RAG/Eval：先预测候选、输入、出处、支持与完成的区别，再自行构造新的两项要求及部分回答反例。原测试作为回归，新迁移测试由读者编写；真实反馈才用于该环节学习验收，不从固定标签或材料存在推导真人校准。

## 迁移练习

1. **引用正确但断言错误**：将30改成60，保留真实引用，确认不会通过支持判断；给每条失败保留可定位原因，不把原文秘密放入错误。
2. **未知支持**：让Judge返回unknown或错误，分别设计复核与有限重试，不能公开未经验证结果。
3. **受控自由生成**：使用Model接口接入已配置Provider adapter，传递context和输入/输出预算，默认测试仍用fake；保存资料、模型、参数与Judge版本，补真实样本及人工校准。未执行前不填写真实模型支持率。
4. **部分回答**：明确哪些未完成项可以说明、哪些任务要求整体拒绝，再实现独立契约；不能简单删除失败句子后声称任务完成。
5. **固定与按需检索**：先画出一次固定检索到Run的交接，再设计最多两次检索的Loop变体。用固定工具让首轮只返回目录、后续读取才提供参数，核对新观察进入输入及预算耗尽行为；最终仍走同一引用与支持出口。当前Run不会自主追加查询，不把设计或fake变体称为真实模型检索收益。

原Eino组装练习保留在[eino-rag-project](../eino-rag-project/README.md)。

## 来源与许可

原创代码，MIT。标准库语义见[encoding/json](https://pkg.go.dev/encoding/json)、[context](https://pkg.go.dev/context)。

本地模型按模型卡Apache-2.0；Transformers和safetensors按Apache-2.0，PyTorch与传递依赖保留各自上游许可。只分发原创封装与元数据，不分发权重或运行时。

## 生成前与交付前回查当前资料

current.go的RunCurrentJoint接收应用注入的CurrentEvidenceReader，旧RunJoint保持不变。Reader按可信tenant和ID取得当前Evidence并执行读取政策；固定测试不提供真实身份系统。输入最多8项，重复ID或错租户在读取前拒绝；生成前及成功结果交付前核对全部字段，更新、删除或读取失败返回空JointAnswer。模型拒答也复核。

先确认测试存在，再运行：

```bash
go -C labs test ./part3-agent/chapter28/evidence-answer -list '^TestCurrentEvidence'
go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestCurrentEvidence' -race -count=1
```

两个测试覆盖未变化正常交付、同版本正文变化、资料版本变化、删除、跨租户、第二次读取故障、取消、拒答期间更新，以及预检失败时不调用模型。读取故障保留原error链，资料变化返回ErrEvidence；检查后立即发生的变化与多记录一致快照不由包装保证。已发生的生成与读取仍需由上层记录消耗，失败不返还预算。

迁移练习从current_test.go与current.go开始：自行增加TestExerciseUnusedEvidenceChanges，提供两份输入、只引用一份，在生成期间更改未引用的一份。验证严格策略仍不交付；解释如果改为仅检查引用列表，会遗漏哪些生成依赖。新测试由学习者创建，先-list确认后运行，原测试不能替代该任务。真实SQL/Redis授权及客户端交付另行联合验证。

## 原始草稿编码不能靠JSON解码修复

Parse与ParseJoint都在结构解码前检查UTF-8，非法字节返回ErrDraft。单引用入口此前会接受Go解码器替换后的文本；原记录保持不变，重放按当前验证契约判断。合法的Unicode替换字符仍允许，不将所有出现该字符的文本当作坏编码。

```bash
go -C labs test ./part3-agent/chapter28/evidence-answer -list '^TestDraftRejectsInvalidUTF8BeforeJudgment$'
go -C labs test ./part3-agent/chapter28/evidence-answer -run '^TestDraftRejectsInvalidUTF8BeforeJudgment$' -race -count=1
```

测试在text、quote与evidence_id中插入非法字节，核对Parse返回ErrDraft，Run不进入Judge、不交付断言；另检查合法Unicode文本可解析。本检查针对原始UTF-8字节，不额外实现JSON转义代理对规范化。

<a id="book-ch28-rag-app-输出解析要有界"></a>

## 输出解析要有界

### 输出解析要有界

实验限制草稿32 KiB与16个断言，先拒绝非法UTF-8，再检查重复键、未知字段、空内容及多余JSON负载：

```go
func Parse(raw string) (Draft, error) {
	if len(raw) > 32<<10 || !utf8.ValidString(raw) {
		return Draft{}, ErrDraft
	}
	if err := uniqueFields(json.NewDecoder(strings.NewReader(raw)), 0); err != nil {
		return Draft{}, ErrDraft
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var draft Draft
	if err := dec.Decode(&draft); err != nil {
		return Draft{}, fmt.Errorf("%w: %w", ErrDraft, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Draft{}, ErrDraft
	}
	if draft.Abstained {
		if len(draft.Claims) != 0 || draft.Reason != "insufficient_evidence" {
			return Draft{}, ErrDraft
		}
		return draft, nil
	}
	if draft.Reason != "" || len(draft.Claims) == 0 || len(draft.Claims) > 16 {
		return Draft{}, ErrDraft
	}
	for _, c := range draft.Claims {
		if strings.TrimSpace(c.Text) == "" || c.EvidenceID == "" || c.Version == "" || strings.TrimSpace(c.Quote) == "" {
			return Draft{}, ErrDraft
		}
	}
	return draft, nil
}
```

重复键需要另行检查：Go的JSON解码器默认可能用后出现的值覆盖前值，DisallowUnknownFields并不禁止重复。uniqueFields先遍历对象与数组，在每个对象内按小写形式核对字段名，并限制嵌套深度；claims与Claims不能互相覆盖，不同断言对象仍可以各自拥有text字段。拒绝歧义之后，再做结构解码和业务校验。

Go的JSON解码器会将字符串中的非法UTF-8字节替换成Unicode替换字符，随后仍可能返回成功。这样得到的断言已经不同于模型原始输出，不能继续把它当作完整草稿。入口先检查原始字节，非法编码归为草稿失败；原文本身包含合法的替换字符则按普通Unicode处理。原始输出仍保留用于排查，不静默修补后覆盖记录。

这些是教学契约的上限，不代表所有业务最佳配置。真实Provider还需要在读取前限制响应体，并配置生成长度；拿到字符串后再拒绝不能撤销费用或避免此前分配内存。

校验失败时可以在预算内修复格式，但不能把未经验证的草稿先流式发布，再希望最终校验纠正。需要边生成边展示时，应区分未验证预览与已验证业务结果，并定义用户看见部分内容后的处置；本实验只在整份回答通过后交付。


<a id="book-ch28-rag-app-数值正确也可能缺少推导前提"></a>

## 数值正确，也可能缺少推导前提

### 数值正确，也可能缺少推导前提

给生成模型两份资料，并不保证它把两份依据都写进引用。实际本地生成的一条断言如下，原始记录见[联合生成对照](testdata/joint-generation-1.7b-v2.json)：

```json
{"text":"Harbor current timeout is 43 seconds.","citations":[{"id":"policy","version":"1","quote":"Harbor timeout is 6 seconds when enabled and 43 seconds when disabled."}]}
```

数值与当前部署一致，但交付仍有两项缺口：`id`不符合`evidence_id`契约；唯一引用只说明条件规则，没有引用当前开关状态。即使应用只把字段名改对，也只能让草稿进入支持判断，不能替模型补出缺失前提。联合Judge仅收到该断言引用的资料，不能从请求中其他未引用文档“借用”依据后宣称引用完整。

这类问题按四步定位：

1. **检查生成是否完整**：保留原文、截断与用量，未完成输出不进入后续判断。
2. **检查结构与归属**：字段名、ID、版本与摘录必须符合契约；格式拒绝不能记作正确拒答。
3. **检查推导前提**：每个条件都能回到引用集合；输入里有资料、引用里没资料是两种状态。
4. **检查任务回答**：复述“开启6秒、关闭43秒”没有回答当前走哪个分支；缺少状态时应说明不足，而非猜数值。

同一模型配置对照还出现过“当前300秒”的草稿，它引用的部署文本只包含开关状态。出处与摘录可以真实存在，断言却没有支持。这样的结果应拒绝或待复核，不能把引用有效率当作答案正确率。

当前联合生成对照使用锁定的本地Qwen配置与相同预算，原文直接进入RunJoint；固定规则Judge只验证这个条件案例。0.6B两种提示均未通过解析，1.7B出现字段错误与待复核结果，没有交付合格组合回答。它们说明生成、引用与支持可以在不同位置失败，不证明模型配置的普遍优劣；通用语义Judge、人工校准与独立任务仍需验证。


<a id="book-ch28-rag-app-输入标识对齐仍不替模型补引用"></a>

## 输入标识对齐，仍不替模型补引用

### 输入标识对齐，仍不替模型补引用

输入资料用id，输出要求evidence_id，模型可能沿用输入字段名；减少这种格式噪声应在输入适配中完成，而非在输出失败后改字段冒充成功。显式引用投影让资料按evidence_id、version、text呈现，原文与可用范围保持；实际输入消息与原始草稿分别记录，便于定位变化发生在哪一层。

真实对照中，字段名已正确的回答仍只引用策略，没有引用当前部署状态；另一份输出把citations写成字符串数组；仅有开关状态时还出现“当前60秒”的断言。三种问题分别属于前提缺失、引用结构错误和无依据数值，不能由字段对齐一起修好。

因此按顺序核对：**实际输入 → 草稿结构 → 引用集合 → 联合支持 → 任务完成**。Go提交载荷摘要定位伴随程序的stdin，保存的messages定位模型消息视图，两者不是相同记录；聊天模板和分词还会进一步编码消息。必要引用和评分答案不进入生成器，模型漏掉依据时仍拒绝或待复核。配置、全部失败与用量见[输入引用键对照](#v3输入引用键与实际消息)。


<a id="book-ch28-rag-app-逐字原文与一般释义采用不同检查"></a>

## 逐字原文与一般释义采用不同检查

### 逐字原文与一般释义采用不同检查

断言如果与整段原文完全相同，可以用[确定性Judge](extract_judge.go)确认文本身份：

```go
func ExactSourceJudge(ctx context.Context, claim Claim, evidence Evidence) (Verdict, error) {
	if err := ctx.Err(); err != nil {
		return Unknown, err
	}
	if claim.Text == evidence.Text {
		return Supported, nil
	}
	return Unknown, nil
}
```

Validate在调用前已经核对ID、版本和Quote归属。这个Judge只比较完整evidence.Text与claim.Text；数值变化和一般释义仍为unknown。Quote较短时，用户可以回查完整资料，应用也用完整证据判断支持。逐字匹配仍不检查资料真实性、任务相关性或断言是否脱离语境。

比较生成配置时固定问题、资料和生成上限；比较验证策略时使用同一批生成记录。分词器、聊天模板与thinking开关也影响配置，差异不能全部归因于参数量。原文匹配规则接受了更多逐字原文，也不代表模型的语义推理增强；一般释义的支持判断需要另外校准。


<a id="book-ch28-rag-app-排名更好为什么仍可能缺前提"></a>

## 排名更好，为什么仍可能缺前提

### 排名更好，为什么仍可能缺前提

回答“当前超时是多少”需要当前部署状态和适用规则。规则片段排到第一位，却没有取到部署状态，仍不能选择正确分支。下面是假设的排序例子，不是配套实验的实际分数：

| 排名 | 合法候选 | 对当前问题增加什么 |
| --- | --- | --- |
| 1 | 当前策略中的完整超时规则 | 说明开关开启/关闭分别采用哪个数值 |
| 2 | 同规则的重复片段 | 没有补充当前开关状态 |
| 3 | 客户端重试说明 | 主题相关，但不决定服务端超时分支 |
| 4 | 当前部署的开关状态 | 决定规则中的哪个分支适用 |

只取前两项，会得到很集中的超时描述，必要前提仍未齐。增加重复片段的排名不能弥补缺失状态；扩大top-k也不是总能解决，更多噪声可能挤占Context预算。

评估时可以为任务预先标注**必要证据组**。每组代表一个必须有依据的前提，组内列出经过复核、可以替代的来源；所有组都有合法证据才算资料齐备。这里至少有“当前部署状态”和“当前适用规则”两组。若原始规则与另一份批准的完整规则等价，可以列为同组替代项；只有主题相似或相互复制，不能自动认定可以替代。

这类标注属于独立评分依据，不能把隐藏的必要来源清单塞入模型后再报告检索成功。生成器得到用户问题与实际允许的资料，评估侧再核对三个位置：

1. **候选覆盖**：各必要组是否已有正确实体、版本和范围的证据。没有取得状态，先查召回与资料覆盖。
2. **输入保留**：候选虽齐，Context是否保留了完整状态、条件和规则。状态被预算选择丢掉，先查组装，不重新责怪检索器。
3. **答案支持**：输入虽齐，断言是否引用并使用了全部必要前提。这里只是进入支持判断的条件，仍要检查事实与任务完整性。

报告同时保留逐任务缺失组和“全部必要组齐备”的任务数；分母包含已经标注且实际进入该评估范围的全部任务，失败与缺失结果不能删除。没有标注的任务另报覆盖数量，不能擅自当作前提齐备。组内替代项尚未复核时保留未知，不用来源数量凑覆盖。

必要组覆盖与Recall、排序指标回答不同问题：前者检查一项任务是否凑齐全部前提，后两者检查标注资料的命中与顺序。覆盖全部组仍不能证明资料真实或生成正确，也不应强迫自由任务必须采取唯一检索路径。

[Eval的集合评分契约](../../chapter33/eval-harness/evidence_coverage.go)已实现给定标注下的覆盖检查。每组命中一个复核后的替代来源才取得该前提；都未命中记为缺组，核心代码和分母解释见[Eval的必要证据评分](../../../../book/part3-agent/governance/chapter35-evaluation.md#必要证据组怎样评分)。

集合评分不决定两个来源是否语义等价，也不从SourceID推断当前授权。入口先核对标注版本、Case/Run身份和允许来源，再分别检查候选与输入。缺失运行与未观测阶段保留在已标注任务分母，只有明确观测的空结果才列出全部缺组。通用任务标注、真实执行记录和替代证据的人工依据仍需补齐。

已有[联合生成的输入捕获对照](#实际提交输入与覆盖评分)将实际提交摘要与新记录绑定。三项条件任务中，一项输入包含状态和规则，另外两项各缺一组；这里没有检索阶段，候选覆盖保持未观测。输入齐全的一项仍出现坏引用字段与缺少状态引用，最终三项都未交付合格答案。这说明前提覆盖帮助定位问题，不能代替格式、引用支持和任务验收。


<a id="book-ch28-rag-app-把改动定位到实际失效的阶段"></a>

## 把改动定位到实际失效的阶段

### 把改动定位到实际失效的阶段

| 层次 | 要回答的问题 | 可用证据 |
| --- | --- | --- |
| 检索 | 必要资料是否进入候选 | 版本化相关性标签、Recall与排序指标 |
| 输入 | 关键信息是否进入本轮请求 | Context选择记录、预算与必要事实检查 |
| 生成 | 断言是否完整且回答任务 | 任务Rubric、完整性与正确性评分 |
| 引用 | 断言能否定位有效证据 | ID、版本、摘录归属与支持判断 |
| 运行 | 超时、取消和成本是否符合限制 | Trace、错误分类、usage与费用记录 |

结果按“生成完成→结构合法→引用归属→断言支持→任务完成”逐层记录。截断、坏字段、伪来源与未知支持分别计数，失败样本和未进入后续层的原因保留。只有引用通过的释义才能进入支持判断；报告支持率时同时报告覆盖与分母，避免从极少数筛选后的句子推断整个系统质量。

Judge校准样本应包含正确释义，以及数值错误、否定翻转、范围扩大和上下文遗漏。真人判断分歧也保留，不能由模型自报“已得到支持”替代独立判断；当前实验未完成真人语义校准。

[ALCE](https://aclanthology.org/2023.emnlp-main.398/)将答案正确性与引用质量分别评估。本项目同样不能用“有引用”代替正确性：每条引用能否支持对应断言，以及答案中的必要断言是否都获得依据，是两项不同的检查。拼上很多来源可能掩盖无用引用，只给少量有效引用也可能漏掉关键结论；评分器准确性及覆盖分母继续按Eval章验证。

改检索、换模型或缩短上下文时，应同时看这些层次。检索Recall提高却带入噪声，可能降低回答效果；拒答率降低也可能意味着系统开始猜测。

用受控自由生成评估释义支持时，保存任务、资料版本、模型配置、生成参数和评分器版本，并用真人样本校准Judge。评分器的偏差与不确定性也要报告，不能由模型自报“已得到支持”。默认离线实验先验证机制，真实模型效果另行验证。


<a id="book-ch28-rag-app-最佳实践"></a>

## 最佳实践

## 最佳实践

### 输出前建立验证出口

将草稿、归属和支持检查放在同一交付路径上。任一关键断言失败，整份回答停止公开；需要部分回答时，另行定义可接受部分、缺失标记与任务完整性，而非随意删掉失败句子。

### 保留原文与版本

引用检查依赖当时使用的证据。原文更新后，不应悄悄用新版本重新解释旧回答；缓存也要绑定资料版本与访问范围。

### 把拒答作为受控结果

拒答可以是正确行为，也可能是不必要的放弃。数据集同时包含“应该回答”和“应该拒答”的任务，分别观察误答和误拒答，避免只优化一个比例。


<a id="book-ch28-rag-app-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 有真实引用，答案仍然错误

先看具体断言，再看引用的条件、单位与版本。出处有效只能排除伪来源，不能替代支持判断；制造“引用正确、数值改错”的反例，确认不会通过。

### Judge不可用时仍显示正常答案

检查是否将错误或unknown默认为true。将支持判断服务设为失败，答案不得经过成功出口；按明确策略重试、拒绝或转人工，不能静默跳过检查。

### 资料中的指令改变了执行权限

先检查工具清单和应用授权，确认来源内容只作为数据使用。Prompt可以提醒模型，服务端策略必须拒绝越权动作；引用校验无法替代工具权限控制。

