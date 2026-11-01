# 实验：会话快照、版本冲突与隔离

建立内存Session Store的消息契约：可信Scope与会话ID、独立快照、调用/结果ID、原子版本追加和TTL。实验仅保存固定教学数据；不访问模型或网络，不实现长期Memory、数据库持久化、幂等批次或执行恢复。

## 前置与入口

先读[Session正文](../../../../book/part3-agent/building/chapter26-context.md#topic-agent-session)，理解slice复制、Mutex与错误链。Go版本以[labs/go.mod](../../../go.mod)为准，准备见[运行约定](../../../../book/introduction.md#running-examples)。

- [main.go](main.go)：封闭消息类型、快照复制、历史校验、身份范围与版本提交。
- [main_test.go](main_test.go)：消息原子拒绝、竞争、过期、嵌套字节复制及Scope隔离。

## 预测与运行

创建会话分配修订号1，追加user和工具提议分配2，追加工具观察和最终文本分配3。持有修订号2再提交，应发生什么？

仓库根目录执行：

```bash
go -C labs run ./part3-agent/chapter27/session-history
go -C labs test ./part3-agent/chapter27/session-history -race -count=1
go -C labs test ./part3-agent/chapter27/session-history -cover
go -C labs build -o /tmp/session-history ./part3-agent/chapter27/session-history
```

输出：

```text
version=3 messages=4 stale_conflict=true
```

修订号在整个Store内递增，不等于会话消息数；其他会话提交会让编号跳跃。内存Store重启不保留修订号，生产实现需持久会话代次或不可复用ID。

## 故障观察

| 条件 | 结果 | 实验入口 |
| --- | --- | --- |
| 原参数或读取快照被修改 | 保存历史不受影响 | TestSnapshotsIsolationAndExpiry |
| 两个请求以同版本提交 | 一个成功，一个ErrConflict | TestConcurrentCompareAndAppend |
| 未知/重复tool结果或过早新用户轮 | 整批拒绝，版本与消息不变 | TestCallAssociationAndAtomicRejection |
| 不同Scope使用同ID | 不可读取或追加对方会话 | 隔离与过期测试 |
| 精确到达ExpiresAt | 读写返回ErrNotFound | 过期测试 |
| 同ID重建后，旧请求再次提交 | 旧版本返回ErrConflict | TestExpiredIdentityDoesNotReuseVersion |

最多128条消息、每消息64个提议与64 KiB内容；注册身份由调用方提供，不能交给模型决定。过期清理发生在访问时，不代表有后台回收器；长期无人访问的条目仍可能占内存。锁内不执行模型或工具。

## 修订号耗尽不回绕

[revision_test.go](revision_test.go)将Store计数设置到uint64上限附近，验证最后一个合法版本可创建或提交；之后Create/Append返回ErrRevisionExhausted，不发布零版本、追加半批历史或改变旧会话。旧快照仍可读取。该设置仅用于确定性反例，生产调用方不能通过重置计数绕过耗尽。

```bash
go -C labs test ./part3-agent/chapter27/session-history -list 'TestRevisionExhaustion|TestLastRevision'
go -C labs test ./part3-agent/chapter27/session-history -run 'TestRevisionExhaustion|TestLastRevision' -race -count=1 -v
```

计数器仍只在Store实例生命周期内有效；耗尽拒绝不提供跨进程代次、持久迁移或幂等提交。迁移练习需自己定义新代次与旧请求失效，不把同ID重新创建、版本清零当安全方案。

## 练习

在副本中完成以下任务；新行为需要自行添加测试，原测试只作为回归基线。

| 任务 | 起点与改动 | 反例与完成证据 |
| --- | --- | --- |
| 1. 处理提交冲突 | 从Store.Append及TestConcurrentCompareAndAppend起步，让竞争请求提交不同文本；在调用层设计“重新读取→判断原结果是否仍适用→提交”的流程 | 先记录两个请求的旧版本、提交结果与新快照；已经执行的工具不能因冲突再次执行。自行加入结果已过期和仍可用两种情况，不能无条件把expected替换为新版本重发 |
| 2. 扩展快照复制 | 给Message加入可变证据列表，并同步clone、bounds及validate；从TestSnapshotsIsolationAndExpiry改造新字段的用例 | 分别修改调用前参数、Append返回值和Read快照，新字段的保存内容均不受影响。限制元素与总大小，保留原Call/Args复制，不宣称支持任意指针深拷贝 |
| 3. 增加幂等批次 | 从Store.Append的锁内提交边界起步，设计Scope/会话/代次下的稳定batch ID与载荷摘要；保持版本冲突行为 | 自行验证同ID同载荷的重试、同ID不同载荷、响应丢失后的重试及会话过期重建。重复批次不再追加，但新批次仍检查版本；批次身份不替代工具幂等或Checkpoint |
| 4. 迁移数据库 | 根据Create/Read/Append写表结构与条件更新设计，将版本检查、消息写入和必要的批次登记放在同一事务中 | 先画出两写者及提交结果未知的时序；若实现数据库，用独立测试库和两个连接复现竞争，核对整批历史及版本。设计稿或内存PASS不算数据库机制已验证 |

从仓库根目录运行基线，先确认测试名称：

```bash
go -C labs test ./part3-agent/chapter27/session-history -list 'TestSnapshotsIsolationAndExpiry|TestConcurrentCompareAndAppend'
go -C labs test ./part3-agent/chapter27/session-history -run '^(TestSnapshotsIsolationAndExpiry|TestConcurrentCompareAndAppend)$' -race -count=1 -v
```

新增测试也要先用-list核对，再筛选执行。原历史保存成功不证明执行可以恢复；用自己的新反例说明会话版本、消息关联与Checkpoint游标分别负责什么。

## 来源与许可

课程原创代码，MIT。[sync](https://pkg.go.dev/sync)、[encoding/json](https://pkg.go.dev/encoding/json)用于核对标准库语义。

<a id="book-agent-session-最佳实践"></a>

## 最佳实践

## 最佳实践

### 一次提交明确一个批次

让调用方知道本次追加成功还是整体失败；不要在一个方法里偷偷拆分提交。若批次过大，调用方分阶段提交，并为每一阶段明确预期版本与恢复策略。

### 缩短临界区

复制与JSON解析前先限制批次数量、体积与调用数，再在锁内做有界的查找、复制、校验和更新，不执行模型、网络或工具。实验限制整份历史最多128条消息，每条最多64个提议；Content、CallID、提议ID、工具名与参数累计最多64 KiB，避免在锁内处理无限历史。这个计数不包含JSON字段名与编码开销，不能当成序列化请求的完整体积。规模增长后考虑按会话加锁、持久化与容量回收，先用测量决定是否需要更复杂的存储。

### 保存事实与选择输入分开

上下文太长时，不要为适应一次模型请求就覆盖原历史。Context生成一个输入视图；原始历史的删除或归档是另一个有数据保留规则的操作。


<a id="book-agent-session-常见问题排查"></a>

## 常见问题排查

## 常见问题排查

### 两个请求都成功，后来少了一条消息

先检查是否用旧快照整份覆盖，再检查版本比较与写入是否原子。让两个请求读同一版本并同时追加，应有一个冲突，而非两个“成功”。之后再检查应用是否吞掉冲突或无条件重试。

### 历史参数在模型调用后发生变化

先检查读快照是否共享Calls或Args，再检查写入是否共享原请求。修改输入参数和返回快照，重新读取Store，历史都应不变；只复制最外层slice不够。

### 工具结果无法关联

直接检查提议ID和结果ID，再检查adapter转换，最后看并行回填。不要先用工具名字猜对应关系；未知或重复ID应产生可见失败。

