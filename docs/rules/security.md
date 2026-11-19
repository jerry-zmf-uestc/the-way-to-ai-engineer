# 安全、变量与外部工具规范

## Secrets

- API Key、token、真实用户 ID、真实文档 URL 只存在服务端本地变量或秘密管理系统。
- `.env.example` 只能包含无效占位符。
- 日志、错误、测试快照、prompt 和站点不得包含秘密。

## External data

- 地图、天气、搜索和 LLM Provider 必须设置超时、响应大小和调用预算。
- 所有外部输入均不可信，包括网页内容、MCP 响应和模型生成 JSON。
- Provider host 使用 allowlist；Agent 不接受任意 URL 作为工具目标。

## Agent safety

- 工具权限由应用注册表决定，不由 prompt 决定。
- 结构化参数必须在执行工具前校验。
- Agent 默认只读；写数据库由应用层校验和执行。
- 当前禁止预订、支付、发送消息和其他不可逆工具。
- Memory 持久化需要明确数据范围和用户授权。

### 必须覆盖的 Agent 测试

- 固定模型输出、工具响应和评测数据集，避免将概率波动当作回归。
- 未知工具必须拒绝。
- 工具参数 schema 错误不得执行调用。
- 循环、token、工具调用和响应大小必须有上限。
- 工具失败不得被模型结果掩盖。
- 非工具来源的地点不得通过事实校验。
- 用户 memory 必须隔离，未授权偏好不得持久化。
- Prompt injection 不得扩大工具权限。

## Repository safety

- 迁移前扫描源仓未提交修改和敏感文件。
- 不提交生成站点、二进制、profile、runtime state。
- 外部内容必须记录来源、revision 和 license。
- 安全扫描缺工具、执行异常或数据库不可用必须报告失败；不得静默跳过。默认 validate 离线，security 查询漏洞库时明确联网，工具版本固定在 scripts/security-tools.json。
- GitHub Actions 默认 contents: read；第三方 Action 固定完整提交 SHA；不通过特权 PR 事件执行外部代码，不给默认测试注入真实 Provider 密钥。
- 公开漏洞报告渠道与受支持范围见 [SECURITY.md](../../SECURITY.md)。这是对外入口，本文件只维护实现约束；新增或改变入口时两者同步。

## Incident baseline

发现泄漏时停止相关 Provider、轮换凭据、清理 Git 历史前先保留审计证据，并检查日志、CI artifact 和生成站点。具体轮换责任将在 Provider 接入 spec 中补齐。

<a id="variables"></a>

## 变量规划与凭据管理

下表是未来独立应用接入外部能力时的变量规划，不是现有 Lab 的配置清单。各 Lab 的实际配置以源码和 README 为准；规划变量仅在对应 Provider spec 批准后引入，名称可在该 spec 中调整。

| Name | Used by | Scope | Source | Rotation | Risk |
| --- | --- | --- | --- | --- | --- |
| `LLM_API_KEY` | Agent service | Server | Local env / secret manager | Provider policy | 模型费用与数据访问 |
| `LLM_BASE_URL` | Agent service | Server | Config | On provider change | SSRF/错误 Provider |
| `MAP_API_KEY` | Map adapter | Server | Local env / secret manager | Provider policy | 配额与费用 |
| `WEATHER_API_KEY` | Weather adapter | Server | Local env / secret manager | Provider policy | 配额与费用 |
| `MYSQL_DSN` | Trip service | Server | Local env / secret manager | Credential rotation | 数据读写权限 |
| `REDIS_ADDR` | Backend services | Server | Config | Infrastructure change | 拓扑泄漏 |
| `REDIS_PASSWORD` | Backend services | Server | Secret manager | Credential rotation | 缓存和会话访问 |
| `E2E_LIVE` | Tests | Server/CI | Explicit test env | N/A | 意外触发真实费用 |

### 配置约束

- 客户端 bundle 不得包含任何 Provider 或数据库秘密。
- 前端只能获得服务端签发的公开配置，例如地图 JS SDK 确实需要的受域名限制 key；引入前必须单独评审。
- 配置对象的日志输出必须屏蔽秘密字段。
- `.env` 被 Git 忽略，仓库只保留 `.env.example`。

### 接入真实服务前的检查

- [ ] 所有秘密来自部署环境或 secret manager。
- [ ] 客户端构建产物扫描无服务端秘密。
- [ ] Provider key 配置最小权限、配额和来源限制。
- [ ] 日志和 Trace 屏蔽请求中的 PII/secret。
- [ ] E2E 有费用上限且不作为默认 CI。
- [ ] 轮换负责人和撤销步骤已记录。
