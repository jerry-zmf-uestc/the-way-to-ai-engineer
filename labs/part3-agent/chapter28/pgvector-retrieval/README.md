# pgvector：实际向量存储与HNSW对照

同一固定向量数据比较Go精确扫描、PostgreSQL/pgvector精确查询与真实HNSW。前置：[Retrieval](../../../../content/book/part3-agent/building/chapter27-rag.md#topic-ch28-vector-retrieval)、SQL、权限与context。既有pgx驱动，不新增Go依赖。

## 验证范围

- 默认只运行1024条合成三维向量的Go扫描，不联网。数据不来自训练Embedding，不代表语义质量。
- 显式E2E才连接专用vector_lab，Seed再次检查数据库名；只建立session临时表和索引。应用Seed写夹具，Agent Search只读事务。
- pgvector锁定0.8.7（官方tag提交f37c13f68b57d2c3472b2214fbcff699d6d34876）；Vector(3)、cosine distance。PostgreSQL本机验证18.6，其他版本须独立验证。
- Scope由可信应用提供tenant/space/version，查询始终WHERE参数化；参数向量由有限数值生成，不接受任意SQL。固定query的空间由可信实验确定，裸Vector类型不能证明编码模型；真实adapter应显式核对查询Space标记。
- 精确模式禁用index/bitmap scan并检查Seq Scan，ANN模式检查vector_items_hnsw确实出现在EXPLAIN中。诊断SET LOCAL不作为生产调参建议。
- ANN的WHERE保证输出范围，但过滤可能在索引候选扫描后生效；迭代扫描用于补读，受预算与索引行为限制。不是RLS或跨租户物理隔离证明。
- 相对精确top-k的Recall验证索引近似，不能当真实问题的资料相关性Recall。无延迟/规模/神经Embedding收益结论。

## 默认运行

从仓库根目录执行，依赖准备见[运行约定](../../../../content/book/introduction.md#running-examples)：

```bash
go -C labs run ./part3-agent/chapter28/pgvector-retrieval
go -C labs test ./part3-agent/chapter28/pgvector-retrieval -race -count=1 -v
```

默认输出baseline_hits=5及首命中/距离，明确vector_database=not_run。单元测试PASS，真实数据库测试显示显式E2E跳过；这不是数据库已验证。

## 实际数据库对照

只在隔离的本机测试PostgreSQL中安装上述版本的pgvector，创建vector_lab与专用用户。安装参考[官方固定版本](https://github.com/pgvector/pgvector/tree/v0.8.7)；不要用生产数据库或未核对镜像。源码构建需要正确pg_config和开发头，临时安装与系统安装是不同作用范围。

设置VECTOR_DSN时在本机环境使用私有凭据，不提交、打印或放入聊天。实验启动需要：

```bash
VECTOR_E2E=1 VECTOR_TEST_DATABASE=1 \
  go -C labs test ./part3-agent/chapter28/pgvector-retrieval \
  -run '^TestActualPgvectorExactAndHNSW$' -race -count=1 -v
```

本次实际验证：pgvector0.8.7、PostgreSQL18.6，真实独立临时SCRAM集群、1024合成行；exact_plan_seq=true、ann_plan_hnsw=true、hits=10、recall_vs_exact=1.00。连接/数据/凭据为测试生成，验证后集群停止且临时数据删除。这个固定结果不代表所有语料Recall=1、ANN无损或速度更快。

测试核对Go与SQL精确top-k、ANN相对召回、全部ANN命中Scope、请求取消。SQL临时表在连接结束清理；启动的数据库服务由测试部署者负责停止，不因Go进程退出自动停整台服务。

## 练习

| 任务 | 起点与提示 | 预期观察与边界 |
| --- | --- | --- |
| 候选预算与过滤 | 修改ef_search/iterative_scan，减少合法Scope占比 | 记录命中数与相对Recall；WHERE不会自动保证top-k填满 |
| 距离与索引匹配 | 在副本改变distance operator/index opclass | EXPLAIN必须对应；不能只创建索引就声称查询走ANN |
| 索引更新与来源 | 更换version/space并比较旧资料 | 不混合空间；过滤和来源字段同时检查 |
| 真实Embedding | 输入锁定模型、维度和编码配置的向量，独立标注任务资料 | 区分语义Recall与相对精确Recall，保留原文、版本和真实延迟 |
| 多租户隔离 | 比较共享索引、分区或单租户表 | 输出权限和候选/性能隔离分别验证，不将参数WHERE当作全部安全保障 |

## 来源与许可

原创Go实验，MIT。pgx依赖版本见labs/go.mod，MIT；pgvector0.8.7采用[PostgreSQL License](https://github.com/pgvector/pgvector/blob/v0.8.7/LICENSE)，临时源码/二进制不纳入仓库。[官方文档](https://github.com/pgvector/pgvector/tree/v0.8.7)用于核对operator、HNSW与过滤；未复制实现或图片。
