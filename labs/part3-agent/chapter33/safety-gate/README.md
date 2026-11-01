# Safety Gate：不可信提议与受控执行

默认实验把模型提议直接送入应用Gate，验证拒绝是否发生在Reader执行之前。只有注册资料的只读读取，不访问网络、文件系统或真实模型，不执行Shell和业务写操作。显式容器伴随实验另验证固定进程的隔离，不给Agent新增工具。

前置：[Tool](../../../../book/part3-agent/building/chapter23-actions.md#topic-agent-tool)、[MCP](../../../../book/part3-agent/building/chapter23-actions.md#topic-agent-mcp)、[Serving](../../../../book/part3-agent/runtime/chapter33-serving.md#topic-agent-serving)。

## 机制与边界

- Scope、资料分类、能力清单和过期时间由可信应用建立；提议只有工具名与参数，不能自行传user/role改变身份。
- 固定注册read_document，参数只允许一个document_id；拒绝重复字段、其他字段、多JSON、错误类型、URL/路径及1024字节以上参数。ID不是路径，不访问真实文件。
- 文档ID必须在批准集，Scope相同且分类不高于策略。secret始终不可读；读取资料中的指令仍是untrusted数据。
- 每Run最多4个尝试，包含拒绝；并发共用计数。第5次以后拒绝，不继续Reader。Audit记录4次决策及首次预算耗尽，不因攻击重复增长。
- Reader执行前检查权限，返回后重查取消/期限/撤销；取消不会强行抢占正在运行的函数，撤销也不撤回已发生的读取。
- 返回ID和4096字节输出上限验证，不证明断言来源/真实性，也不自动识别任意自由文本中的秘密。可信Reader与资料分类正确性是前提；真实系统还需投影、DLP、出口限制与治理。
- Audit只存可信Run ID、尝试、固定工具类别、原因与成功字节数，不存问题、原始参数、Scope、结果或原始err.Error。此处丢弃私有错误原因，未实现持久/防篡改审计与受限诊断通道。
- 只有同进程、每Run对象的原子权限边界；不是OS Sandbox、身份提供方或MCP完整安全实现。

## 从仓库根目录运行

Go与依赖准备见[运行说明](../../../../book/introduction.md#running-examples)。

```bash
go -C labs run ./part3-agent/chapter33/safety-gate
```

输出：

```text
evidence_trust=untrusted injected_action_denied=true audit_events=2
```

先预测：检索结果要求删除数据库，或者参数附带user=bob，是否会改变应用权限？随后运行：

```bash
go -C labs test ./part3-agent/chapter33/safety-gate -race -count=1 -v
```

全部测试应PASS。对抗提议直接到Gate，拒绝用例同时检查错误与Reader调用次数为0；正常读取保留原文的指令但标记untrusted，随后恶意动作拒绝。另覆盖登记后输入map变化、迟到结果与撤销、取消、过期、错误结果ID/大小、日志canary以及并发尝试预算。

Canary是`SYNTHETIC_CANARY_NOT_A_CREDENTIAL`，仅为合成测试标记，不能用真实秘密测试日志泄漏。没有真实模型，测试比例不作为Prompt Injection攻击成功率。

## 结构化结果的公开字段投影

[projection.go](projection.go)保留内部ID/Scope、超时/重试以及合成凭据/地址。Gate先授权资料，Reader再调用ProjectConfig核对源记录归属与数值，构造PublicConfig并序列化；不先序列化整份内部记录。旧默认演示不变：

```bash
go -C labs run ./part3-agent/chapter33/safety-gate -projection
go -C labs test ./part3-agent/chapter33/safety-gate -list '^TestProjection'
go -C labs test ./part3-agent/chapter33/safety-gate -run '^TestProjection' -race -count=1 -v
```

输出：

```text
projection={"document_id":"guide","timeout_seconds":30,"retry_limit":2} trust=untrusted
```

[projection_test.go](projection_test.go)验证准确三字段及值、私有canary不进入结果/审计、跨用户与secret拒绝时Reader不调用，以及源记录ID/Scope不符和数值越界不返回部分文本。超时1–3600秒、重试0–10仅为合成任务合同，合法边界另验证。字段投影不认证数据来源、不过滤任意自由文本，也不证明数值支持回答；Gate的期限、撤销、预算与交付检查仍保留。

## 练习

| 任务 | 起点与提示 | 边界与验证 |
| --- | --- | --- |
| 扩展对抗用例 | main_test.go的提议表，增加未知字段、编码和重复JSON | 被拒绝时Reader计数为0；保留正常请求可读 |
| 检查预算绕过 | 同Run并发发送正常/拒绝请求 | 尝试最多4，拒绝也计入；审计不增长、不丢已有成功决策 |
| 定义新的公开投影 | 从projection.go的PublicConfig/ProjectConfig起步，为另一结构化资料设计独立公开字段；保留原数字投影作回归 | 自行加入新增私有字段、错来源与公开自由文本canary反例；说明哪些只能靠字段白名单保护、哪些仍需内容治理。原三字段测试不替代新合同或任意文本秘密检测 |
| 验证授权中途过期 | 在副本设短期限、控制Reader返回 | 过期后不释放结果；记录稳定拒绝，不假装撤回已执行读取 |
| 隔离选择 | 对照Workspace与真实容器/进程能力设计 | 明确文件/网络/CPU/秘密挂载与销毁；未实际隔离不标通过 |

## 固定进程的实际Docker隔离

准备本机Docker Linux引擎与已缓存Go工具链。测试只允许本地Unix endpoint，随后固定该endpoint，拒绝远程上下文；需要访问本机Docker socket的权限。默认验证跳过本实验，显式启用时缺少条件将失败。

```bash
SAFETY_SANDBOX_E2E=1 go -C labs test ./part3-agent/chapter33/safety-gate -run '^TestActualContainerBoundaries$' -race -count=1 -v
```

[container_test.go](container_test.go)将[固定探针](cmd/sandbox-probe/main.go)以CGO_ENABLED=0交叉编译为引擎架构，再将仅含探针及合成根文件的tar导入已绑定的本机Engine；不使用外部基础镜像或独立构建后端，不下载镜像。Go准备沿用仓库缓存约定，源码、编译产物和合成资料都由测试控制，模型不提供镜像、命令或挂载参数。探针只在Linux/UID65532及内部开关满足时执行；不是普通主机工具。

UID和开关用于防止误运行，不提供身份或容器认证；实际执行环境与权限仍由可信测试端创建并核对。

容器采用非root65532、cap-drop ALL、no-new-privileges、只读根、network none、64MiB内存与等额memory-swap、0.5CPU、16 PIDs；/tmp为8MiB的noexec/nosuid tmpfs。只读挂载自建公开合成资料，未挂载主机根、Docker socket或凭据；内部开关和GOMAXPROCS为唯二显式环境参数。

预期分层结果：

- 探针实际UID65532，/proc/self/status报告NoNewPrivs=1及CapEff=0。
- 批准资料可读，写入批准挂载与根文件得到EROFS，临时目录可写；未挂载的合成文件不可见。
- TEST-NET地址198.51.100.1没有外部路由，本容器loopback仍可连接；不访问真实外部服务。
- 内存模式最多申请256MiB并触页；64MiB容器应OOMKilled=true、退出137、Running=false。
- PID模式最多启动16次固定/probe wait，成功创建者全部kill/Wait；cgroup v2 pids.events的max应从前值增加。启动数量及事件次数随调度变化，线程也占额度，不断言必须创建16个进程。缺少控制器时显式失败。
- CPU模式执行固定忙循环，约两秒墙钟后检查cpu.stat；nr_periods、nr_throttled与throttled_usec均须比前值增加。实际本机一次结果为周期/事件0→21、限流时间0→1006782微秒；数值不作为固定断言、吞吐或SLO，宿主调度可能延长返回时间。
- 固定等待模式中，300ms客户端等待超时后容器仍Running，显式kill后停止；300ms是守护试验条件，不是业务SLO。
- 每个容器和镜像都带本次唯一标记，变更前登记标签清理，使用独立短时限删除自建资源，再核对无标记遗留。额外创建者的ID故意丢弃，仍须被标签清理找到；这不模拟所有daemon失联/创建时序。共享Docker构建缓存不prune，失败时报告清理是否确认。

实际本机环境为Docker Engine28.5.1、Linux/aarch64；跨平台仍需重新运行。正向/拒绝、内存/PID/CPU与等待实验不证明容器逃逸免疫、真实注入防护、目标白名单SSRF、生产隔离或模型收益。可改变只读/挂载策略在独立副本中观察，但不扩大到主机根目录、真实资料或控制socket。

## 来源与许可

原创Go代码和机制，MIT。参考[OWASP Prompt Injection防护](https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html)、[MCP安全实践](https://modelcontextprotocol.io/docs/2025-11-25/tutorials/security/security_best_practices)。未复制其代码、图片或整段文字。

上游context已超时与下游返回包装的DeadlineExceeded使用同一安全deadline_exceeded分类；主动取消用canceled。TestParentDeadlineHasDistinctSafeAuditCategory确认Reader未执行、无正文和准确停止类别，原始错误文本仍不进入审计。

<a id="book-agent-safety-把容器配置转成可验证边界"></a>

## 把容器配置转成可验证边界

### 把容器配置转成可验证边界

容器启动成功，只说明进程能够运行。若把主机根目录或Docker socket挂进去，进程仍可能取得过大的能力。Docker控制端因此只由可信应用持有，挂载、身份、网络和资源限制不接受模型提议。[Docker安全说明](https://docs.docker.com/engine/security/)进一步解释控制接口与容器配置的风险。

配套实验从课程自有静态Go探针准备最小根文件系统镜像，运行固定模式，不下载基础镜像，也不加载模型生成代码。批准资料单独只读挂载，其余主机资料不挂载；根文件系统只读，只开放有大小限制的临时目录。它验证一个具体执行环境，未将Docker或Shell注册给Agent。

| 控制 | 应观察的证据 | 仍需区分什么 |
| --- | --- | --- |
| 非root、去除capabilities、禁止新权限 | 实际UID、进程CapEff及NoNewPrivs | 参数存在与内核状态分别核对，不能据此证明任意逃逸免疫 |
| 只读根与批准挂载 | 正常读取成功，写入返回只读文件系统错误 | 未挂载文件不可见不等于资料自动获得业务授权 |
| 无外部网络 | 保留目标地址无路由，本容器loopback仍可用 | 不提供带域名白名单、代理或重定向的出口策略 |
| 内存上限 | 有界分配触发OOM，状态明确标记OOMKilled | 单独退出码137也可能来自其他强制终止 |
| PID上限 | 有界子程序创建触发pids.events中的max计数增加 | 控制器计入线程，启动次数不等于内核任务数量 |
| CPU上限 | 固定计算负载下nr_throttled与throttled_usec增加 | 限流参与调度，不证明业务吞吐或延迟SLO |
| 超时与清理 | 停止自己的容器，确认标记下没有遗留容器或镜像 | 客户端停止等待不自动结束后台执行；共享构建缓存另管理 |

[文件探针](cmd/sandbox-probe/main.go)让“可以使用批准资料”与“不能改写文件”分别取证：

```go
		r.MountReadOnly = errors.Is(os.WriteFile("/input/attempt.txt", []byte("attempt"), 0600), syscall.EROFS)
		r.RootReadOnly = errors.Is(os.WriteFile("/root-writable.txt", []byte("attempt"), 0600), syscall.EROFS)
		r.TemporaryWrite = os.WriteFile("/tmp/probe.txt", []byte("temporary"), 0600) == nil
		_, err = os.ReadFile("/host-canary.txt")
		r.UnmountedAbsent = errors.Is(err, os.ErrNotExist)
```

测试资料与预置根文件具有可写权限，拒绝依据是EROFS，避免将普通用户权限错误当作只读挂载证据。批准资料的读取和临时写入另有正向检查，不能用“所有操作都失败”证明环境既安全又可用。

PID验证还要区分程序数与内核任务数。Linux的PID控制器限制fork/clone创建的任务，线程也占用额度；Go运行时与子程序的线程会共同消耗名额。因此，设置16不意味着可以再启动16个子程序。配套探针最多请求16次创建，再核对cgroup v2的pids.events前后计数，限制触发后终止并Wait收齐已创建子程序。

[计数读取](cmd/sandbox-probe/main.go)只接受明确的max字段，缺少控制器时不假定成功：

```go
func pidEvents() (uint64, error) {
	data, err := os.ReadFile("/sys/fs/cgroup/pids.events")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "max" {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, errors.New("missing pids max counter")
}
```

一次Start返回错误也可能来自文件、权限或内存，不能单独作为PID限制证据。内核计数说明对应限制被触发，但不证明实际业务的并发配置合适；线程需求、资源释放和正常任务可用性仍需一起评估。控制器的字段与线程口径见[Linux cgroup v2](https://docs.kernel.org/admin-guide/cgroup-v2.html#pid)。


<a id="book-agent-safety-最佳实践"></a>

## 最佳实践

## 最佳实践

- **缩小可执行面**：只注册当前任务需要的能力，参数选择批准资源，拒绝未定义动作。
- **授权一直到执行点**：身份、具体资源、分类与期限都检查，不能只在提示词或入口检查一次。
- **把资料当资料**：来源标记帮助组织Context，应用策略不从文档、模型或工具描述更新。
- **提前限制数据传播**：凭据不进入模型，公开字段和日志分别投影，结果与错误均有边界。
- **分别证明安全与可用**：直接提议测试执行器，模型对抗测试策略，正常任务测试误拒绝；未验证的隔离能力单独记录。


<a id="book-agent-safety-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

| 现象 | 检查顺序 | 原因与处理 |
| --- | --- | --- |
| 文档让Agent调用未批准工具 | 实际调用计数 → 注册表 → 入口是否被绕过 | Prompt不能强制执行权限；拒绝必须在唯一执行点成立 |
| 子角色获得父级全部资源 | 子能力集 → 资源Scope → 共享预算 | 权限与上下文继承过宽，按必要能力交集派发 |
| 引用正确但带出私有内容 | 资料授权 → 输入组装 → 答案投影 | 引用归属不是访问许可，检索命中前后都需范围约束 |
| 取消以后仍出现结果 | 下游协作 → 返回后检查 → 提交边界 | 已在途不等于可释放，丢弃迟到结果并记录终止 |
| 日志有key或原问题 | 记录入口 → DTO/事件字段 → 原始异常 | 先收集过量数据；改白名单，不依靠事后正则保证 |
| 画了Sandbox却仍能访问主机 | 实际挂载、网络、身份与资源限制 | 隔离配置不成立；用环境反例验证，不改标签掩盖 |

排错先定位应用Gate与实际调用，再检查adapter、数据与模型策略。安全边界可以在固定提议下验证，不需要等真实模型偶然生成一次攻击才知道权限是否有效。

