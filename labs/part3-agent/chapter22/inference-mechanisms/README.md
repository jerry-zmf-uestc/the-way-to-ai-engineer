# 推理机制：字节分词、因果Attention、KV与采样

以真实数值运算解释接口背后的机制。标准库、固定向量与规则，不下载权重、不调用Provider。前置：Go slice/数学函数，理解向量点积；关联[Model正文](../../../../book/part3-agent/building/chapter22-architecture.md#topic-ch22-llm-api)。

## 这几个小实验各证明什么

- Tokenizer从UTF8字节开始，按已给定pair rank合并，Decode拼回原始字节。规则没有训练，词表不是任何Provider的真实tokenizer；不实现预切分、特殊Token、聊天模板或计费。
- Attention使用独立的四ID词表、二维固定embedding/线性投影和位置夹具；它不是Tokenizer输出接入训练模型，也不是完整Transformer（没有多头、FFN、残差、LayerNorm或LM头）。
- FullCausal每一行只读当前位置及以前的K/V。KVCache每次追加一个Token，只生成新增投影，并拒绝编辑历史或更换缓存身份。数值等价在1e-12容差内；Projections/ScorePairs是运算计数，不能当实际耗时或GPU benchmark。
- 采样从给定logits按temperature计算softmax、top-p保留并归一化，再用显式u选样；不包含模型预测质量。temperature=0采用稳定argmax，只证明这个固定分布的greedy规则。
- 输入有界：文本4096字节、64条merge、Attention/缓存64个教学ID、分布256项；检查NaN/Inf/类型范围。单调用方拥有缓存，未实现线程共享或跨服务缓存。

## 预测、运行与观察

从仓库根目录执行，准备见[运行约定](../../../../book/introduction.md#running-examples)：

```bash
go -C labs run ./part3-agent/chapter22/inference-mechanisms
```

输出：

```text
bytes=11 runes=7 toy_tokens=9 roundtrip=true
cache_equivalent=true full_projections=10 cached_projections=4 cached_score_pairs=10
top_p_probabilities=[0.731 0.269 0.000] fixed_draw=1
```

先预测：修改词表合并优先级，会不会改变Token？修改缓存的第一个历史ID以后，是否可以只追加最后一个ID？提高temperature是不是意味着答案正确率提高？

runes统计Unicode码点，不是组合字符的视觉长度。第一行使用“AI学习 AI”：AI合并，其余UTF8字节仍作为回退Token；三个长度不能相互替代。第二行对每个前缀重算与缓存递增对照；缓存保留旧K/V，但新Q仍读所有历史K。第三行logits=[2,1,0]、temperature=1、top-p=.8，保留前两项；u=.9选ID1，没有现实语言或正确性结论。

```bash
go -C labs test ./part3-agent/chapter22/inference-mechanisms -race -count=1 -v
```

应全部PASS：字节往返、中文/emoji、pair优先级、非法UTF8；未来ID改变不影响已有因果行；缓存与全计算等价、历史/身份变化拒绝且不改变状态；分布归一化、温度/保留集合、固定draw与非有限数值拒绝。

## 练习

| 任务 | 起点与提示 | 边界与验证 |
| --- | --- | --- |
| 增加merge规则 | NewTokenizer规则表，先确认只能引用已有ID | Encode/Decode仍往返；同文本Token可变，不能将结果冒称真实模型计数 |
| 改变历史或位置 | Cache.Step的identity与prefix | 必须拒绝或明确重算，不复用旧K/V；拒绝不污染缓存 |
| 检查因果遮罩 | 在副本让较早Q读取全部K | 改未来ID使过去输出变化，测试应失败；恢复只读前缀后复验 |
| 比较采样 | 给同一logits固定u，对照temperature和top-p | 概率归一，保留集合符合规则；不从概率推断正确率 |
| 测量真实延迟 | 在受控Provider实验分离首输出、每步、总时长和usage | 锁定模型/输入/并发，记录缓存命中证据；不能用这里的投影次数宣称实际提速 |

## 来源与许可

原创实现、固定数值与图示，MIT。机制依据：[Attention论文](https://arxiv.org/abs/1706.03762)、[Hugging Face BPE课程](https://huggingface.co/learn/llm-course/en/chapter6/5)、[KV Cache解释](https://huggingface.co/docs/transformers/en/cache_explanation)、[生成策略](https://huggingface.co/docs/transformers/en/generation_strategies)。未复制实现、图片或训练权重。

## 偏好目标数值，不训练模型

preference.go以完整响应的策略/参考对数概率计算单样本DPO损失，beta是训练目标参数。原CLI另外打印偏好概率0.2/0.5/0.8时损失1.609438/0.693147/0.223144；参考概率固定0.5，beta=1。

```bash
go -C labs test ./part3-agent/chapter22/inference-mechanisms -run '^TestPreference' -race -count=1 -v
```

测试检查参考相同时log(2)、极端差值稳定性、非有限/正对数概率与非法beta拒绝。函数只算目标，不验证概率是否来自同一提示/分词器/模型；不做梯度、SFT、奖励模型或RL更新，也不报告事实正确率或微调效果。对应模型概率应来自实际完整响应计分，不能任意换成逐Token平均。

练习：策略已经偏好一个响应时更换参考概率，解释损失变化；再构造“人更喜欢但事实错误”的候选，说明为什么偏好目标与任务正确性必须分别评估。不要用降低损失代替真实模型训练与验收。

机制参考[InstructGPT](https://arxiv.org/abs/2203.02155)与[DPO目标](https://arxiv.org/abs/2305.18290v3)，公式实现为原创，不复制上游训练代码。

<a id="book-ch22-llm-api-后训练怎样影响接口行为"></a>

## 后训练怎样影响接口行为

### 后训练怎样影响接口行为

模型能续写文本，仍可能不按要求回答、不遵守JSON形状，或者对错误事实表现得很肯定。训练阶段的目标不同，形成的倾向也不同；应用需要知道这些倾向从哪里来，才能选择模型与评估任务。

| 阶段或路线 | 用什么信号优化 | 对工程选择的影响 |
| --- | --- | --- |
| 预训练 | 语料上的Token预测 | 学到语言与知识模式，调用仍需给足上下文与任务说明 |
| SFT | 指令与示范响应 | 学习示范中的回答方式和格式，效果依赖数据与部署任务的关系 |
| 典型奖励模型RLHF | 偏好排序训练奖励模型，再用RL调整策略并约束偏离参考模型 | 优化的是偏好反馈，奖励分数仍需独立任务验证 |
| DPO | 同一提示的偏好响应对，加上策略与参考模型对数概率 | 直接优化偏好目标，数据身份和评分口径必须一致 |

![预训练与SFT之后，奖励模型RLHF和DPO采用不同优化路径；应用仍独立检查协议、证据、权限与预算](../../../../public/diagrams/model-posttraining.svg)

SFT通常在示范响应Token上计算负对数似然，数据格式与损失mask决定哪些Token参与学习。[InstructGPT论文](https://arxiv.org/abs/2203.02155)展示了示范微调、偏好排序与奖励模型强化学习的具体路线；RLHF还有其他实现，不能把这一流程当成所有后训练的定义。

[DPO](https://arxiv.org/abs/2305.18290v3)对同一提示x的偏好响应y+与非偏好响应y-，比较它们相对于参考模型的对数概率变化。令m为这个差，单样本损失为`-log(sigmoid(beta*m))`。它不要求为这一目标另训独立奖励模型，也不需要每次优化时在线重新采样响应；偏好数据仍需事先准备。

[PreferenceLoss](preference.go)只计算这个目标值：

```go
func PreferenceLoss(policy, reference PairLogProb, beta float64) (float64, error) {
	if !finite(beta) || beta <= 0 || beta > 1e6 {
		return 0, ErrInput
	}
	for _, value := range []float64{policy.Chosen, policy.Rejected, reference.Chosen, reference.Rejected} {
		if !finite(value) || value > 0 || value < -1e6 {
			return 0, ErrInput
		}
	}
	margin := (policy.Chosen - reference.Chosen) - (policy.Rejected - reference.Rejected)
	z := beta * margin
	// Stable -log(sigmoid(z)), including large positive and negative margins.
	if z >= 0 {
		return math.Log1p(math.Exp(-z)), nil
	}
	return -z + math.Log1p(math.Exp(z)), nil
}
```

Chosen与Rejected是完整响应的对数概率，policy与reference必须对应同一提示、响应与计分约定。m先分别减去参考值，再比较两条响应；beta是训练目标参数，与推理temperature作用不同。稳定写法避免大幅度正负差值导致指数溢出，函数只检查数值范围，不能验证概率来自什么模型与数据。

两候选教学例中，参考概率均为0.5；策略偏好响应概率从0.2变为0.8时，损失下降。若策略与参考已经相同，损失仍为log(2)，说明这个目标关注相对变化。实验没有梯度更新或训练权重，损失数值也不证明回答真实、任务完成或模型能力提升。

示范与偏好反馈会影响格式、回答风格和拒绝倾向。部署后仍要检查截断、拒绝、结构合法性与独立任务指标，数据分布改变时重新评估。工具权限继续由应用决定，来源与事实支持由RAG/Eval检查；后训练不会授予执行权限，也不会把模型的confidence变成校准后的正确概率。

前面讨论的是模型能力为什么会呈现出当前的输出倾向。接下来再把视角收回到工程接入：无论底层模型怎样训练，应用仍然只能通过 Provider 契约接住一次调用的输入、输出和失败。

