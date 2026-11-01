# 实验：输入预算与完整观察组

构建本轮模型输入视图：稳定system与当前用户轮必保留，旧用户轮整体选择，调用ID与工具结果必须完整，原历史不被修改。标准库、固定数据、无需密钥。

新增结构化摘要保留检查和显式启用的本地模型生成；默认不加载模型，不实现长期Memory或产物存储。Build的estimate使用码点与固定开销，单位是教学估算量，不是真实Token；生成伴随实验另记本地分词器的实际用量。

## 前置与入口

先读[Context正文](../../../../book/part3-agent/building/chapter26-context.md#topic-agent-context)及[Session](../../../../book/part3-agent/building/chapter26-context.md#topic-agent-session)。Go准备见[运行约定](../../../../book/introduction.md#running-examples)。

- [main.go](main.go)：消息配对、整轮分组、Counter与输入选择。
- [main_test.go](main_test.go)：必要输入不足、完整组、输入所有权、请求封装开销及协议拒绝。
- [summary.go](summary.go)：当前事实快照、候选摘要校验与六种固定候选。
- [summary_test.go](summary_test.go)：遗漏、旧条件、错误来源、未知变确定、版本失效、取消与输出所有权。

## 预测与运行

历史包含一轮旧问答，以及当前问题、一个read提议和对应观察。预算只够规则与当前完整轮，旧轮是否会留下半条消息？

仓库根目录执行：

```bash
go -C labs run ./part3-agent/chapter29/context-builder
go -C labs test ./part3-agent/chapter29/context-builder -race -count=1
go -C labs test ./part3-agent/chapter29/context-builder -cover
go -C labs build -o /tmp/context-builder ./part3-agent/chapter29/context-builder
```

输出中messages=4、dropped_turns=1：system、user、assistant提议、tool观察。Used由明确的教学Counter计算，不拿它作为实际账单。

## 关键观察

| 变化 | 预期行为 |
| --- | --- |
| 减少Window一个单位，必要输入不够 | ErrBudget，不能得到半个当前轮 |
| 删除当前tool观察 | ErrProtocol，不发起下一轮模型调用 |
| 工具结果引用未知ID | ErrProtocol |
| 增大窗口 | 完整历史按原顺序保留 |
| 修改输出视图中的JSON参数 | 原始历史不变 |
| Counter额外包含请求封装成本 | 按完整候选计数，不遗漏额外成本 |

当前算法会跳过太大的旧轮，再尝试较小的更早轮；历史可能非连续。它只检查协议与预算，不证明必要事实保留或模型正确使用输入。历史上限128条、每消息64个提议；历史文本、调用ID、工具名与参数累计上限64 KiB，system另有64 KiB上限；OutputReserve需在真实adapter中对应到实际生成上限。

## 摘要有没有保住下一步所需的事实

应用登记当前预算6000元、离线为必需条件、方案B离线能力未知。候选摘要来自固定数据，校验器不从历史自动提取事实；它对照应用提供的当前快照，而不是让候选自报“已经保留”。

```bash
go -C labs run ./part3-agent/chapter29/context-builder -summary
go -C labs test ./part3-agent/chapter29/context-builder -run '^TestSummary' -race -count=1 -v
```

六行输出中，只有complete为accepted=true、published_facts=3；missing、old_amount、wrong_condition、false_source、unknown_to_known均为accepted=false、published_facts=0。先预测：为什么unknown变成known即使写的是“支持”，也不能通过？

Snapshot.Version绑定当前有效状态；事实的Key同时明确字段和单位，Value保留当前值，Source定位用户要求或资料缺口，State区分known与unknown。unknown的Value必须为空，避免结构一边说未知、一边携带未经确认的结论。最多32项、字段总量64 KiB，拒绝重复键、缺失和额外项。

校验先核对完整输入，再比较集合和各字段，最后检查取消并返回独立副本；不接受任何半份摘要。候选顺序可以不同，但不能改变条件或来源。版本或来源ID本身不提供授权，可信快照必须由应用取得；两份同样错误的结构仍会匹配成功，因此事实提取与语义支持须另外验证。

这里的结构化摘要是任务事实视图，不是自由文本压缩器，也不能替代当前轮的tool消息配对。旧消息仍由Session保存，Build仍采用原先整轮选择；将事实视图用于模型请求时还需经过预算、角色与授权检查。以上固定候选检查不调用模型，没有Token节省或回答质量结论；真实生成诊断见下节。

## 本地模型能否保留有效条件

[compression-case.json](compression-case.json)冻结一个开发任务，包含预算更新、未变的离线要求、方案B未知，以及无关对话。模型分别读取完整历史和最后三项；第二种输入本来就缺少关键事实，用于诊断信息不足，不是公平比较两个完整压缩算法，也不是独立泛化集合。

[compression_model.py](compression_model.py)只投影历史、任务与应用版本，不传Go的有效事实快照；公开字段约定说明摘要如何表示数值、来源和known/unknown。应用预填Version及Facts开头，实际生成后缀单独记录；这部分前缀不计作模型生成Token。输出不去围栏、不补字段、不修复。触及输出上限时保留原始候选，但不进入保留校验。

模型资产与依赖分别锁定在[model-lock.json](model-lock.json)和[generation-requirements.lock](generation-requirements.lock)，复用课程已有的Qwen3-0.6B固定revision。准备独立Python 3.12.14运行时及已校验资产，路径由本地配置；不下载权重到仓库。加载只使用本地safetensors，不执行远程代码，CPU单线程、非thinking、greedy；输入上限1536、输出上限320、整体期限180秒。

从仓库根目录运行，CONTEXT_PYTHON与CONTEXT_ASSETS分别指向已准备的锁定解释器与模型目录：

```bash
go -C labs build -o /tmp/context-summary-validator ./part3-agent/chapter29/context-builder
CONTEXT_COMPRESSION_E2E=1 "$CONTEXT_PYTHON" labs/part3-agent/chapter29/context-builder/compression_model.py \
  --assets "$CONTEXT_ASSETS" --validator /tmp/context-summary-validator \
  --output /tmp/context-compression-result.json
```

报告保存数据摘要、配置、每条模型输入、原始输出、实际Token、截断和独立校验结果；complete只表示两次实验都结束。Go入口读取64 KiB以内的原始候选，拒绝非UTF-8、重复/大小写别名字段、未知字段、null或数字替代字符串及多余负载，再比较保留快照。

```bash
go -C labs run ./part3-agent/chapter29/context-builder -summary-candidate /tmp/context-compression-result-full.txt
```

此命令只检查文件中的摘要形状与字段保留；先核对生成报告的at_output_limit与对应输入，不能将另一任务或截断输出当作完整生成。字段保留仍不证明原资料真实、自由摘要语义或回答质量。失败后的修复另存新配置和记录，不覆盖原始失败。

### 已记录的开发诊断

[原始报告](testdata/compression-development-v1-result.json)保存两次实际生成，无输出截断，两条均未交付事实：

| 输入 | 输入/输出Token | 输出中的问题 | Go检查 |
| --- | --- | --- | --- |
| 完整历史 | 874 / 96 | 预算仍为5000；离线来源写成task；unknown携带非空Value | invalid_shape，published_facts=0 |
| 最近三项 | 294 / 90 | 预算用current占位；离线用mandatory代替布尔字符串；unknown携带非空Value | invalid_shape，published_facts=0 |

保留检查先拒绝unknown的非空Value，因此未进入后续快照比较；报告中的其他事实问题来自人工阅读原始字段，不伪称评分器已逐项给出错误分类。完整输入已经包含6000元更新，仍不保证模型正确保留。最近输入更短，但缺少必须信息，不能据此宣称节省Token而质量不变。

这是一个开发任务的两种输入，不能估计通用压缩成功率或长期回答质量。下一步可针对状态表示与来源选择修订生成契约，再以新任务独立验证；原始失败保留为回归，不直接改写成通过。默认Python检查核对投影、截断、显式门禁和原始记录；Go重放核对真实失败不交付，但都不重新运行模型。

## 故障与迁移练习

先改变输入条件并预测结果，再在副本中实现变体；原历史保持可读取，新机制需要自己的测试。

| 任务 | 起点与改动 | 反例与完成证据 |
| --- | --- | --- |
| 1. 保护任务约束 | 从main.go的Build与TestWholeTurnSelectionAndOwnership起步，旧轮加入预算，当前问题引用“之前的预算”；设计单独保留约束及出处的输入契约 | 先复现旧轮被丢弃，随后核对预算、来源与本次输入；修改约束后旧值不能继续生效。说明事实由谁提取，关键词命中不能冒称通用指代理解 |
| 2. 连续后缀选择 | 保留turns的完整分组，在Build选择循环旁实现独立的最近连续轮策略，与原可跳轮策略比较 | 构造“最近旧轮很大、更早旧轮很小”的历史；记录两策略的保留轮与Used。两者都保护调用/观察关联、最新轮和system，不修改原历史；少保留几轮不能自动解释为质量下降 |
| 3. 适配计数器 | 从Counter及TestAllHistoryAndFramingCounter起步，在调用层绑定工具定义和Provider封装，加入固定额外开销 | 使同一消息集在裸消息预算内能放下、完整封装后放不下；应在模型调用前拒绝。默认只用固定计数，真实tokenizer另记录版本与误差，不能把码点估算当真实Token |
| 4. 检查摘要损失 | 从summary.go的ValidateSummary及summary_input.go的ParseSummary起步，自行增加数值、来源、known/unknown和版本变体 | 错数值、漏来源或把未知改为已知均拒绝，原快照不被部分修改；显式字段全部保留仍不证明自由文本含义正确。真实摘要另保存原输出及独立支持判断 |
| 5. 处理多来源更新 | 在Build调用之前设计消息、偏好与资料的输入投影；固定本次详细要求与长期简洁偏好，再模拟偏好删除或资料换版 | 自行验证本次要求优先、已删偏好不进入新输入、旧资料不成为当前事实。分别核对来源自己的版本，说明消费前复核位置；Build不提供跨存储事务，不能比较不同来源版本号的大小 |

先核对已有基线名称，再执行：

```bash
go -C labs test ./part3-agent/chapter29/context-builder -list 'TestWholeTurnSelectionAndOwnership|TestAllHistoryAndFramingCounter'
go -C labs test ./part3-agent/chapter29/context-builder -run '^(TestWholeTurnSelectionAndOwnership|TestAllHistoryAndFramingCounter)$' -race -count=1 -v
```

迁移后自行补测试，并用-list确认存在；no tests to run或只跑通原例都不能说明新策略已经验证。输入变短、字段保留和任务完成分别记录，不能互相替代。

## 来源与许可

课程原创代码，MIT。标准库参考[encoding/json](https://pkg.go.dev/encoding/json)、[unicode/utf8](https://pkg.go.dev/unicode/utf8)。

<a id="book-agent-context-看见更新不等于保留了更新"></a>

## 看见更新不等于保留了更新

### 看见更新不等于保留了更新

[本地生成对照](#本地模型能否保留有效条件)让固定模型分别读取完整历史和最近几项。模型只看到对话与任务，评分快照留在Go侧；原始输出没有自动修复。

完整历史中已经有6000元更新，模型却仍写5000元；离线要求引用了不存在的task来源。两种输入还都把unknown的Value填成文字，违反约定，因此没有交付摘要。详细配置、原始候选和分层结果在配套实验中保留。

这类失败需要分层处理：先确认候选是否符合结构与状态约定，再检查字段是否保住当前事实，最后评估输入是否足以支持回答。格式修复不能顺手替模型选择正确金额；只保留最近几条造成的信息缺失，也不能靠修改输出格式解决。该开发诊断不证明压缩策略有效，任务效果仍需新的独立样本。

是否值得压缩，要看接下来的任务。若模型仍能按6000元和离线要求比较、识别B的信息缺口，并回到正确资料，才说明这次压缩保留了相应能力；输入变短只能证明长度变化。长任务中的压缩与按需读取策略可参考[上下文工程实践](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)，效果仍应由本任务的对照决定。

前面判断的是压缩后还保住了什么事实；接下来再看另一个输入工程问题：这些内容在请求里怎么摆更容易被模型使用，哪些派生结果又值得复用但不能被误当成新事实。


<a id="book-agent-context-最佳实践"></a>

## 最佳实践

## 最佳实践

### 保护约束，不保护所有旧文本

把当前目标、已确认的用户约束和必要证据变成有来源的固定输入。普通闲聊与冗长结果按需选择；保护范围过大也可能造成超限，因此需要知道每项信息为什么必要。

### 在信息产生处控制体积

工具端分页、摘要或字段选择通常比拿到大块文本后再裁剪更可控。Context仍做最后的输入检查，两个上限分别保护工具读取与模型调用，不能互相替代。

### 把计数与语义评估分开

确定性Counter检查构建规则，真实tokenizer检查容量；任务评估检查事实是否还足够。三种证据各回答一个问题，不把一种成功替代另外两种。


<a id="book-agent-context-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### Provider拒绝工具消息序列

先核对提议与结果ID，检查裁剪是否拆开关联组，再看adapter角色转换。用完整组输入复验；不要直接增加窗口掩盖协议错误。

### 输入变短后，答案漏掉用户条件

比较选前与选后的必要事实，不先改模型。将条件设为受保护任务信息，保留原文来源，重新评估新问题与长历史；若保护信息仍在但模型忽略，才继续检查布局与模型行为。

### 本地计数未超限，真实请求却失败

核对Counter是否遗漏工具定义、封装与模型特定用量，检查输出预留是否真正传给Provider。改用匹配模型的计数与余量，用受控真实调用验证，不把字符估算当成账单或准确容量。

### 压缩或缓存后仍在使用旧条件

先回查用户更新及其消息版本，再看有效约束进入了哪个输入块，最后检查摘要或View缓存是否沿用了旧快照。使受影响的派生内容失效，重新构建输入，核对旧条件是否仍被重复注入。输入已经正确而模型仍使用旧值时，再检查布局与模型行为；只清缓存不能替代这两层诊断。

