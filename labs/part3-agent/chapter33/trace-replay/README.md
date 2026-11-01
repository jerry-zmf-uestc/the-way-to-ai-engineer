# 实验：调用记录、脱敏事件与确定性重放

记录两次串行调用：模型提议read，工具失败。公共Event只有身份、摘要和稳定错误类别；私有Archive保存响应。Replay匹配原请求并返回录制结果，没有实时Handler入口。

基础Recorder使用标准库与固定数据，另有锁定OTel SDK的本地Span/传播对照；全部离线，不证明Checkpoint或真实模型逐字可重现性。

## 前置与入口

先读[Trace与Replay](../../../../content/book/part3-agent/governance/chapter34-observability.md#topic-agent-trace-replay)、Runtime与Checkpoint，运行准备见[全书约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：Recorder、公共事件、私有档案、严格Load与按顺序Replay。
- [main_test.go](main_test.go)：相同失败、变更输入、缺记录、未消费、敏感载荷不进Event、取消与协议错误分类。

## 预测与运行

录制的工具已经失败，重放是否会再请求工具来“修复”这条记录？

```bash
go -C labs run ./part3-agent/chapter33/trace-replay
go -C labs test ./part3-agent/chapter33/trace-replay -race -count=1
go -C labs test ./part3-agent/chapter33/trace-replay -cover
go -C labs build -o /tmp/trace-replay ./part3-agent/chapter33/trace-replay
```

输出：calls=2、events=4、live_tool_failure=true、replay_tool_failure=true、consumed=2。

## 边界观察

- 输入多一个空格、工具名或顺序改变：ErrMismatch，游标不前进。
- 需要的调用缺失：ErrMismatch，不访问真实服务补齐。
- 录制还有调用未消费：Done拒绝提前成功。
- model/tool与对应失败Code矛盾：Load/事件校验拒绝；取消、deadline、协议错误可用于两类调用。
- 负游标：ErrMismatch，不panic、不消费记录。
- 取消后的迟到响应：记录canceled，文本不作为成功响应。
- 超大响应：协议失败类别，与模型/工具服务失败分开。
- 公共Event不包含原输入、响应或内部错误文本；私有Archive仍含响应，需要独立访问保护。

最多16个调用、32个串行事件、每个输入/响应4 KiB、载入档案80 KiB。事件验证检查配对、序号与调用身份；并发Trace需要不同的重叠Span验证。Hash用于匹配，不是匿名化、认证或防篡改签名。

## 文本录制与错误优先级

[response_test.go](response_test.go)验证非法UTF8的成功响应不会被JSON替换后冒充原响应：录制空文本和protocol_failed，保存/加载后Replay仍返回ErrTrace。输入、调用名和Run的坏编码在live前拒绝，无事件或调用；Replay坏编码请求不消费游标。

```bash
go -C labs test ./part3-agent/chapter33/trace-replay -list 'TestInvalidText|TestResponseBoundary'
go -C labs test ./part3-agent/chapter33/trace-replay -run 'TestInvalidText|TestResponseBoundary' -race -count=1 -v
```

取消、包装deadline与普通工具错误同时带超大残余文本时，保留原error/稳定类别，文本不交付。只在没有原错误时将超限/坏编码响应分类为protocol_failed。结果检查不撤销实际调用，也不保证所有私有日志都已脱敏；真实调用方继续按原错误敏感性处理。

## 迁移练习

1. 改变请求编码，定义何时需要稳定规范化，不能忽略参数变化使错误重放“通过”。
2. 故意删除或乱序记录，确认明确失败；不要添加live fallback。
3. 在既有SDK与回环HTTP对照上增加队列传播或采集故障，保留Run/Call映射和计数口径；不能把本机请求当作跨服务部署证据。
4. 区分诊断与私有录制的权限、加密、保留和删除；用固定敏感标记检查公共事件，不能放真实秘密进夹具。
5. 将确认的失败转为Eval用例，区分“旧响应下代码路径重现”与“新模型效果评估”。

### 迁移：请求键序变化，为什么重放失配

在独立副本中从[Recorder.Do、Replay.Do和digest](main.go)开始，参考[失配测试](main_test.go)。为一个新请求定义query与limit字段，分别使用`{"query":"timeout","limit":2}`和`{"limit":2,"query":"timeout"}`作为录制与重放输入。先预测游标与错误，再验证现有按字节指纹匹配的行为。

为这个新请求定义窄类型和稳定编码，在录制与重放两端调用同一编码入口。规范化规则由应用协议决定：键序可以稳定，query内容和limit值不能被随意忽略、裁剪或转换。使用标准库已有编码能力，先说明如何拒绝额外字段、重复键或非法参数，不靠删空格“修复”所有请求。

完成后自行添加`TestExerciseCanonicalRequestReplay`，至少覆盖：

- 仅键序不同得到同一合法请求表示，消费预期记录；limit从2改为3仍ErrMismatch，游标不前进。
- 工具Kind/Name变化、缺记录和未消费记录保持明确失败，始终没有live fallback。
- 格式或参数非法时在调用前拒绝，实时Handler次数为零；公共Event仍不保存原请求或响应。
- 新编码规则有明确身份/版本，旧字节格式档案不被静默重新解释；历史输出不能证明新模型效果。

先用`go test -list`确认新测试存在，再运行它及TestMismatchMissingAndUnconsumedRecords。临时删除一条录制记录验证故障，恢复后复验；不要覆盖原始私有档案或往夹具放真实秘密。

写好新测试后，在副本的仓库根目录执行；发现不到新测试时先修正文件或名字：

```bash
go -C labs test ./part3-agent/chapter33/trace-replay -list '^TestExerciseCanonicalRequestReplay$'
go -C labs test ./part3-agent/chapter33/trace-replay \
  -run 'TestExerciseCanonicalRequestReplay|TestMismatchMissingAndUnconsumedRecords' -race -count=1 -v
```

闭卷解释：规范化解决的是哪种等价，为什么不能让“差不多的请求”共用记录？将query内容改动而只保持键序，再预测结果。SDK/HTTP与队列传播练习继续保留，不能由本练习继承其验证。

## 归档原始字节与传播边界

[json.go](json.go)在结构解码前检查原始UTF-8、深度≤8及重复/大小写别名字段；字段名只接纳本格式的ASCII标识，值仍支持合法UTF-8。80KiB上限先限制原始数据，尾随JSON与未知字段另拒绝。不能用后一个Schema/Run覆盖前值，也不把非法字节替换成另一份记录。

```bash
go -C labs test ./part3-agent/chapter33/trace-replay -run 'TestArchiveAmbiguous|TestArchiveInputGuard' -race -count=1 -v
```

测试覆盖重复、别名、Unicode字段别名、非法UTF-8、深度、尾随和尺寸，以及正常中文Run值。任何失败返回零Archive，原默认录制/重放CLI不变。

正文解释Run/Call与Trace/Span映射、HTTP/队列传播、多个来源的链接、业务终态及导出失败。基础Recorder不包含SDK；下节另用真实SDK/内存Exporter验证本地关系及回环HTTP。Collector、跨进程部署和可靠审计后端仍未接入，不能把内存事件校验当作生产观测完成。

## 实际SDK的Span与上下文传播

```bash
go -C labs run ./part3-agent/chapter33/trace-replay -otel
go -C labs test ./part3-agent/chapter33/trace-replay -run '^TestSDK' -race -count=1 -v
```

输出sdk_spans=4、exporter=in_memory、propagation=trace_context、network=false。Go OTel/API/SDK锁定v1.37.0，见[labs/go.mod](../../../go.mod)，用于固定实际核对的调用，不推荐跟随最新版本更新。依赖准备仍按仓库锁和缓存约定。

[otel.go](otel.go)使用真实SDK的局部Provider、同步内存Exporter及TraceContext。根Run有model/tool两子Span，carrier从tool上下文注入，再由模拟server.lookup提取；测试确认其父ID等于tool ID、同一Trace且父上下文标remote。ID由SDK随机生成，断言关系而非具体字符串。

操作回调真实返回合成private-prompt-canary文本和private-error-canary错误；Span只保留白名单Run/Call/outcome及稳定tool_failed状态，不写载荷/原始异常event。模型Span结束正常但业务Run失败，不能由单组件end推导任务成功。Provider未全局注册，独立一秒context关闭；Exporter快照在Shutdown清空前读取。

此CLI没有HTTP、OTLP、Collector、真实外部模型/工具或持久审计。内存inject/extract与SDK导出证明本地链路；实际HTTP另由下面的测试验证。原默认Recorder/Replay命令输出不变，二者没有自动共享归档或恢复状态。

## 真实HTTP请求中的头部传播

```bash
go -C labs test ./part3-agent/chapter33/trace-replay -run '^TestActualHTTP' -race -count=1 -v
```

[http_test.go](http_test.go)启动标准库httptest回环Server并实际发送请求，不访问外网。需要本机监听权限，Server和Provider均由测试清理。预期TestActualHTTPPropagatesSDKContext通过：

- Client在请求头注入标准上下文；[TraceHTTP](http.go)提取后将Server Span的context传给Handler。
- Client/Server共享Trace ID，Server父ID指向Client且标为remote；Server类型与导出记录分别核对。
- Span属性和事件为空，不复制请求URL、头部、正文或内部错误。
- 第二次请求提供无效traceparent，仍运行Handler、返回204并建立新的有效Trace；不把观测字段当业务授权。

两端仍在同一进程，使用同步内存Exporter；204仅表示此固定Handler执行结果。该测试不验证真实身份、HTTP业务错误分类、跨机器/代理传播、队列、多租户采样、Collector、批量导出失败或可靠审计。可先删除客户端Inject预测关联变化，再恢复代码验证；不要以手工写入固定Trace ID代替真实传播。

## 来源与许可

原创代码，MIT。[OpenTelemetry](https://opentelemetry.io/docs/concepts/signals/traces/)用于理解观测概念，未复制SDK实现或外部记录。

<a id="book-agent-trace-replay-最佳实践"></a>

## 最佳实践

## 最佳实践

### 事件与载荷采用不同保留策略

诊断默认保留必要元信息，原文录制单独授权。脱敏后也要检查来源、字段与错误文本，不以一份字段黑名单代替完整数据分类。

### 记录输入与配置身份

同一句问题在不同资料、模型、工具版本和预算下可能有不同路径。保存相应配置身份，Replay档案格式版本与运行配置分别标记；不能凭相同Run名字推导所有条件相同。

### 失败档案先验证再重放

严格检查格式、数量、指纹和错误类别，限制档案体积。不信任网页或模型提供的“录制文件”，更不允许它声明任意可执行代码或工具权限。


<a id="book-agent-trace-replay-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 重放时偷偷访问了真实服务

检查adapter是否在缺记录或匹配失败时使用live fallback。改成明确ErrMismatch，确认Replay没有实时调用入口；新的真实运行另建Run与预算。

### Trace显示成功，任务仍失败

区分Span调用成功、结果完整与业务目标完成。核对工具错误是否被最终文本掩盖，检查证据和Eval，不通过改Trace颜色修复任务。

### HTTP两端各自有Span，却连不起来

先检查发送前是否把Client Span的context注入实际请求头，再检查接收端是否从实际请求提取，并把新context交给Handler。只在两端分别Start，不传递上下文，就会得到两条独立Trace。若本机传播正确而部署后断链，再检查代理的头部转发和边界接纳策略；Span采集与上下文传播分别排查，不能靠更换Run名字修复父子关系。

### 公共日志没有prompt，却仍泄漏信息

检查录制档案、错误字符串、资源名、用户标识与可猜测摘要。公共事件安全不代表私有载荷安全，分别检查并处理保留副本。

