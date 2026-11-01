# 实验：授权偏好、版本冲突与PostgreSQL存储

候选偏好经可信用户确认写入，按Tenant/User隔离，版本CAS防止覆盖，删除清空内容并保留修订墓碑，旧索引候选必须回查事实源。Fake与真实PostgreSQL适配采用同一契约。

默认离线。SQL集成只在显式MEMORY_E2E和隔离memory_lab数据库上执行，不接触业务数据库、真实用户偏好或模型调用。

## 前置与入口

阅读[Memory正文](../../../../content/book/part3-agent/runtime/chapter30-state.md#topic-agent-memory)、Session、Context与SQL。Go版本与pgx v5.11.0以[labs/go.mod](../../../go.mod)为准；准备见[运行约定](../../../../content/book/introduction.md#running-examples)。

- [main.go](main.go)：Scope、记录、具体值/操作授权、Fake、召回与候选回查。
- [postgres.go](postgres.go)：真实database/sql驱动、参数化主键范围和条件更新。
- [main_test.go](main_test.go)：共用后端契约、并发CAS、隔离、过期、删除与再确认，数据库门控。

## 预测与离线运行

两个请求同时更新revision=1，有多少个成功？删除后旧候选仍存在，是否能召回值？

```bash
go -C labs run ./part3-agent/chapter27/user-memory
go -C labs test ./part3-agent/chapter27/user-memory -race -count=1
go -C labs test ./part3-agent/chapter27/user-memory -cover
go -C labs build -o /tmp/user-memory ./part3-agent/chapter27/user-memory
```

输出recalled=1、deleted_revision=2、stale_candidates_resolved=0。默认PostgreSQL测试跳过，只证明应用规则与Fake契约，不证明SQL行为。

## 候选键先校验，再回查

Resolve最多接收16个候选，每个键须非空白、UTF-8合法且最多128字节。整个列表在任何Peek之前校验；任一坏键返回ErrInput且无部分结果。合法短键查不到记录仍跳过，重复键只回查一次，不裁剪、改写或规范化标识。

```bash
go -C labs test ./part3-agent/chapter27/user-memory -list 'TestResolvePreflights|TestResolveKeepsBounded'
go -C labs test ./part3-agent/chapter27/user-memory -run 'TestResolvePreflights|TestResolveKeepsBounded' -race -count=1 -v
```

[candidate_input_test.go](candidate_input_test.go)将坏键放在合法locale后，检查零后端读取；分别覆盖空、纯空白、129字节、43个汉字与非法UTF-8。128字节的合法未知键及重复候选仍保持无记录/一次回查语义。数量限制不替代单项体积，合法键也不提供权限；范围和当前事实检查继续由原Resolve处理。此测试不启动数据库、Redis或向量服务。

## 独立PostgreSQL准备

仅用自己创建的隔离实验数据库。可采用Docker官方PostgreSQL18镜像：先在本地环境设置一次性MEMORY_POSTGRES_PASSWORD，不写入源码、命令日志或公开夹具。

```bash
POSTGRES_PASSWORD="$MEMORY_POSTGRES_PASSWORD" docker run --rm -d --name ai-memory-db \
  -p 127.0.0.1:15432:5432 \
  --env POSTGRES_PASSWORD \
  --env POSTGRES_DB=memory_lab postgres:18.6
```

设置本地MEMORY_DSN（指向该数据库，密码只放本地变量），再显式运行：

```bash
MEMORY_E2E=1 MEMORY_TEST_DATABASE=1 \
  go -C labs test ./part3-agent/chapter27/user-memory -run TestPostgresContract -race -count=1 -v
```

测试核对current_database()为memory_lab，创建专属course_memory_entries表，并运行同一契约。数据库角色为实验身份，未证明生产RLS或管理员隔离。命名空间包含固定教学Scope；重复测试前使用新的隔离数据库，避免旧夹具混用。

测试结束，只清理本次创建的容器：

```bash
docker stop ai-memory-db
```

容器为--rm，不保留真实数据。也可使用明确配置的本地PostgreSQL；本课程验证曾采用官方校验源码构建的临时18.6实例，不依赖Docker下载是否可用。不要将E2E指向现有业务数据库。

## 关键失败

- 未授权、确认值变化、操作不同或确认到期：写入前ErrDenied。
- 候选类型/枚举非法或TTL超过30天：ErrInput。
- 同版本并发更新：一个成功，一个ErrConflict。
- 跨Tenant/User：不返回对方记录。
- 删除后Value/Source清空，旧revision不能写回；再确认使用新墓碑revision。
- 精确到期或后端读取期间到期：Recall及候选回查不返回；不等于数据库行已物理清理。
- 读取期间取消：不交付迟到记录；空候选也不吞掉取消。
- 有效记录枚举/来源/修订异常、重复键或Peek返回另一键：ErrInput，不返回部分数据。
- 召回按Key排序，使Fake与SQL的模型输入次序一致。
- 网络/数据库错误不伪装成“没有记忆”，context传入查询。

读取结果是当前事实源快照，不是后续使用的永久授权。并发删除与模型消费之间仍有窗口；严格策略应携带版本并在相应边界重新核对。Consent当前在CAS发起前检查，提交时的授权一致性与结果未知需要另行协议。

## 旧读取回填与当前事实源

[cache_snapshot_test.go](cache_snapshot_test.go)按受控顺序重现：A用Peek取得revision1，另一确认CAS更新为revision2，清空缓存之后A再填回旧Record。旧值仍在自身ExpiresAt内；Resolve只使用候选键回查，取得新值而不采用缓存内容。删除以后候选仍存在，也不返回旧偏好；另一Tenant/User不能通过同一候选键取得该用户内容。

```bash
go -C labs test ./part3-agent/chapter27/user-memory -list '^TestLateCacheFillCannotReplaceCurrentMemory$'
go -C labs test ./part3-agent/chapter27/user-memory -run '^TestLateCacheFillCannotReplaceCurrentMemory$' -race -count=1 -v
```

这是顺序受控的内存反例，没有Redis连接、并发调度或失效通知。缓存TTL不等于原信息期限；每次回查仍有源读取成本。本测试是现有边界的验证，不代替下列真实Redis/向量迁移练习，也未证明使用收益。

## 显式Redis缓存对照

[redis_cache_test.go](redis_cache_test.go)启动自有Redis 7.2.11进程，只监听私有Unix socket（无TCP），禁RDB/AOF与daemon，内存上限16MiB。仅存公开合成Record，Scope与字段分别编码进入键；结束停止并等待进程，再清理自有目录。模型不能指定二进制、socket或命令。默认测试跳过。

运行时使用[官方源码与SHA256清单](redis-runtime-lock.json)，不安装到系统目录。准备示例在自己的临时目录执行：

```bash
MEMORY_REDIS_BUILD_ROOT=$(mktemp -d /tmp/memory-redis-build-XXXXXX)
curl -fL https://download.redis.io/releases/redis-7.2.11.tar.gz -o "$MEMORY_REDIS_BUILD_ROOT/source.tar.gz"
printf '%s  %s\n' '2f9886eca68d30114ad6a01da65631f8007d802fd3e6c9fac711251e6390323d' "$MEMORY_REDIS_BUILD_ROOT/source.tar.gz" | shasum -a 256 -c -
tar -xzf "$MEMORY_REDIS_BUILD_ROOT/source.tar.gz" -C "$MEMORY_REDIS_BUILD_ROOT"
make -C "$MEMORY_REDIS_BUILD_ROOT/redis-7.2.11" -j2 MALLOC=libc BUILD_TLS=no redis-server redis-cli
export MEMORY_REDIS_SERVER="$MEMORY_REDIS_BUILD_ROOT/redis-7.2.11/src/redis-server"
export MEMORY_REDIS_CLI="$MEMORY_REDIS_BUILD_ROOT/redis-7.2.11/src/redis-cli"
MEMORY_REDIS_E2E=1 go -C labs test ./part3-agent/chapter27/user-memory -run '^TestActualRedisStaleCacheRevalidation$' -race -count=1 -v
```

先确认源码校验通过再解包/构建；需要make与C工具链。版本与源码为复现实验而锁定，升级须更新清单并重跑；server/cli路径由可信应用提供，版本检查不代替源码校验。

实际Redis SET/DEL/GET/PTTL确认旧revision1在晚回填后仍存在且TTL为正；Resolve回查Fake事实源取得revision2，删除后旧缓存不恢复偏好，另一Scope/字段不会混用键。停止Redis后GET失败，应用明确回源Recall，仍取得已确认locale，不能把缓存连接错误当成没有记忆。

Redis单门禁对照使用Fake事实源。准备上节自己的隔离memory_lab并配置MEMORY_DSN后，可用同一缓存契约验证真实PostgreSQL：

```bash
MEMORY_REDIS_E2E=1 MEMORY_E2E=1 MEMORY_TEST_DATABASE=1 \
  go -C labs test ./part3-agent/chapter27/user-memory -run '^TestActualRedisPostgresRevalidation$' -race -count=1 -v
```

测试先核对current_database()，再使用SQLStore和按测试名区分的合成Scope。使用新隔离数据库，避免重复运行与旧夹具冲突。事实有效期采用注入的固定时钟，Redis PTTL来自真实缓存，不能把两者当成一个时钟；时间字段按同一时间点比较，不用Go时区Location指针判断是否相等。

实际联合结果同样为stale-revision=1、current-revision=2、TTL-positive=true、delete-not-revived=true、scoped=true、cache-down-fallback=true，primary=PostgreSQL。Redis保存整份旧Record仅用于暴露反例，实际输入只从当前事实源准入。它没有CLIENT TRACKING通知、网络集群、RLS、跨系统事务、崩溃恢复或延迟收益证据；这些不能由两种存储同时启动推导。

## 迁移练习

默认先用合成记录与Fake验证契约；数据库和缓存实践仅使用上文显式门禁及隔离环境，不操作真实用户信息。

| 任务 | 起点与改动 | 反例与完成证据 |
| --- | --- | --- |
| 1. 复核确认范围 | 从Memory.authorized/Write/Delete和TestConsentBindsValueAndOperation起步，分别改变确认值、操作、Scope与确认期限，再设计撤销后的检查位置 | 每一种旧确认都应被对应边界拒绝，存储版本不变；Consent由可信应用形成，不能从模型JSON恢复成授权。新增撤销机制须有自己的用例，过期测试不替代撤销 |
| 2. 验证持久存储 | 从postgres.go的SQLStore与TestPostgresContract起步，沿隔离memory_lab准备方式写入，再关闭连接池、重新打开读取 | 保存前后Record、Revision与过期判断，确认读取来自重连后的SQL；Fake PASS不能证明持久化。数据库不可用或测试跳过时报告未验证，不改门禁制造通过 |
| 3. 处理旧缓存候选 | 从Memory.Resolve和cache_snapshot_test.go起步，新增向量候选或失效通知/缓存断连变体；故意保留删除前键 | 先看到旧候选仍命中，再核对事实源回查不发布已删/错Scope记录。已有Redis/Fake及Redis/SQL对照只作基线；新增路径须记录自己的事件、回源与失败，不拼接不同运行当联合证据 |
| 4. 保存开放文本 | 从Record与validate设计有界文本记录，明确来源、冲突、作用范围、期限及授权；保留当前固定偏好接口作基线 | 自行构造一次临时要求、相反偏好与不可信资料建议，说明哪些可以保存及如何失效；不把一次提问自动变成永久偏好，字段合法不等于语义或授权正确 |
| 5. 增加数据库隔离 | 在SQLStore的表结构基础上设计RLS及专用非管理员身份，限定隔离测试库；应用Scope过滤继续保留 | 自行加入绕过应用查询、错租户与连接复用反例，核对实际数据库身份和RLS拒绝。管理员结果、WHERE条件或配置截图不能证明该运行身份已被隔离 |
| 6. 设计个人与团队范围 | 从Scope及进入Context前的Resolve边界起步，定义个人偏好和团队手册的所有者、成员资格、读写与删除规则 | 画出加入/退出团队和资料删除的失效路径；实现时新增成员撤销测试。当前Tenant/User接口不具备团队授权，不使用公共User混合各用户，也不把设计图标为共享存储已完成 |

先确认默认基线存在，再运行：

```bash
go -C labs test ./part3-agent/chapter27/user-memory -list 'TestFakeContract|TestConsentBindsValueAndOperation'
go -C labs test ./part3-agent/chapter27/user-memory -run '^(TestFakeContract|TestConsentBindsValueAndOperation)$' -race -count=1 -v
```

新增路径需要自行补测试。真实数据库、Redis、向量与人工授权效果分别记录，不以原测试通过代替新增机制；筛选前核对-list，不将全跳过算作完成。

## 来源与许可

原创代码和合成数据，MIT。PostgreSQL查询行为参考[INSERT](https://www.postgresql.org/docs/18/sql-insert.html)，驱动使用锁定[pgx](https://github.com/jackc/pgx)。依赖保留自己的许可证，不复制或提交数据库源码与运行状态。

## 当前事实到模型输入的投影

input.go的Memory.Input接收应用绑定的Scope和当前任务所选候选键，经Resolve回查后只返回key/value/revision。Source、Scope、Consent、期限与墓碑不进入投影；不自动构造system消息，不调用模型或授予工具权限。Resolve的16候选及单键128字节边界保持。

```bash
go -C labs test ./part3-agent/chapter27/user-memory -list '^TestMemoryInput'
go -C labs test ./part3-agent/chapter27/user-memory -run '^TestMemoryInput' -race -count=1
```

默认测试核对旧键更新后只输入detailed@2、序列化字段白名单、调用方修改投影不改事实源、跨用户为空、删除与到期不产生输入。既有TestActualRedisPostgresRevalidation在相同旧缓存/SQL更新场景中另核对实际投影；启用它仍须原Redis和隔离数据库门禁。默认跳过的集成测试不算真实存储通过。

投影是读取快照，生成期间更新与交付时授权需另定义复核；仅有revision不保证原子消费。迁移练习从input_test.go开始，创建TestExerciseCurrentRequestOverridesPreference：保存concise偏好，再提供本次detailed请求，设计输入组装明确当前请求优先，确认不修改持久偏好；新测试须-list发现后执行。不要用现有投影测试代替请求优先级或模型采用效果。

## 真实SQL/Redis输入到本地生成

在原隔离memory_lab与Redis准备之外，需固定Python3.12.14、完整[生成依赖锁](../../chapter28/evidence-answer/generation-requirements.lock)和已校验本地Qwen3-1.7B资产；GENERATION_PYTHON、GENERATION_ASSETS及MEMORY_INPUT_RECORD_PATH为应用指定绝对路径。记录路径必须尚不存在，排他0600创建。生成通过既有公开local_generator.py JSON入口，不导入另一Lab私有实现。

```bash
go -C labs test ./part3-agent/chapter27/user-memory -list '^TestActualMemoryInputGeneration$'
MEMORY_INPUT_GENERATION_E2E=1 MEMORY_E2E=1 MEMORY_TEST_DATABASE=1 MEMORY_REDIS_E2E=1 RAG_GENERATION_E2E=1 go -C labs test ./part3-agent/chapter27/user-memory -run '^TestActualMemoryInputGeneration$' -race -count=1 -v
```

沿用本页MEMORY_DSN、MEMORY_REDIS_SERVER/CLI配置。全流程180秒，每次输入1024/输出192Token、CPU单线程，三个状态各生成一次，初始化资产检查计入总时间。更新后、原事实仍存在时的其他用户、删除后三次逐一执行；不先删除再用旧快照批量生成。

| 状态 | 实际提交 | 原始结果 | 窄逐字检查 |
| --- | --- | --- | --- |
| Redis仍保存concise@1，SQL已为detailed@2 | response-style、版本2、detailed | insufficient_evidence拒答 | 失败：当前值可用却拒答 |
| 原记录仍存在，其他用户读取同一候选 | 空证据 | insufficient_evidence拒答 | 通过 |
| 删除后重新读取 | 空证据 | insufficient_evidence拒答 | 通过 |

Schema2实际三调用共输入579/输出54Token，均未截断；完整原文/messages在testdata/input-generation-v2.json。Schema1首轮在删除后批量生成旧快照，同样2/3，但只作输入/协议诊断，不能证明即时使用；原testdata/input-generation-v1.json保留。两轮是同三个状态，不能计成六个独立合格案例；Tokenizer计数不是Provider账单。

TestActualMemoryInputGeneration通过表示实际调用和记录合同完成，不表示模型质量通过。ExactResults只检查严格逐字值与引用或空证据拒答；没有通用语义Judge、实际表达风格任务或真人效果。默认重放不调用模型或数据库。迁移任务可在已有输入练习之外，将相同偏好用于一个故障解释任务，先写任务正确性与表达标准，再检查当前请求如何覆盖长期偏好；不得只以字符长度判定质量。

## 字段含义视图的同任务对照

沿原实际生成门禁显式设置MEMORY_INPUT_VIEW=key-value，并使用尚不存在的新MEMORY_INPUT_RECORD_PATH。默认纯值视图不变；新视图只把已回查的MemoryInput格式化为response-style=detailed，引用ID/版本独立保留，不传身份、确认或评分标签。原问题、Qwen模型、rag-joint-v3及180秒/1024/192/CPU1预算不变。

Schema3原记录testdata/input-generation-v3.json：三个实际调用输入581/输出77Token，均未截断，严格结果仍为失败/通过/通过。当前值输出为：

```json
{"abstained":false,"claims": [{"text":"response-style=detailed","citations":[{"evidence_id":"response-style","version":"2","quote":"response-style=detailed"}]}]}
```

模型已经引用当前字段和值，但问题要求值detailed，返回却包含字段名，并增加本次生成提示要求的claims之外的abstained字段。通用RAG的ParseJoint允许abstained=false，不能把本次提示的更窄要求说成通用解析器必然拒绝；即使放宽该字段，整段字段表达仍不满足原逐字值要求。原文不删除/改写；字段含义改善不等于任务完成，不能将JSON可解析算作通过。两个空输入正确拒答。Schema1/2原记录保持，三个视图重复同三状态不增加独立案例。

```bash
go -C labs test ./part3-agent/chapter27/user-memory -list 'TestMemoryKeyValue|TestMemoryInputGenerationRecords'
go -C labs test ./part3-agent/chapter27/user-memory -run 'TestMemoryKeyValue|TestMemoryInputGenerationRecords' -race -count=1 -v
```

默认投影/错值/错引用反例与三份原记录重放不调用模型。EvidenceText只是格式化，授权与新鲜度仍由Memory.Input/消费边界负责。实际表达偏好采用、通用语义评分与完整知识作品继续另行验收。

<a id="book-agent-memory-最佳实践"></a>

## 最佳实践

## 最佳实践

### 把确认和候选分开

模型可提取候选，用户或可信策略决定是否保留。确认绑定具体数据和操作，不给一张长期有效的空白授权。

### 用事实源裁决当前状态

缓存或向量命中后回查授权、版本与删除，身份范围始终来自调用方。不让不同后端各自维护互相矛盾的“最终值”。

### 生命周期进入接口

写入期限、当前请求覆盖、修改、删除与重建明确表达。结果未知、数据库失败和无记录分别处理，不把网络错误解释为“用户没有偏好”。


<a id="book-agent-memory-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 新会话读到另一个用户的设置

检查每次查询的Tenant/User绑定、缓存键和向量过滤，再看Context是否混入共享数据。不能只在前端隐藏记录，存储查询必须带可信范围。

### 删除后旧偏好又出现

检查主表状态、缓存、向量候选和模型历史。回查当前事实源，阻止旧候选直接入输入；另行清理衍生副本和保留范围。

### 偏好突然被旧任务改回去

核对revision与授权有效期，确认更新条件不是无条件upsert。竞争后重新判断，旧值不因重试获得写入资格。

