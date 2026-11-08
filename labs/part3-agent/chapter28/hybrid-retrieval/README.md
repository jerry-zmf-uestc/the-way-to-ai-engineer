# 实验：关键词、向量与排名融合

用版本化片段和固定教学向量，验证可信租户过滤、词法/向量两路排序、RRF融合与重排失败传播。默认离线，只依赖标准库。

本实验不实现真实Embedding、BM25、ANN或远程重排；词重合只用于解释词法路线，向量由教学数据明确给定。真实语义效果需要独立资料集与服务对照，不由这里的命中证明。

## 前置与入口

阅读[Retrieval正文](../../../../content/book/part3-agent/building/chapter27-rag.md#topic-ch28-vector-retrieval)与[Context](../../../../content/book/part3-agent/building/chapter26-context.md#topic-agent-context)。Go准备见[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：码点切分、向量空间检查、只读索引、预过滤、排名融合与评分回调。
- [main_test.go](main_test.go)：召回互补、租户隔离、版本与复制、非法空间/向量、取消及重排故障。
- [ranking.go](ranking.go)：独立标签的Recall、单查询RR与分级nDCG，未判断结果与分母状态。
- [ranking_test.go](ranking_test.go)：相同命中不同顺序、分级优先级、截断、未知及非法排名。

## 预测与运行

关键词E42只命中第一个片段，向量路线优先返回第二个。融合后哪个片段领先？先计算各路排名贡献，再运行。

```bash
go -C labs run ./part3-agent/chapter28/hybrid-retrieval
go -C labs test ./part3-agent/chapter28/hybrid-retrieval -race -count=1
go -C labs test ./part3-agent/chapter28/hybrid-retrieval -cover
go -C labs build -o /tmp/hybrid-retrieval ./part3-agent/chapter28/hybrid-retrieval
```

候选带LexicalRank和VectorRank，0表示未进入该路候选。融合贡献使用1/(60+rank)，同分按ID稳定排序。candidateK限制每路候选，topK限制最终结果，两者各有正值及上限检查。

## 故障观察

- 查询属于另一个tenant：只有该范围资料可以计分，没有资料正常返回空集合。
- 维度相同但Space不同：返回ErrInvalid，不能跨模型空间比较。
- 零向量、NaN、Inf或非法top-k：返回ErrInvalid，不伪装成空检索。
- 返回向量被调用方修改：索引数据不变。
- 重排服务失败：保留错误链；非有限分数拒绝，不能静默变成成功。
- 重排回调与返回候选的向量都是独立副本；TestRerankOutputCannotRewriteOriginalCandidates验证双向修改隔离、nil评分器和超过128候选拒绝。未加载真实交叉编码器，排序效果不由接口通过证明。

索引创建后只读。Split按段落和码点切分，ID绑定来源、版本与序号；更新正文或切分规则必须更新版本。它不是内容真实性验证。

## 相同Recall能否说明排序相同

```bash
go -C labs run ./part3-agent/chapter28/hybrid-retrieval -ranking
go -C labs test ./part3-agent/chapter28/hybrid-retrieval -run 'TestSameRecall|TestUnjudged|TestRankingCutoff' -race -count=1
```

标签critical=3、useful=1、noise=0；两种顺序都命中全部正相关ID，Recall=1。noise/useful/critical的RR=.5、nDCG=.541；critical/useful/noise的RR=1、nDCG=1。将useful放第一位时RR仍为1，但nDCG低于理想排序，说明首命中指标不区分相关等级。

EvaluateRanking最多128个唯一排名ID、128个等级标签，k为1–128，等级为0–3。重复/空ID、坏等级或越界拒绝；标签不进入Index.Search或重排回调。RR只代表本查询，批量MRR需要按相同查询集合汇总。

unknown未判断不能当作grade=0。RR在第一份已知相关之前存在未知项时未定义；第一项已知相关、后面未知时RR仍为1，但nDCG未定义。报告保留Judged/Ranked，Recall仅针对已标注正相关集合，理想排名也仅针对该标签池，不声称全库标注完整。

无正相关标签时三个Known标记为false，不显示成功；有正标签但空返回时分数为0。服务错误先保留错误，不能作为空返回正常评分。这个原创固定案例验证指标口径，不代表模型、真实重排或生产检索收益；原默认融合输出不变。

## 迁移练习

1. **两路对照**：构造精确错误码、同义表达与无关资料，分别记录单路与融合排名；教学向量验证机制，接真实Embedding时另行标注Recall和延迟。
2. **候选池与重排**：让必要片段排在候选池之外，观察重排不能找回它；增加candidateK后比较质量与调用成本。
3. **结构切分**：保留代码块或表格为完整单元，加入多字节字符与版本更新测试；不要随意截字节。
4. **来源更新**：替换同Source的新版本，设计旧索引与旧引用的处理；不要在原ID下静默改正文。

原基础练习28.1/28.2仍在[Eino向量实验](../eino-vector-store/README.md)，没有因新增融合实验而删除。

## 来源与许可

原创代码，MIT。RRF机制参考[官方排名融合说明](https://learn.microsoft.com/en-us/azure/search/hybrid-search-ranking)，未复制第三方实现。

排名公式参考[信息检索教材](https://nlp.stanford.edu/IR-book/html/htmledition/evaluation-of-ranked-retrieval-results-1.html)与[TREC RR定义](https://github.com/usnistgov/trec_eval/blob/main/m_recip_rank.c)。实现和分级教学数据为原创，没有复制TREC代码或评测集。

## 整批索引发布与查询快照

[snapshot.go](snapshot.go)复用NewIndex的整批校验/复制，用Publisher.Publish的expected generation在同一临界区切换当前快照；Capture取得的私有句柄保持旧Index。查询先取得Identity的Generation/Space/Dimensions，再准备对应向量；同维度的新Space不兼容旧查询。代次溢出、构建失败、取消或陈旧发布均不替换当前入口。

```bash
go -C labs test ./part3-agent/chapter28/hybrid-retrieval -run 'TestSnapshotPublication|TestConcurrentPublisher|TestRejectedBuild' -race -count=1 -v
```

[snapshot_test.go](snapshot_test.go)验证v1/space-a与v2/space-b并存时每个句柄只返回本代资料，原向量/返回向量修改不污染索引；两并发发布只一个成功，非法新向量、取消与uint64溢出不产生新快照。旧句柄仍能读旧资料，这正是交付前还需当前目录/删除/权限回查的原因，不是删除保证。

当前为同进程内存发布，不存Embedding模型、不自动迁移向量库、不实现分布式切换或权威目录。Generation不是身份凭证；caller拥有输入期间不得并发修改它。原检索/评分任务保留，此发布基线不代替更新/删除跨存储或任务质量验收。

## 显式本地交叉编码器

[reranker-lock.json](reranker-lock.json)锁定cross-encoder/ms-marco-MiniLM-L6-v2官方revision与6份资产SHA256；[reranker-requirements.lock](reranker-requirements.lock)沿用课程已准备的Python3.12.14/torch2.9.0/Transformers4.57.1固定runtime。本实验只在RERANK_E2E=1加载本地已校验资产，不自动下载/加载远程代码，不分发权重或runtime。

先按lock中的官方repository/revision/files在自己的本地目录准备并验证资产，再设置RERANK_PYTHON和RERANK_ASSETS。仓库根目录执行：

```bash
python3 - <<'PYINPUT' > /tmp/rerank-input.json
import json
from pathlib import Path
cases=json.loads(Path('labs/part3-agent/chapter28/hybrid-retrieval/rerank-cases.json').read_text())['cases']
print(json.dumps([{k:c[k] for k in ('query','candidates')} for c in cases]))
PYINPUT
RERANK_E2E=1 "$RERANK_PYTHON" labs/part3-agent/chapter28/hybrid-retrieval/local_reranker.py "$RERANK_ASSETS" < /tmp/rerank-input.json
```

模型只看query和id/text，不看grades。整进程60秒、最多6请求、每请求8候选、输入总64KiB、单对512Token，拒绝超长而非截断；CPU单线程、FP32、模型输出原始logit，不是概率。以上是独立Python入口；默认Go测试只重放现有记录，不加载模型。

原始评分在[rerank-result.json](rerank-result.json)，绑定冻结案例文件hash和版本。ApplyCrossScores先拒绝不完整/新增/重复ID、非有限数值与Token越界，再复用Rerank；用调用方的同查询/候选绑定，不把casehash当认证。

| 开发任务 | 原池次序nDCG@3 | 真实重排nDCG@3 | 关键片段名次 |
| --- | --- | --- | --- |
| request-deadline | 0.541 | 0.983 | 3→1 |
| unknown-write | 0.590 | 0.983 | 3→1 |
| deleted-memory | 0.590 | 0.710 | 3→2 |

Recall@3都为1，池和分母不变。等级0无关、1同领域补充、3直接回答关键条件，均为预先冻结的作者标签，未做独立人工校准。候选原次序只是固定演示基线，没有比较真实Embedding/ANN召回。三题的新运行不自动增加合格/独立Eval总数。

实际模型前向约0.014–0.016秒，仅是该CPU/输入下forward计时，未包括资产校验、加载、分词/检索/服务，不是生产延迟。删除题的概念片段仍胜过操作约束；不能把排名改善当作全部条件满足或答案已支持。

```bash
go -C labs test ./part3-agent/chapter28/hybrid-retrieval -run 'TestOriginalLocalCrossEncoder|TestCrossScoreBatch' -race -count=1 -v
```

Go重放核对revision/冻结输入hash、ID/Token/score边界与同池指标；错误评分批次无部分排名。原练习保留，仍需学习者增加新任务/校准标签、真实召回池和端到端答案对照，不提供通用重排效果答案。

## 从真实向量排序到重排

[vector_rerank_compare.py](vector_rerank_compare.py)以固定应用路径调用neural-retrieval的公开encoder.py JSON CLI，再调用本Lab local_reranker.py；没有import其他topic私有实现，不接受模型提供的执行路径。两个模型的输入都是同一冻结query/candidate文本，grades留在调用外。

使用已准备的两套独立固定runtime与资产（EMBEDDING_PYTHON/ASSETS、RERANK_PYTHON/ASSETS），明确启用两个门禁：

```bash
EMBEDDING_E2E=1 RERANK_E2E=1 python3 labs/part3-agent/chapter28/hybrid-retrieval/vector_rerank_compare.py > /tmp/vector-rerank-new.json
```

两子进程各65秒，总150秒；固定revision/Space/384维、数量、有限数值、单位范数和截断检查后，才按同一查询对应的向量计算精确排序。重排集合/ID/512pairToken仍检查。复用公开本地CLI协议，不代表支持任意程序或自动安装；运行时准备仍按两Lab lock核对。

原始[vector-rerank-result.json](vector-rerank-result.json)绑定原case文件hash，保存12个实际向量、9个真实pair评分与两种次序。三题精确向量次序和重排完全一致：nDCG为0.983/0.983/0.710，增益为0；删除题关键片段仍第二。前面固定原次序的改善不能当作相对向量基线的改善。

```bash
go -C labs test ./part3-agent/chapter28/hybrid-retrieval -run '^TestOriginalVectorRerankChainKeepsActualBaseline$' -race -count=1 -v
```

默认只重放原始向量和评分，重新计算余弦顺序与排名指标，检查固定revision/Space/输入hash和截断；不加载模型。语料仍是每题3个允许候选的精确小池，没有ANN/SQL、多租户服务、额外域任务、人工校准或生产性能证据。重排是否值得需相同实际召回器、信息与预算下再判断，原迁移练习保留。

<a id="book-ch28-vector-retrieval-创建了索引还要确认查询真的使用它"></a>

## 创建了索引，还要确认查询真的使用它

### 创建了索引，还要确认查询真的使用它

查询计划受数据规模、选择性、ORDER BY、LIMIT、opclass和优化器选择影响。小表可能选择扫描，创建HNSW以后仍可能没使用它；复杂排序也可能改变路径。

配套适配器检查EXPLAIN，明确区分诊断中的精确与ANN模式：

```go
	if approx && !strings.Contains(plan, "vector_items_hnsw") {
		return nil, plan, ErrInput
	}
	if !approx && !strings.Contains(plan, "Seq Scan") {
		return nil, plan, ErrInput
	}
```

测试的SET LOCAL用于隔离路径，不能当作生产环境必须禁用扫描/排序的建议。实际服务应保留执行计划、索引参数和数据规模，先解释选择，再比较延迟与召回。


<a id="book-ch28-vector-retrieval-真实分数怎样进入候选排序"></a>

## 真实分数怎样进入候选排序

### 真实分数怎样进入候选排序

[本地交叉编码器](local_reranker.py)使用锁定revision的ms-marco-MiniLM-L6-v2，对(query, passage)成对编码，输出一个原始logit。它只看问题和候选文本；相关性等级留在模型外。资产与运行时固定，超长输入直接拒绝，不在不知情时截断后半证据。

Go端的[ApplyCrossScores](cross_scores.go)先核对评分集合完整性：每个原候选恰有一项有限分数，重复、未知或缺失ID都拒绝，再交给Rerank。调用方仍需把这份评分绑定到原查询和已授权候选快照，不能复用另一问题的分数。

同池对照应按以下层次解释：

1. **候选是否齐全**：返回集合不增删，重排不能恢复未召回的证据。
2. **前排是否更有用**：用预先定义的等级与排名指标观察位置，不按模型分数倒推标签。
3. **关键条件是否被保留**：高分可能只是同领域内容，未必回答当前条件、版本或否定关系。
4. **新增消耗是否值得**：联合编码成本随候选数与长度变化；模型前向计时不包含加载、检索、网络和整体任务延迟。


<a id="book-ch28-vector-retrieval-重排要与实际召回排序比较"></a>

## 重排要与实际召回排序比较

### 重排要与实际召回排序比较

把候选按任意次序排成基线，重排后关键资料上升，只能说明新排序比这条次序更好。实际召回器可能早已把它放在前排，重排就没有新增收益。**先固定候选集合，再保留真实召回排序，最后比较重排后的变化。**

可以分三步判断：

1. **核对起点**：基线来自关键词、向量或融合的实际输出，记录查询、资料版本和候选预算，不用随机或刻意较差的次序代替。
2. **核对同池变化**：候选、标签与评估口径保持一致，比较关键资料的位置和RR/nDCG；池中资料相同，Recall未必改变。
3. **核对下游收益**：输入窗口内是否多保留了必要条件，答案是否更完整，再计入联合编码的时间与消耗。排序改善不能直接记作答案通过。

配套[真实向量到重排对照](#从真实向量排序到重排)出现过两种不同结论：相对固定演示次序，关键资料可以上升；相对同一候选池的实际向量排序，次序却没有改变。具体评分和原始输入留在Lab，这个结果不支持把前一种改善当作真实召回后的增益。

删除记忆的问题还暴露了另一种偏差：“向量召回相似记忆”的概念片段比明确的删除规则得分更高。词义相关可能使资料进入前排，却不足以满足“删除后不得召回”的任务条件。先检查标签、查询与候选中的规则，再检查下游是否真的采用该条件，不用更高logit裁决授权或删除状态。

这个同池比较采用小型原创任务及作者标签，没有全库ANN、生产分布或独立人工校准。次序相同不证明所有语料无需重排；新增排序收益也不证明资料真实或引用支持。候选预算、查询表示或模型变化以后重新评估，保留失败与无收益结果。


<a id="book-ch28-vector-retrieval-排名没有变不代表生成内容正确"></a>

## 排名没有变，不代表生成内容正确

### 排名没有变，不代表生成内容正确

相同八条资料、六个问题的本地对照中，五个有相关资料标签的问题，直接编码与假想文档编码的Recall@1都为1.00；第六个无资料问题在两路都返回cancel近邻，Recall未定义。这次扩展没有增加召回收益，却增加了一次生成。五条有标签问题均触及64个新增Token上限。

生成内容还暴露了查询漂移：关于操作等待时长的问题被解释成实验室或医疗设备操作；关于避免无限循环的问题生成了未经资料支持的“loop-free-loop / LFLOOP”术语。两条查询仍命中对应原始资料，**排名正确不能证明假想文本可信**，也不能证明最终答案正确。

采用前要回答：基线遗漏了哪些资料，扩展是否找回，是否把无关资料推到前排，新增费用与延迟是否值得。若扩大输出长度、改提示词或换模型，应在开发集调整，再用独立验收问题比较；不挑选成功案例，也不预设HyDE必须优于直接查询。生成失败、文本为空、超限或明显漂移时，可以按明确策略回退原始查询，并记录回退原因；原批量对照仍在生成协议失败时直接报错；下节另提供独立单问题回退策略，不改变既有结果。


<a id="book-ch28-vector-retrieval-最佳实践"></a>

## 最佳实践

## 最佳实践

### 分离建索引与在线查询

离线阶段解析、切分、向量化与发布索引版本；在线阶段向量化查询、过滤、召回与排序。不要每次问答都重新向量化全部资料。旧Eino组装实验为了展示接口采用单次构建流程，不能直接按生产查询路径照搬。

### 保留可解释的来源链

每个命中保留来源、版本与两路排名。更新时同时核对原文、向量空间和引用目录，不让同ID悄悄变成另一段内容。

### 先找遗漏层，再调参数

无命中先查范围和索引；错误命中先查原文与查询表示；目标入池但排名低才查融合和重排。top-k变大可能掩盖问题，同时增加下游噪声。


<a id="book-ch28-vector-retrieval-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 向量维度正确，排序却完全异常

核对查询与文档的Space及模型版本，再核对预处理和归一化。维度检查只能抓住形状错误，不能证明空间一致；用可解释固定向量先排除算法问题，再查真实adapter。

### 某用户检索不到资料，管理员却能命中

检查可信身份与过滤条件，再检查该版本资料是否已索引。不能为了命中移除权限过滤，也不能在日志中输出其他用户的资料正文。

### 重排没有改善结果

先检查目标资料是否已进入候选池，然后比较新旧排序，最后检查评分服务和任务标签。评估数据少或标签过时，也会产生看似无效的结论。

