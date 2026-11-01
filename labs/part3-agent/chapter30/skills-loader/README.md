# 实验：能力包发现、激活与加载预算

使用真实临时SKILL.md与references文件，发现元信息、按批准范围激活正文、按需读取带摘要的资源。默认离线，不安装Codex技能，不执行脚本或命令。

格式采用Agent Skills子集；本实验额外要求metadata.version及资源摘要，不能作为完整规范验证器。Select仅教学词重合，不是语义路由。

## 前置与入口

阅读[Skills正文](../../../../content/book/part3-agent/building/chapter28-skills.md#topic-agent-skills)、Context与Tool。准备见[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：有界YAML、目录、批准、正文缓存、参考资源与工具政策。
- [main_test.go](main_test.go)：渐进加载、重复预算、批准不扩张、版本变化、资源篡改、路径与取消。
- [comparison.go](comparison.go)：用实际Loader生成配对输入，不向模型提供评分字段。
- [compare_model.py](compare_model.py)：固定本地模型生成、实际Token与原始结果；[comparison_test.py](comparison_test.py)默认离线检查协议与评分边界。

## 预测与运行

包声明allowed-tools: run-command，宿主只批准read-doc；加载成功以后能否调用run-command？重复激活是否再扣字节预算？

```bash
go -C labs run ./part3-agent/chapter30/skills-loader
go -C labs test ./part3-agent/chapter30/skills-loader -race -count=1
go -C labs test ./part3-agent/chapter30/skills-loader -cover
go -C labs build -o /tmp/skills-loader ./part3-agent/chapter30/skills-loader
```

目录有2包，只激活incident-diagnosis；重复激活缓存不增加Used，run-command仍拒绝。正文包含诊断步骤，另一个包包含报告组织步骤。实际Used按加载的UTF8字节计，不是真实Token。

## 故障观察

- 未批准或未知名称：ErrDenied，不能因发现元信息而激活。
- 正文超出剩余预算：ErrBudget，不部分提交。
- 发现后元信息版本变更：ErrSkill；已缓存正文保持本轮快照。
- 参考文件只在已激活包内读取，路径必须登记SHA256；修改内容但不更新包：拒绝。
- SHA256先解码再规范化，大小写等价；格式错误、错摘要和越权路径仍拒绝。TestReferenceDigestUsesBytesNotHexLetterCase也验证新实例拒绝改动资源，旧实例缓存保持已加载快照。
- ../、跨目录或逃出Root的符号链接：不能读取。
- 正文与allowed-tools不会修改应用工具清单。

发现最多8包、文件前部4KiB，正文16KiB、参考8KiB，总预算最多32KiB。Loader单调用方拥有；不是并发共享服务，也不提供发布认证或热更新。

## 全量与按需输入的模型对照

先导出实际加载内容，不需要模型：

```bash
go -C labs run ./part3-agent/chapter30/skills-loader -export-inputs > /tmp/skills-comparison-inputs.json
python3 labs/part3-agent/chapter30/skills-loader/comparison_test.py
```

六个development任务覆盖诊断、报告、组合、缺少规则、不需要技能和关键词重叠。两条路径都使用相同获准目录、事实与只读工具政策；full加载两个包及其参考，selected先运行词重合选择器，再实际加载。各任务使用新Loader，预算和缓存不跨任务复用。模型输入只有任务、事实、目录与加载内容；预期选择、必要数字和缺失标记留在独立评分文件。

真实生成需要Linux/macOS上的独立Python 3.12.14环境，依赖按[generation-requirements.lock](generation-requirements.lock)安装，并提前按[model-lock.json](model-lock.json)准备固定Qwen3-0.6B资产。准备依赖/权重需要显式联网；运行脚本只访问本地资产，逐文件核对大小与SHA256，不加载远程代码。权重和运行时不入仓库。

由调用方设置三个路径，输出必须是尚不存在的新文件：

```bash
SKILL_COMPARISON_E2E=1 "$SKILL_COMPARISON_PYTHON" \
  labs/part3-agent/chapter30/skills-loader/compare_model.py \
  --assets "$SKILL_COMPARISON_ASSETS" \
  --output "$SKILL_COMPARISON_OUTPUT" < /tmp/skills-comparison-inputs.json
```

脚本最多12次生成，CPU单线程，单次输入1024 Token、输出160 Token，总进程300秒。每条完成后保存原始输出、截断标记、实际Token、加载字节、选择与本机生成耗时；中断文件保留complete=false，不能当作完成报告。生成设置为greedy且关闭thinking，是锁定教学配置，不作为通用模型配置推荐。没有工具执行、Provider账单或自动语义Judge。

shape_ok检查严格JSON结构，coverage_ok检查预先定义的数字、来源ID和缺失标记。检查失败、截断和输入超限保留；不去掉代码围栏或补造字段让结果通过。即使这些检查通过，semantic_support和task_quality仍为unreviewed：包含必要数字不能证明因果判断、报告质量或所有断言正确。selection_exact只对selected路径解释。

新任务必须预先定义评分标准，不按当前选择器输出反推预期包。missing-policy与overlap故意区分“报告缺失事实”和“生成报告”，用于观察宽泛词重合的误选。当前数据用于开发诊断，不作为独立泛化或production-like效果证据。

首轮实际输入与输出保存在[testdata](testdata/)：selected选择精确匹配4/6，两条路径均0/6通过严格JSON。full/selected正文与参考总加载字节1692/1128、输入Token总数2003/1731、输出290/291；全部未触及输出上限。原始结果带代码围栏，还遗漏事实数字或把已有字段标成缺失；不自动修复。默认Python测试仅重放评分检查，不再次加载模型，不把既有记录算作新一轮效果证据。

### 固定提示版本，再检查新任务

默认提示为skills-comparison-v1，保留原始输入组织。v2明确要求使用事实数值、只标记确实缺少的必要字段，并把事实与技能指导分别呈现；评分函数、模型资产、工具政策与生成上限保持相同。提示与输入组织同时改变，因此将它视为一个配置版本，不将改善归功于某一句提示或Skill本身。

新任务在首次生成前冻结，使用新问题、数值、来源ID、多来源事实与不同缺失字段。导出和运行：

```bash
go -C labs run ./part3-agent/chapter30/skills-loader -export-inputs \
  -comparison-set holdout-v1 > /tmp/skills-holdout-inputs.json

SKILL_COMPARISON_E2E=1 "$SKILL_COMPARISON_PYTHON" \
  labs/part3-agent/chapter30/skills-loader/compare_model.py \
  --assets "$SKILL_COMPARISON_ASSETS" --output "$SKILL_COMPARISON_OUTPUT" \
  --prompt skills-comparison-v2 < /tmp/skills-holdout-inputs.json
```

comparison-set仅接受development-v1、holdout-v1，不接受文件路径；提示也只接受两个登记版本。v2在开发任务两路径均格式6/6、字段覆盖5/6；原先缺重试政策的任务返回retry policy而非retry_policy，判定保持失败。新任务的full/selected格式均6/6、字段覆盖2/6与3/6，输入Token2308/1975、输出300/294，未触及输出上限。多个source_ids带上呈现标签Source前缀；还出现没有失败证据却声称发生超时的文本，不能靠删前缀掩盖内容问题。

三组记录保存在testdata并以任务文件SHA256关联。新任务属于相同合成任务族，第一次运行未参与结果调优；读过结果后再修改策略，就应把这组题作为回归，不能继续宣称为未见任务。字段覆盖是有限检查，格式改善与输入减少均不能替代人工语义评分或更广泛的任务收益证据。

## 迁移练习

在自己的副本中完成下面任务，保留原实验与默认离线验证。表中的新测试由学习者编写，当前回归通过不能代替迁移完成。

| 任务 | 起点与需要完成的改动 | 失败观察与完成证据 |
| --- | --- | --- |
| 1. 从字段覆盖转向任务判断 | 从comparison.go的exportComparisons和comparison-cases.json的结构起步，另建有版本的新任务文件；先冻结问题、事实与评分标准，再登记显式数据集选择，不覆盖原开发/回归记录 | 构造字段齐全但条件解释错误的回答；独立审阅应能指出具体矛盾。保存实际输入、原回答、人工依据和全部失败，字节减少与字段命中不作为语义通过 |
| 2. 处理能力包冲突 | 修改main.go的fixture，在两个包正文中加入同一场景的相反规则；在exportComparisons的激活前选择处定义优先级或拒绝冲突的策略 | 先复现两个包同时加载的矛盾输入，再自行添加冲突测试；来源/版本改变时重新判断。明确冲突处理属于宿主策略，Loader不负责理解任意正文，也不能扩展工具许可 |
| 3. 更新包与资源快照 | 以main_test.go的loader和TestReferenceContentVersionBinding为起点；先加载正文及参考，再同时更新版本、资源内容与登记摘要，分别访问旧Loader和新建Loader | 旧已加载内容保持原快照；新实例发现新版本并读取新资源；只改资源不改摘要仍拒绝。自行验证Used不因缓存命中重复增长，摘要正确不代表发行者已认证 |
| 4. 拒绝坏元信息 | 从main.go的header/New和compatibility_test.go起步，用临时目录分别加入非法YAML、重复字段、空description与目录名/name不一致 | 每一种坏输入均在发现阶段返回错误，不出现部分可用目录；添加独立用例并保留正常包对照，不能放宽校验来让坏包被加载 |
| 5. 接入新的选择器 | 保留Loader.Select作为词重合基线，在exportComparisons调用Select的位置注入另一选择方法；默认用固定提议，真实模型另按显式门禁与预算运行 | 固定提议包含未知包、未批准包及关键词重叠任务；提议不能直接绕过Load批准。记录选择、实际加载内容与任务结果；评分元数据不进入选择输入，可靠选择与任务收益分别判断 |

先确认已有基线测试存在，再运行：

```bash
go -C labs test ./part3-agent/chapter30/skills-loader -list 'TestProgressiveLoadingAndPermissionIsolation|TestBudgetVersionAndPathRejection|TestReferenceContentVersionBinding'
go -C labs test ./part3-agent/chapter30/skills-loader -run '^(TestProgressiveLoadingAndPermissionIsolation|TestBudgetVersionAndPathRejection|TestReferenceContentVersionBinding)$' -race -count=1 -v
```

为新增行为自行编写测试后，同样先用-list核对名称，再筛选执行。没有新增测试、出现no tests to run或只有原基线通过，都不能说明上表任务完成。需要真实模型或人工审阅的项，分别保留其证据状态；不以固定提议冒称模型效果。

## 来源与许可

原创代码与合成包，MIT。[Agent Skills规范](https://agentskills.io/specification)为格式参考，使用既有yaml.v3依赖。模型来源与许可见[固定Qwen3模型卡](https://huggingface.co/Qwen/Qwen3-0.6B/blob/c1899de289a04d12100db370d81485cdf75e47ca/README.md)和仓库[第三方说明](../../../../THIRD_PARTY_NOTICES.md)；调用封装没有复制上游实现。

## compatibility格式与宿主环境

[compatibility_test.go](compatibility_test.go)验证字段省略正常，提供时须字符串且非空，最多500个Unicode字符；本加载器还拒绝纯空白。501字符、数字、null和空字符串返回ErrSkill，500个汉字仍可通过（同时受4KiB frontmatter读取上限约束）。发现和Load共用header，KnownFields与原名称/描述/metadata.version检查保留。

```bash
go -C labs test ./part3-agent/chapter30/skills-loader -run '^TestCompatibilityFieldHasIndependentFormatBound$' -race -count=1 -v
```

这是格式检查，不检测实际依赖、安装软件、启用网络或允许工具。没有实现的Agent Skills标准/宿主特性仍按正文说明，不宣称完整参考验证器。原包选择/加载/真实生成记录与练习保留。

## YAML原始类型与结构解码

[scalar_types_test.go](scalar_types_test.go)覆盖name、description、license、allowed-tools及metadata键值的原始类型。数字、布尔值和null不能因为yaml.v3可转换为Go字符串而被接受；需要文本数字时明确加引号。metadata是字符串键和值的映射，本实验额外要求version，不从数字自动形成版本。

```bash
go -C labs test ./part3-agent/chapter30/skills-loader -list 'TestFrontmatterDoesNotCoerceScalarOrMetadataTypes|TestYAMLTypesAreCheckedDuringDiscoveryAndActivation'
go -C labs test ./part3-agent/chapter30/skills-loader -run '^(TestFrontmatterDoesNotCoerceScalarOrMetadataTypes|TestYAMLTypesAreCheckedDuringDiscoveryAndActivation)$' -race -count=1 -v
```

反例分别检查header与真实临时目录发现；发现后修改头部，Load仍拒绝且Used不增加。带引号的数字字符串、多行description及字符串元数据保持合法。这个修复验证格式边界，不检测包真实性、宿主环境或模型是否正确采用规则。

<a id="book-agent-skills-指导与证据怎样进入同一请求"></a>

## 指导与证据怎样进入同一请求

### 指导与证据怎样进入同一请求

Skill可以规定“先检查超时配置，再确认重试政策”，但这条流程不说明本次服务的超时值。事实来自受控证据，流程来自选定能力包，问题来自当前任务；把三者混成一段system文本，会让模型难以区别“应该检查什么”和“已经确认什么”。

[模型输入投影](compare_model.py)先组织专用视图，评分字段留在调用外：

```python
    payload = {
        'question': case['question'], 'evidence': case['evidence'],
        'catalog': [{'name': x['Name'], 'description': x['Description']} for x in mode['catalog']],
        'skills': [{'name': x['Name'], 'instructions': x['Text']} for x in mode['skills']],
        'references': mode['resources'],
    }
```

目录说明可用能力，正文提供流程，参考文件补充规则，evidence提供本次事实。投影以后还要检查消息怎样编码；不同字段放进同一个JSON不保证模型正确理解其角色，应用指令应说明资料与指导的区别，权限仍由执行器决定。

标识也要与呈现标签分开。来源ID是policy-4时，“Source policy-4:”中的Source只是排版，不属于ID；让模型把整段标题复制到source_ids，引用就无法匹配可信目录。输入应清楚标示ID与原文，输出仍按实际ID校验，不能在交付时随意删前缀或猜来源来制造通过。


<a id="book-agent-skills-协议通过以后继续核对事实使用"></a>

## 协议通过以后，继续核对事实使用

### 协议通过以后，继续核对事实使用

模型可能返回围栏包裹的JSON，也可能把“已配置30秒超时”写成“已经发生超时”。前者是应用协议失败，后者是把配置值误当事件。二者需要不同修复，增加“只输出JSON”无法解决事实误用。

| 现象 | 所在判断层 | 应检查什么 |
| --- | --- | --- |
| JSON外有围栏或缺字段 | 输出协议 | 生成是否完整、格式约定与严格解析；修复尝试单独记录 |
| 正文已加载，但缺少重试规则 | 选择与资源供给 | 是否选对包，必要参考是否读取并进入本轮输入 |
| 将呈现标签复制成来源ID | 引用身份 | 稳定ID与文本是否易于区分，返回ID是否属于本次证据 |
| 将配置值写成已发生的故障 | 语义支持 | 资料实际确认了什么，结论是否增加了不存在的事件或因果 |
| 给出正确数字，却漏掉适用条件 | 任务完整性 | 用户要求、单位、例外与版本是否保留 |

排查按实际链路推进：**先核对选择和加载，再核对真正发出的输入，随后检查协议与引用，最后判断语义和任务完成。** 每一步保留原始失败，不能通过格式清洗让下游看不到错误，也不能因为必要数字都出现了，就默认断言有依据。

配套[真实生成对照](#固定提示版本再检查新任务)已经出现过上述围栏、来源标签和配置误用。提示与输入组织的修订改善了格式，仍未完成人工语义核对；工具也没有在该对照中实际执行。具体模型、任务划分、输入消耗和全部原始结果保存在Lab，不由字段覆盖推导完整能力收益。


<a id="book-agent-skills-最佳实践"></a>

## 最佳实践

## 最佳实践

### 一个包有清楚的适用范围

用任务和产出组织指令，保留证据与不足处理。稳定规则、临时参数和运行结果不混写进同一个长期文件。

### 版本变更可追查

记录发布内容、加载Digest、元信息与兼容条件。更换流程或引用资源时更新版本，不用相同名字掩盖行为变化。

### 资源加载不自动递归

明确本轮需要的文件，限制数量、大小与来源。循环引用和无限展开会重新造成上下文膨胀，甚至绕过批准范围。


<a id="book-agent-skills-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 能力目录存在，但模型没用上

先检查描述和选择，再确认正文实际进入输入，最后看必要参考是否加载。目录发现不是激活，也不是模型正确使用的证据。

### 重复加载让费用或输入持续增长

检查缓存键与运行版本，确认重复内容没有重复加入Context。字节缓存不能替代真实usage核对，状态丢失或新实例也会重新加载。

### 加载后请求越权工具

检查应用批准清单，确认allowed-tools和正文没有改写执行政策。直接测试拒绝，再看模型为何选择该动作，不先开放工具来满足指令。

### 选择正确，但回答没有采用规则

先核对本轮实际模型输入，确认正文和必要参考已经进入Context，再检查冲突规则、输出截断和实际动作。目录命中、Load返回成功、输入中存在规则、模型正确使用规则是四个不同观察；只改description无法修复后续阶段的问题。

