# 实验：stdio握手、发现与受控调用

实际启动本课程程序的本地Server子进程，发送换行分隔JSON-RPC：initialize、initialized通知、tools/list、tools/call。Server仅提供固定read_doc，Client保留独立批准清单。

协议锁定2025-11-25的教学子集，另支持固定资源与命名提示模板。默认离线，不启动外部任意程序、不加载模型；下方本地模型对照需显式启用。不实现HTTP、OAuth、资源模板、二进制内容、分页、通知流或并发请求。

固定Loop与本地模型路由共用真实stdio Server，但结论分别记录：固定模式验证结果回填和执行契约；真实模型已生成动作，重复ID/重复来源导致比较任务失败。应用绑定ID的变体修正标识职责，仍未证明可靠选择或任务完成。旧生成记录的默认重放不算新一轮模型调用。

## 前置与入口

阅读[MCP正文](../../../../content/book/part3-agent/building/chapter23-actions.md#topic-agent-mcp)、Tool与context。准备见[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：stdio Server、Client、握手、Schema子集、本地namespace和失败分类。
- [agent.go](agent.go)：有界决策、批准工具调用与按Call ID回填观察。
- [context.go](context.go)：Resources/Prompts发现、独立批准、读取与消息边界。
- [context_test.go](context_test.go)：真实stdio交互及URI、角色、类型、分页与权限反例。
- [context_agent.go](context_agent.go)：可信选择、资料快照、完整输入视图预算与原Loop适配。
- [context_agent_test.go](context_agent_test.go)：真实资料进入决策、副本隔离、未选/未批准、每轮预算、坏模板与工具权限不扩张。
- [router.go](router.go)：可信别名到独立Client的路由快照与双进程演示。
- [router_test.go](router_test.go)：同名工具的真实目标、独立批准与断连不回退。
- [agent_test.go](agent_test.go)：真实stdio结果进入下一轮、失败修复、拒绝不发送RPC、取消及预算。
- [routed_agent_test.go](routed_agent_test.go)：两个真实Server进入同一Loop，来源/调用ID、共享配额、独立批准、取消和失败不回退。
- [main_test.go](main_test.go)：真实子进程通信、发现不授权、参数拒绝、阻塞读取/写入取消、响应ID不匹配。

## 预测与运行

Server发现了read_doc，但本地approved为空；模型是否可以调用它？工具请求合法但文档不存在，属于RPC错误还是工具错误？

```bash
go -C labs run ./part3-agent/chapter30/mcp-stdio
go -C labs test ./part3-agent/chapter30/mcp-stdio -race -count=1
go -C labs test ./part3-agent/chapter30/mcp-stdio -cover
go -C labs build -o /tmp/mcp-stdio ./part3-agent/chapter30/mcp-stdio
```

输出namespace=docs::read_doc，text="Timeout is 30 seconds."。子进程是当前已构建课程程序，-server模式stdout只写协议，关闭输入后退出。docs::是本地路由约定，不是MCP标准命名格式。

## 协议与失败观察

- 未握手发现：拒绝；Client核对版本与tools能力后发initialized。
- approved为空：发现不暴露工具，调用ErrDenied，发送序号不增加。
- 非法id类型或额外字段：本地拒绝；Server也检查同一约定。
- doc ID合法但不存在：Server返回isError，Client返回ErrTool，不作为资料。
- 响应ID错误、非支持结果、超大消息：协议失败，不接受内容。
- 当前消息子集拒绝Response带method/params，以及Request/Notification带result/error；TestMixedResponseEnvelopeClosesSession与TestServerRejectsResponseFieldsInRequest覆盖保留字段混用，Client失败关闭，不接纳内容。
- 写入或读取阻塞时取消：关闭会话并等待I/O退出；不证明Server动作已回滚。

Client单调用方拥有，一次只允许一个未完成请求；响应64 KiB、请求16 KiB、文本4 KiB、发现最多8工具、总请求最多32次。完整Schema和传输实现应使用合适SDK并按锁定版本验证；当前子集只接对象参数id字符串。

## 正文机制对应的运行观察

以下记录帮助核对正文的输入交接、失败回填与多Server路由；协议行为、固定决策和真实模型效果分别判断。

- **输入交接**：真实stdio资源和模板进入固定决策回调，工具提议、结果关联与终止沿用RunLoop。未批准选择、坏模板、超限与迟到取消分别验证；模板获准不扩大工具批准。该对照验证应用输入链路与执行边界，真实LLM是否使用资料、模板适用性与任务完成另行评估。
- **失败回填**：正常固定决策先读取doc-A，实际Server返回正文，第二轮从观察构造Final。缺失ID用例先得到tool_failed，再改用合法ID；两次Call ID不同，历史保留失败。上限为四轮、两次工具尝试；重复ID或Final与工具提议同时出现时拒绝。
- **多Server路由**：两个实际stdio进程分别返回30秒与45秒，固定回调从观察构造带来源的文本；权限拒绝、共享配额、取消与断连核对两边请求变化。该对照使用单调用方和固定决策，不证明多Agent协作或真实LLM路由效果；完整SDK与远程身份另行验证。

## 决策循环集成

```bash
go -C labs run ./part3-agent/chapter30/mcp-stdio -agent
```

输出：

```text
agent_rounds=2 tool_calls=1 tool_failures=0
namespace=docs::read_doc text="Timeout is 30 seconds."
```

这是实际Server进程返回的文本进入固定决策的下一轮，不是预写最终文本。工具观察有Failed与Call ID；缺失文档先失败再换参数，本地拒绝不发送RPC。Loop最多4轮/2工具，普通工具失败不被Final抹去，协议/传输失败直接停止。没有真实模型，不能把固定决策恢复当作LLM质量提升。

## 资源与提示模板的独立准入

先预测：批准read_doc是否会同时批准文档URI和explain_doc模板？读取资源是否增加Loop的tool观察？

```bash
go -C labs run ./part3-agent/chapter30/mcp-stdio -context
```

输出包含resources=1、prompts=1、prompt_messages=1、automatic_model_injection=false，以及resource_uri=course://docs/doc-A和固定文档文本。它验证真实Server子进程的resources/list、resources/read、prompts/list与prompts/get；没有把资源或消息自动送入模型。-context与-agent互斥。

资源和模板批准清单在构造时复制，与工具政策分开。未批准URI/模板本地拒绝，RPC序号不增加；能力未声明则发现失败。Read要求同URI、text/plain、单条非空文本且不超过4 KiB；GetPrompt仅接受user/assistant文本消息，最多8条、总文本4 KiB。重复键、大小写别名冲突、尾随JSON、非UTF-8、超深嵌套、不支持字段和部分分页列表不被接纳。Catalog同时发现两类能力，是本实验约定。

资源不存在返回RPC -32002，坏模板参数返回-32602；这与tools/call的isError不同。固定Server不访问任意路径或URL，course://是教学标识，不是协议规定。默认测试检查错URI、二进制/错MIME、外部system消息、分页和独立授权；这些契约不证明真实用户身份、模型选择或任务效果。

## 应用选择后接入决策输入

```bash
go -C labs run ./part3-agent/chapter30/mcp-stdio -context-agent
```

输出：

```text
context_requests=2 agent_rounds=2 tool_calls=1
namespace=docs::read_doc text="Timeout is 30 seconds."
```

Host显式选择资源与模板，检查独立批准后实际发送resources/read和prompts/get；真实内容进入MCPContext，原Loop回填实际工具观察。决策是固定Go回调，没有真实LLM调用；Server/URI/PromptName只定位来源，模板角色作为资料，不变成system权限。

模型回调每轮得到新的切片副本。JSON序列化Context与History后检查MaxBytes，上限16 KiB，示例选择8 KiB；这个计数不含真实Provider的工具定义、可信指令和封装，不作为准确Token。第二轮观察可能触发超限，测试确认该轮决策回调没有执行。Rounds计Loop进入的决策边界，预算拒绝也可能占一轮，不能把它当作真实Provider请求次数。

ContextRequests记录准备阶段的请求序号增量，不包含此前初始化/发现；ToolCalls仍是进入工具执行边界的提议次数，本地拒绝可能不发送RPC。正常例子分别为2和1；工具未获准时准备仍为2、工具提议为1，但不会额外发送tools/call。

获准却未选择的资料不读取；一项选择未获准时整份选择在RPC前拒绝。资源读取后模板非法、输入超限或取消，不释放部分输入/Final，但已发生的读取仍保留请求事实。真实Provider、模板语义与答案支持另外评估。所有示例模式互斥，旧-context仍仅展示独立准入。

## 两个Server提供同名工具

```bash
go -C labs run ./part3-agent/chapter30/mcp-stdio -multi-server
```

输出：

```text
namespace=archive::read_doc text="Archive timeout is 45 seconds."
namespace=primary::read_doc text="Timeout is 30 seconds."
```

Host仅启动本课程程序的primary/archive两个固定profile，分别初始化与发现；相同远端工具名和doc-A在各自Server中返回不同固定内容。名称排序只为稳定演示输出，不是模型选择策略。Router最多四个Client/32条路由，单调用方串行使用，返回名称副本；它不拥有连接，CLI关闭并等待两个进程退出。-multi-server与-agent、-context互斥。

测试另外将archive批准清单设为空：工具仍被该Client发现，但不会暴露到Router，调用被本地拒绝，两个Client的RPC序号都不增加。裸名字、未知Server和错误完整名称同样拒绝。两目标获准时，文本与请求序号共同验证实际目标；archive工具错误不回退primary，关闭archive输入造成真实传输失败后，不相关的primary仍可读取。重复别名拒绝，重连与动态能力更新需重新审核快照；不验证远程身份、并发请求或真实模型路由。

## 同一决策循环调用两个Server

```bash
go -C labs run ./part3-agent/chapter30/mcp-stdio -routed-agent
```

输出：

```text
agent_rounds=3 tool_calls=2 tool_failures=0
answer="archive::read_doc: Archive timeout is 45 seconds. | primary::read_doc: Timeout is 30 seconds."
```

这是一个固定决策循环使用两个Server，四轮/两次工具预算共同累计，不是两个Agent。客户端仍按各自政策校验；第二个Server不会继承第一个的工具权限。两个实际响应分别进入带完整Tool名和Call ID的观察，最终文本从观察构造，不预写30/45两个数值。

RunLoop与RunRoutedLoop只在执行函数上绑定不同目标，复用runLoop的全部语义。业务错误保留失败来源，传输错误直接结束，不自动换Server；测试检查primary未被隐式调用。第三次工具提议即使换Server仍被共享预算拒绝，取消后的提议不发往任何一个Server。

每个Client还有独立的请求上限，初始化/发现也产生实际请求，不能将Loop的ToolCalls作为所有MCP流量。Host关闭两个Client并等待自己启动的进程；路由器不拥有进程。-routed-agent与其他示例模式互斥，旧-multi-server继续只展示直接路由。固定控制不证明真实模型选对来源、比较任务完整性或协作收益。

## 真实本地模型提出路由动作

[model_decision.py](model_decision.py)只接收批准工具全名及当前History，不见期望30/45标签；工具仍由Go Host调用实际stdio Server。[model_agent.go](model_agent.go)保留原始JSON与实际Token，拒绝截断，不删围栏、补参数或猜目标；JSON解码完成以后，RunRoutedLoop继续检查分支、Call ID和预算。

按[model-lock.json](model-lock.json)准备固定Qwen3-0.6B资产，按[generation-requirements.lock](generation-requirements.lock)准备独立Python 3.12.14运行时。设置MCP_MODEL_PYTHON、MCP_MODEL_ASSETS及MCP_MODEL_SCRIPT为本机绝对路径；脚本使用本Lab的model_decision.py，属于可信应用配置，不由模型输出决定。

```bash
MCP_MODEL_E2E=1 go -C labs run ./part3-agent/chapter30/mcp-stdio -routed-model > /tmp/mcp-routing-result.json
```

仅CPU单线程、greedy/nonthinking，单次输入1024/输出192，总Run180秒，最多四轮/两工具。简单实现每轮重新加载资产，启动开销不算算法优势；报告保留失败尝试的用量。受控Python stdout最多32 KiB，模型响应与候选继续受严格JSON/16 KiB检查。进程失败时usage未取得，不能解释为零费用。

[首轮原始记录](testdata/model-routing-v1.json)实际执行两次模型生成：输入166+255=421、输出24+24=48，均未触及输出上限。模型先选择archive::read_doc，实际读取45秒；第二轮重复同一工具和字面Call ID“a unique call id”。循环在第二次工具执行前返回协议失败，primary没有被读取。记录保留三个历史消息和两条原始候选；Error=run_failed、CoveragePass=false，complete只表示本次实验结束，CLI失败退出不能当作通过。

两条Stage=decoded只说明JSON被接收，重复ID仍在Loop层被拒绝。有限覆盖要求无运行错误、两来源的实际观察及最终文本包含30/45；它不检查单位、归属是否交换或任意释义，SemanticQuality仍为unreviewed。本例没有完成双来源比较或证明可靠路由。

默认Go测试重放这两条原始生成，实际启动两个课程Server，确认仍只读取archive一次、primary未收到工具调用；不重新运行模型。继续修改提示或标识契约时另存新版本，不能改写原始失败或默认扩大工具预算。

### v2：应用绑定关联ID

v1保持默认。显式MCP_MODEL_CONTRACT=v2时，模型仅输出Call.Tool/DocumentID或Final；应用先分配本轮model-N，在合法工具提议解析后绑定。原始响应不重写，BoundCallID另存；非空模型ID、混合Final/Call与缺字段拒绝。绑定不修改来源或参数，也不是MCP JSON-RPC请求ID。

```bash
MCP_MODEL_E2E=1 MCP_MODEL_CONTRACT=v2 go -C labs run ./part3-agent/chapter30/mcp-stdio -routed-model > /tmp/mcp-routing-v2.json
```

[v2原始记录](testdata/model-routing-v2.json)使用同任务、资产与预算：三次实际生成，输入178+265+352=795、输出18+18+17=53，无截断。前三条候选均选择archive，分别绑定model-1/2/3；前两次实际读取都返回45秒，第三次在执行前触发两工具预算。primary仍未读取，没有Final，CoveragePass=false，CLI失败退出。

标识关联已经成立，信息获取策略仍失败。增加合法ID只让重复查询消耗了额外配额，不能计为任务质量提升。两版同时存在提示和接口差异，不推导单因素收益；后续应检查重复动作、实际观察与任务缺口，另用独立任务评估。默认重放确认archive两次、primary零次以及第三次不发送请求，保留原v1重复ID诊断。

## 固定结构化结果与模型观察

```bash
go -C labs run ./part3-agent/chapter30/mcp-stdio -structured
go -C labs test ./part3-agent/chapter30/mcp-stdio -list '^TestStructured'
go -C labs test ./part3-agent/chapter30/mcp-stdio -run '^TestStructured' -race -count=1 -v
```

输出namespace=docs::read_doc，text为id=doc-A、timeout_seconds=30、unit=seconds、version=4的JSON对象。它由同一个程序的structured固定Server profile实际返回，经发现Schema/结构化对象/文本JSON/本次身份和版本检查后投影，不调用模型。-structured与其他演示模式互斥，旧primary/archive输出不变。

[structured.go](structured.go)使用固定四字段输出Schema：id/unit/version字符串，timeout_seconds整数；所有字段必须出现且大小写准确，不接受额外字段。业务另限制1–3600秒及seconds单位，资料版本由可信调用方指定。结果体8KiB，两份对象各4KiB，重复字段、非法UTF8、类型/ID/版本错误、双通道矛盾或工具isError不返回候选。无Schema、未批准或取消在发送前拒绝，错误不回退旧文本路径。

当前子集不支持通用JSON Schema，声明enum/minimum等扩展也拒绝；Go int词法解码拒绝30.0或指数写法，不宣称与所有JSON Schema整数实现兼容。Observation只是字段投影，不自动进入模型输入，不证明资料真实或最终答案支持。

[structured_test.go](structured_test.go)包含真实stdio初始化/发现/成功/工具失败/错版本，以及拒绝不增加RPC序号；codec反例独立检查结构、范围、身份、重复/额外/大小写字段和两份数值冲突。先测试确定性边界，再接Context/Loop及真实任务评估，默认不加载模型或远端服务。

## 迁移练习

在独立副本中选择与当前学习阶段对应的任务。分页、通知和HTTP属于进阶实现，先写清协议与连接设计，再修改代码；不要让一次接口新增破坏原stdio路径。下面只给出起点和验收条件，不预先实现答案。

| 任务 | 起点与需要完成的改动 | 失败观察与完成证据 |
| --- | --- | --- |
| 1. 替换固定决策 | 从agent.go的Decision及model_agent.go的适配边界起步，为另一Provider编写受限适配；默认测试用固定原响应，真实调用显式启用 | 自行加入重复Call ID、混合Final/Call、取消及截断反例；坏提议不发送RPC。保存原响应、转换后提议、实际观察与消耗，不导入其他Lab私有实现，权限仍由Client控制 |
| 2. 对照不同Server契约 | 从router.go的NewToolRouter和main.go的serveProfile起步，在副本给两个Server设置不同Schema或资料版本 | 一份参数不能因另一Server接受就被当前Server接受；记录具体目标与调用计数。重新连接时创建新Client并重新发现、批准、建路由，旧快照不能自动继承新的能力 |
| 3. 增加分页和清单更新 | 从Client.Discover与context.go的Catalog起步，先定义页数/条数上限、游标循环拒绝及完整清单发布，再设计listChanged的重新发现流程 | 自行复现第二页失败、重复项、循环游标与能力撤销；未完成列表不作为完整快照，新增发现不自动批准。原nextCursor拒绝测试保留为旧子集回归 |
| 4. 设计HTTP传输 | 先根据参考传输规范写请求/响应、版本头、Origin、身份和会话生命周期，再为同一能力增加独立transport实现；stdio的exchange保留 | 给出坏版本、坏Origin、身份缺失、会话失效与断流测试；每项定位拒绝层。普通JSON端点或本机连通只能证明相应接口，不算完整MCP或远程认证通过 |
| 5. 重新协商连接 | 从Client.Initialize及main_test.go的processForServer起步，自行构造错误版本与错误能力响应，再用新Client连接正常Server | 错协商不能进入ready或调用；旧Client关闭后不复用旧发现。新连接须重新握手、发现与批准，确认失败连接的进程和I/O得到清理 |
| 6. 扩展资源选择 | 从context_agent.go的prepareContext、ContextSelection与RunContextLoop起步，改变选择规则、资料长度或来源组合 | 在新增用例中分别验证获准但未选择、未获准、模板提及未批准工具及第二轮超预算；核对RPC计数和决策回调次数，失败不发布部分输入，准备请求不误算作工具调用 |
| 7. 增加第二种结构化结果 | 从structured.go和structured_test.go的四字段合同起步，加入带资料版本的重试次数；同步Schema适配、字段/单位/版本校验和Context投影 | 自行编写缺字段、错单位、文本/结构化冲突及旧版本反例；未知Schema仍拒绝。核对原返回、本轮实际输入和答案支持，原固定测试只能证明旧契约没有退化 |

先核对现有协议和Context基线，再执行：

```bash
go -C labs test ./part3-agent/chapter30/mcp-stdio -list 'TestActualStdioHandshakeDiscoveryCallAndToolFailure|TestDiscoveryDoesNotAuthorize|TestContextBudgetRechecksHistoryBeforeNextDecision'
go -C labs test ./part3-agent/chapter30/mcp-stdio -run '^(TestActualStdioHandshakeDiscoveryCallAndToolFailure|TestDiscoveryDoesNotAuthorize|TestContextBudgetRechecksHistoryBeforeNextDecision)$' -race -count=1 -v
```

迁移实现需要学习者新增针对新契约的测试，并先用-list确认筛选名称存在。no tests to run、全跳过或只运行原测试均不算新任务完成。固定协议、真实Provider效果和真人故障解释分别验收；没有做HTTP或远程身份实验时，不写成已支持。

## 来源与许可

原创代码，MIT。参考[MCP生命周期](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle)、[Tools](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)、[Resources](https://modelcontextprotocol.io/specification/2025-11-25/server/resources)、[Prompts](https://modelcontextprotocol.io/specification/2025-11-25/server/prompts)、[传输](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)；未复制第三方SDK。

<a id="book-agent-mcp-host怎样把资料交给决策边界"></a>

## Host怎样把资料交给决策边界

### Host怎样把资料交给决策边界

发现完成以后，Host仍有三个选择：不加载、读取后保留在应用侧，或选入本轮输入。批准说明允许获取，选择说明当前任务需要；二者不能合并成“发现后全部注入”。

[输入适配](context_agent.go)保留连接别名、资源URI与模板名称，将外部角色标签作为资料字段：

```go
type MCPContext struct {
	Server     string
	Resources  []ResourceText
	PromptName string
	Template   []PromptMessage
}
```

Template中的user或assistant表示Server返回的模板内容，不被这份适配直接提升为应用消息角色。可信system规则仍由Host单独管理。Server是绑定本地Client的别名，不是远程认证凭据；实际模型adapter还需选择合适的消息格式，保留资料来源与信任边界。

接入按以下顺序推进：

1. **检查整份选择**：URI和模板各自已获批准且已发现；未选择的项不读取，批准工具不会顺带批准资料。
2. **取得并核对内容**：Read检查URI/MIME与大小，GetPrompt检查角色与文本；任一失败，整份输入不进入决策回调。已经完成的读取仍计入请求事实。
3. **构建本轮视图**：为资源、模板和消息历史分别制作副本；模型侧修改回调参数不能改变后续轮的资料快照。
4. **检查完整输入容量**：包括新增的提议与观察，超过边界时停止，不继续调用决策回调。
5. **推进原Loop**：模型提出工具调用后，仍由原Client检查批准范围并回填观察；资源资料不会被伪造为一次tool调用的结果。

每轮的大小检查紧接模型边界：

```go
		payload, err := json.Marshal(struct {
			Context  MCPContext
			Messages []LoopMessage
		}{view, history})
		if err != nil || len(payload) > s.MaxBytes {
			return LoopReply{}, ErrContextBudget
		}
		return decide(ctx, view, history)
```

这是已序列化可见视图的字节上限，不是Token窗口或完整Provider请求计数；工具定义、可信指令和Provider封装仍须在真实adapter中合计。第一次能放下，下一轮新增工具观察后可能超限，因此不能只在准备资料时检查一次。

三类批准控制的是操作入口，不自动形成底层资料的统一ACL。模板也可能包含文档正文；真实服务需根据调用身份和参数核对资料权限，不能只看模板名字是否获准。当前固定公开文档实验不提供这种远端身份与文档授权。

输入交接的检查到这里才结束：资料和模板满足准入条件，整份视图通过容量检查，决策回调取得独立快照。后续工具提议仍经过原执行边界；模板中出现工具名称，也不会扩大批准范围。输入正确交付之后，还需检查模型是否使用了资料、模板是否适用，以及回答是否完成任务。

前面分别讲了 Tools、Resources 和 Prompts 各自怎么进来；这里再把三条路径并在同一个 Host 决策闭环里看一遍。


<a id="book-agent-mcp-工具输出schema与模型可见内容分开"></a>

## 工具输出Schema与模型可见内容分开

## 工具输出Schema与模型可见内容分开

Server返回“请求超时30秒”，应用可能还需要timeout、unit和资料版本来做比较。仅解析文本会把业务逻辑绑定到措辞；结构化字段方便程序处理，但模型是否看见这些字段，仍取决于Host的适配。

MCP 2025-11-25将工具结果的普通内容放在content，结构化JSON对象放在structuredContent。工具还可以声明outputSchema；Server必须按所声明Schema返回结构化结果，Client应校验它。[Tools规范](https://modelcontextprotocol.io/specification/2025-11-25/server/tools#structured-content)同时明确：这份数据由Server产生，与LLM按Schema生成的“结构化输出”是两回事。

| 契约 | 谁提供、谁检查 | 不能证明什么 |
| --- | --- | --- |
| inputSchema | Server描述参数，Host及Server检查请求 | 参数合法不代表调用获准 |
| outputSchema | Server描述结构化结果，Client检查实际字段 | 字段合法不代表业务事实正确、资料最新 |
| structuredContent | Server返回JSON对象，应用保留类型与来源 | 收到结构化字段不代表模型已经使用它 |
| content | Server返回文本等内容，adapter转换模型观察 | 文本可读不代表完整、与结构化字段一致或支持最终答案 |

[固定结构化Server](structured.go)返回文档ID、超时值、单位和资料版本；文本块保留同一对象的JSON，结构化字段供程序直接处理：

```json
{
  "content": [{"type": "text", "text": "{\"id\":\"doc-A\",\"timeout_seconds\":30,\"unit\":\"seconds\",\"version\":\"4\"}"}],
  "structuredContent": {"id": "doc-A", "timeout_seconds": 30, "unit": "seconds", "version": "4"}
}
```

对应outputSchema声明四个必需字段：id、unit、version为字符串，timeout_seconds为整数，拒绝额外字段。适配器另外检查秒数为1–3600、单位为seconds，并核对本次请求ID与应用指定资料版本。Schema形状、业务范围和来源适用性各自判断；version是资料版本，不能与MCP协议版本混用。

接收方的[类型](structured.go)保留程序需要的字段：

```go
type StructuredDocument struct {
    ID             string `json:"id"`
    TimeoutSeconds int    `json:"timeout_seconds"`
    Unit           string `json:"unit"`
    Version        string `json:"version"`
}
```

接收与输入组装按四步分开：

1. **保留协议结果**：核对请求关联、isError、内容类型与体积，失败结果不进入正常事实路径。
2. **验证结构化数据**：按已审核的outputSchema检查字段，再按本次任务核对对象、单位、资料版本及业务范围。
3. **决定模型视图**：只投影本轮需要的值与来源，必要时生成简短说明；不要把完整JSON和同义长文本无条件重复塞入Context。
4. **核对最终使用**：答案中的数值与引用仍须经过RAG/Eval检查。字段存在、进入输入、被正确用于回答是三种不同证据。

规范为兼容旧Client，建议结构化结果同时提供序列化JSON的文本块；它不是自动的Host转换。若文本说30秒、结构化字段却是60秒，应按应用契约拒绝或待复核，不能让模型随意挑一个，也不能用isError=false消除矛盾。需要额外读取的资源链接同样不直接授予权限。

[CallStructured](structured.go)先确认发现结果包含支持的outputSchema，再执行同一批准与参数检查。工具错误返回ErrTool，正常结果严格核对两份对象、字段大小写、身份与版本；缺字段、重复字段、错类型或文本矛盾均不返回候选。取消在发送、响应及解析后检查，不将迟到结果交付。

Observation只投影已经检查过的字段，调用方再决定是否进入Context。实验通过真实stdio验证交接，没有调用模型、证明资料真实性或提供通用JSON Schema引擎。当前输出Schema仅接受这一固定四字段子集，不支持enum、minimum等扩展；Go int解码还拒绝30.0等数字写法，即使其数学值为整数。旧Call仍消费文本，不自动获得这条结构化验证能力。


<a id="book-agent-mcp-多个server如何共享一次运行"></a>

## 多个Server如何共享一次运行

### 多个Server如何共享一次运行

切换Server改变的是动作的目标，不应重建Run或清空历史。一个Loop可以依次提出archive::read_doc和primary::read_doc，收到各自观察后再比较；完整工具名与Call ID一起保留，不能把两个同名工具的结果混成同一个来源。

[路由适配](agent.go)只替换执行函数，轮次、工具尝试、ID校验和失败处理仍使用同一个循环：

```go
func RunRoutedLoop(ctx context.Context, router *ToolRouter, question string, decide Decision) (LoopReport, error) {
	if router == nil || router.routes == nil {
		return LoopReport{}, ErrProtocol
	}
	return runLoop(ctx, question, decide, func(ctx context.Context, call LoopCall) (string, error) {
		return router.Call(ctx, call.Tool, map[string]any{"id": call.DocumentID})
	})
}
```

两个Server共享四轮决策与两次工具尝试；每个Client还保留自己的连接和请求限制。第三个工具提议不会因为换了Server就得到新配额。模型选择的是Host已登记的完整名称，不是可执行文件或网络地址；两个Client的权限也不会合并。

工具业务失败回填带原名称的失败观察，下一轮可以明确提出另一个获准动作；这与路由器自动替换目标不同。传输失败直接结束本次运行，不用另一份同名资料掩盖原目标失效。若任务确实允许降级，应先定义可以替换哪些来源、怎样标记缺失和判断任务完成，再由策略显式选择。

多Server接入仍可以由一个决策循环完成。它增加了资料来源与会话边界；多Agent协作则还涉及多个执行者的任务和结果交接，不能因连接了两个Server就混为一谈。判断路由是否有效，要核对实际选中的来源、两边的请求变化和最终回答；远程身份、SDK兼容性与真实模型选择分别验证。


<a id="book-agent-mcp-关联标识由谁生成"></a>

## 关联标识由谁生成

### 关联标识由谁生成

本地JSON模型接口不必要求模型凭空编一个唯一ID。应用可以先分配本轮关联标识，模型只输出工具与参数，再由adapter把合法提议绑定到该标识。标识解决提议和观察的对应关系，来源选择仍由模型决定。

配套[v2适配](model_agent.go)只在完成生成与形状检查后绑定：

```go
	if callID == "" || len(callID) > 128 || reply.Call.ID != "" || reply.Call.Tool == "" || reply.Call.DocumentID == "" {
		return LoopReply{}, ErrModel
	}
	reply.Call.ID = callID
	return reply, nil
```

这段代码不填工具名、不替换文档ID，也不把缺失字段改成成功。原始响应与BoundCallID分别保存，通用Loop继续检查重复和预算。已有Provider工具调用ID时，要按对应协议保留并回填；应用绑定是本地接口的明确约定，不能偷偷改写外部协议标识。

Agent调用标识、MCP JSON-RPC请求ID和Server工具名字是三件事：前者关联决策与观察，请求ID关联一次协议交换，名字定位能力。分清所有权以后，还要检查是否重复读同一资料、遗漏另一个来源；换一个合法ID不能增加证据。


<a id="book-agent-mcp-最佳实践"></a>

## 最佳实践

## 最佳实践

### Server身份与连接由部署配置决定

Host启动或连接的目标应来自批准配置，不能来自用户文档或模型提议。stdio进程继承的环境、文件权限和资源范围也要限制；Remote HTTP另有身份与来源校验。

### 能力变化重新检查政策

工具清单更新时，发现新能力、校验Schema、审核批准范围再发布。分页必须完整处理，本实验遇到nextCursor直接拒绝，避免把部分清单误当全部能力。

### 先看协议，再调模型

坏ID、错误版本或不支持的结果类型先在Client/Server复现。协议稳定后，再检查模型选择与观察输入；调Prompt不能修复损坏的stdio消息流。


<a id="book-agent-mcp-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 初始化成功，却不能调用工具

检查initialized通知、Server能力、本地批准清单与发现记录。握手和发现都不保证获准执行，不能移除政策检查来“调通”。

### 同名工具读取了另一份资料

先核对完整本地名称到Client的绑定，再看请求使用的远端工具名与资料ID，最后检查返回来源。不要按裸工具名合并发现列表，或在断连后随意换Server。请求ID只在对应会话中关联，跨Server日志还需记录可信别名与应用Run身份。

### 日志输出导致解析失败

核对stdout是否混入普通日志，将诊断写stderr。再检查换行与消息大小，不用忽略非法行的方式跳过可能丢失的响应。

### 工具错误被模型当作资料

检查isError是否在适配中被丢弃，失败是否被转换成正常字符串。直接测试工具失败，再核对Loop回填；错误观察不等于可信证据。

### 资源已发现，模型仍没有看到资料

先检查resources能力和独立批准范围，再核对读取结果的URI、MIME与内容边界，最后检查Context是否选择了该资料。资源列表不是模型输入，读取结果也不会自动变成tool观察。Prompt同样需要用户选择和消息组装，不能通过移除角色校验来接入外部system消息。

