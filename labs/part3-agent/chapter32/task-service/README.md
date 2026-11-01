# Task Service：请求入口与独立任务

将已有任务契约接到HTTP，不重复实现Runtime。服务、任务和请求分别有生命周期；使用标准库和只读fake worker，无真实模型、数据库或外部业务工具。

前置：[Async Tasks](../../../../book/part3-agent/runtime/chapter32-async-tasks.md#topic-agent-async-tasks)、[Streaming](../../../../book/part3-agent/runtime/chapter33-serving.md#topic-ch32-streaming-service)、Go HTTP/context/测试。共享任务状态机在[labs/internal/tasks](../../../internal/tasks/tasks.go)，Async Lab与本Lab独立启动，不依赖彼此状态。

## 配置与边界

- 默认127.0.0.1:18082，只允许字面量回环IP，拒绝0.0.0.0/公网监听；不允许把公开演示身份暴露到网络。
- `demo-alice`/`demo-bob`是公开测试标签，服务端表分别映射到demo租户下的alice/bob；不是生产秘密，不证明身份提供方安全。生产要换成经验证的会话/Token，并配置TLS、撤销与轮换。
- 全局2个在途任务、16条保留记录；输入最大4096字节，HTTP body8192字节，输出8192字节；无队列、无跨租户公平保证。
- 任务总时限5秒；fake等待300毫秒。HTTP读头2秒、读/写3秒、空闲30秒、MaxHeaderBytes8192。MaxHeaderBytes是Go头部限制配置，不声称完整网络报文精确上限。
- 终态保留5分钟，每30秒可信应用清理一次，去重窗口同样过期。记录不持久化，重启不支持旧ID。
- SIGINT/SIGTERM：readiness置false，拒绝新工作，取消任务，HTTP shutdown与worker等待共用3秒总关闭预算；未退出报告失败，不冒充成功。
- body预检查拒绝原始非法UTF-8、重复字段及大小写别名和过深嵌套；JSONDecoder随后拒绝未知字段、类型错误和多份JSON。它不是完整JSON Schema/Unicode语义验证器，业务字段由任务契约继续校验。

## 启动与协议观察

从仓库根目录执行，准备方式见[运行说明](../../../../book/introduction.md#running-examples)：

```bash
go -C labs run ./part3-agent/chapter32/task-service
```

启动输出：

```text
task service listening on http://127.0.0.1:18082
```

创建任务：

```bash
curl -i -X POST http://127.0.0.1:18082/tasks \
  -H 'Authorization: Bearer demo-alice' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: request-1' \
  -d '{"input":"timeout"}'
```

应返回202、Location `/tasks/task-1`、X-Request-ID和任务快照。首次通常为running；调度较慢时也允许已进入终态，不能以固定睡眠证明状态。相同Scope/key/归一化初始输入重试得到同ID；换输入返回409。

```bash
curl -i http://127.0.0.1:18082/tasks/task-1 -H 'Authorization: Bearer demo-alice'
curl -i http://127.0.0.1:18082/tasks/task-1 -H 'Authorization: Bearer demo-bob'
curl -i http://127.0.0.1:18082/readyz
```

Alice完成查询应为200，state为completed、output为`evidence: timeout`；Bob查询同ID为404；无Authorization查询任务为401。Request ID每次不同，不是Task ID或幂等键。

更新与取消必须对**尚未结束**的任务操作，先查当前revision。默认fake很快，固定阶段竞争使用测试控制，不通过抢速度证明：

```bash
curl -i -X PATCH http://127.0.0.1:18082/tasks/task-1 \
  -H 'Authorization: Bearer demo-alice' -H 'Content-Type: application/json' \
  -d '{"input":"retry","revision":1}'
curl -i -X POST http://127.0.0.1:18082/tasks/task-1/cancel \
  -H 'Authorization: Bearer demo-alice' -H 'Content-Type: application/json' \
  -d '{"revision":2}'
```

已完成任务或旧revision返回409；合法控制请求返回202。Cancel响应是canceling或已知取消状态，退出确认通过后续查询。GET/tasks只列当前Scope的保留任务。

## 验证

```bash
go -C labs test ./part3-agent/chapter32/task-service ./internal/tasks ./part3-agent/chapter31/async-tasks -race -count=1 -v
```

测试应全部PASS：12个并发相同提交只产生一个ID；满额时不同新任务503并带Retry-After，同键重试仍返回已有任务。另覆盖未知身份、跨用户读取/更新/取消、body上限和未知字段、请求结束以后任务继续、版本冲突、readiness与liveness分离、协作关闭和不协作worker导致关闭超时。

关闭超时测试最后手动释放阻塞worker，验证最终canceled且迟到输出不提交；不能把仍在阻塞的goroutine留在默认测试中。实际SIGTERM测试与HTTP往返另行验证，httptest不是代理/真实认证/持久服务的证明。

## 练习

### 实际HTTP的执行与保留容量

```bash
go -C labs test ./part3-agent/chapter32/task-service -run '^TestHTTPExecutionAndRetentionCapacity$' -race -count=1 -v
```

[capacity_http_test.go](capacity_http_test.go)通过标准库httptest实际发送回环HTTP请求；需要本机监听权限。worker由通道控制，测试任务期限1分钟、请求批次守护期限10秒，用于避免调度先后与固定sleep决定结果；这些不是生产CLI的5秒任务配置或性能目标。

预期观察：12个不同key同时提交，2个接纳、10个capacity拒绝；满额时同key重试返回原ID。放行worker并逐项Wait确认完成后，将终态记录填至16条，第17个新请求503，旧key仍能查回。readiness保持200，当前接纳标记不检测瞬时容量。

应用维护调用Cleanup时传入模拟未来时间，清理16条记录及幂等键；不实际等待5分钟，也不证明生产定时器已运行。随后同key获得新ID并实际执行，worker合计17次。HTTP请求全部收齐，响应体有界并关闭，Server与Manager在退出时清理。

该实验验证实际网络请求下的接纳/去重/保留边界，worker仍为本机固定函数，不测模型能力、持续负载、P95或租户公平；任务状态Wait与HTTP响应是两份证据。

### 迁移任务

| 任务 | 起点与提示 | 边界与验证 |
| --- | --- | --- |
| 响应丢失后重试 | 创建以后丢弃响应，用相同key重发 | ID相同、worker仅一个；换输入409 |
| 请求超时不取消任务 | 短客户端context发起任务，再用新请求查询 | 已接纳任务可继续；尚未接纳时不得创建 |
| 扩展错误日志 | 为拒绝和调用失败记录Request/Task关联 | 不记录Authorization、原问题、全部输出；公开error稳定 |
| 队列取舍 | 在副本添加有界等待队列 | 排队/运行分别限额，过期和取消可测；不能改成无限等待 |
| 部署配置 | 将服务配置移到服务端环境，保留安全默认值 | 无秘密进Pages；回环演示身份不当作生产鉴权；部署后实际验证依赖与关闭 |
| 容量观测 | 分别统计请求、逻辑任务、worker调用与保留记录 | 重试不算新增任务，202不算任务完成；说明稳态/突发负载和轮询延迟口径 |

共享Manager还保留worker_canceled/worker_deadline_exceeded，任务DTO不暴露原始包装错误；显式取消、总任务超时与子请求停止分别定位。

## 请求编码与字段歧义

body先通过8192字节读取上限，再检查原始UTF-8、每对象大小写归一后的字段唯一性及16层嵌套上限；随后typed decoder拒绝未知字段与多余JSON。input/input、input/Input、转义后同名键与非法UTF-8均返回400，不登记任务、不启动worker，也不占用幂等键；超过body仍413，媒体类型仍415。

```bash
go -C labs test ./part3-agent/chapter32/task-service -run 'TestAmbiguousBody|TestUniqueNames' -race -count=1 -v
```

这是窄结构协议的防歧义规则，不是完整Schema/Unicode语义校验。不同对象的同名字段不互相冲突，正常body沿用原接纳/资源/版本/关闭契约。

## 来源与许可

原创Go代码，MIT。HTTP语义见[RFC9110](https://www.rfc-editor.org/rfc/rfc9110.html)，关闭见[Go Server.Shutdown](https://pkg.go.dev/net/http#Server.Shutdown)。未复制第三方框架代码；本机容量边界不提供生产部署或持续吞吐结论。

<a id="book-agent-serving-最佳实践"></a>

## 最佳实践

## 最佳实践

- **以资源契约组织API**：创建、查询、更新与取消含义明确；Task State与HTTP状态分别处理。
- **让同一Scope贯穿操作**：身份从可信验证结果取得，资源层再次检查，返回DTO限制公开内容。
- **先处理去重再申请新名额**：同键重试应能查回已有任务，响应丢失不能变成重复执行。
- **用有限拒绝保护执行资源**：body、在途、保留记录、队列、重试和任务期限分别有界。
- **确认关闭事实**：readiness、HTTP退出与worker退出分开记录，关闭超时不等于已经排空。


<a id="book-agent-serving-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

| 现象 | 先定位哪一层 | 原因与处理 |
| --- | --- | --- |
| 同一问题执行两次 | 客户端key → 接纳原子性 → 存储范围 | 换键、查建分离或跨实例本机去重；绑定请求并使用一致性边界 |
| 已返回202却查不到 | Scope → 实例 → TTL与持久状态 | 身份或实例不一致、记录过期/重启；不能仅增加前端重试 |
| 请求断开后任务消失 | 请求context与任务context | 生命周期耦合；按异步契约由应用持有执行 |
| 200页面显示成功，任务其实失败 | DTO.state与error → 页面映射 | HTTP查询成功被误作任务成功；保留执行失败状态 |
| 流量停止仍有worker | readiness → HTTP关闭 → 下游取消 → drain确认 | 不再监听不是执行已退出；检查阻塞边界与关闭期限 |
| 偶发503越来越多 | 在途、保留记录、排队等待分别统计 | 容量、清理或重试风暴；不要直接无限扩worker |

排错先直接检查Task Manager，再用httptest检查入口，最后验证实际网络与部署。真实认证、代理、跨实例存储与Provider是各自的验证层，不能由本地handler测试代替。

