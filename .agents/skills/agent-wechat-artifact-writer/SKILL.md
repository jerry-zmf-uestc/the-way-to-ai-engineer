---
name: agent-wechat-artifact-writer
description: 为本项目 Agent 每日一题制作公众号候选稿、配图与排版预览，或将已确认内容纳入正式稿并在明确授权后保存公众号草稿。适用于 AE 题目制作、旧稿试排和草稿更新。
---

# 公众号内容制作

在本仓库串联研究、写作、技术核验、润色、配图、排版和草稿交付。默认交付可比较的本地候选预览；只执行用户当前请求包含的环节，不把制作或预览授权扩展为覆盖正式稿、写入远端、提交 Git 或部署站点的授权。

## 项目上下文

从本 Skill 所在目录向上三级定位仓库根目录，不依赖当前终端目录。先读根目录 AGENTS.md 与目标路径适用的局部规则，保留已有未提交修改。完整读取 docs/rules/interview-content-contract.md、docs/rules/security.md 和 AGENTS.md 对当前工作指定的其他规范与模板；它们是项目约束的真源，本 Skill 不复制正文模板和审核规则。

content/interview/questions.json 定义题目、路径、前置和最低证据；state.json 管编辑状态；website.json 单独控制网站可见性。按 AE-ID 查记录，不按顺序猜文件，不复用封存 ID。正式正文与素材路径从清单及正文读取，不硬编码第一题路径。

## 根据请求选择模式

| 请求 | 执行与停止条件 |
| --- | --- |
| 制作题目／先给预览 | 确认题目范围与已有素材，按需研究、起草、核验、润色、配图和排版，交付候选预览；不写正式正文或状态。已有授权充分时直接继续，不重复索要确认。 |
| 试排／润色／只换封面 | 只执行指定环节，保存独立候选与差异。图片合格则复用，不为展示工作量重绘。 |
| 采用某一版 | 确定采用的候选及部分，如文案、封面、排版；核对原稿未并发变化后备份并迁入正式路径，重新核对来源与审核，不沿用旧正文摘要。范围不清才询问。 |
| 保存／更新草稿 | 仅在具体候选及远端操作已获授权时，读取 [草稿交付](references/draft-delivery.md) 并执行。仅保存草稿，不登记 published。 |

未指明题号时，可运行 next-work 提出下一题；请求涉及哪篇仍不明确时先定位，不自动制作其他题。需要的选择优先采用项目已确认偏好；只有影响任务且不能推断的信息才询问。

## 候选工作区与产物

从 scripts/interview/workspace.mjs 的 workspaceRoot/workspaceFile 解析私有工作目录并校验路径；尊重 INTERVIEW_WORKSPACE，不绕过符号链接保护。每次在私有目录 artifact-runs/<AE-ID>/<唯一运行标识>/ 建立独立工作区，禁止覆盖另一轮候选。不要为本 Skill 引入第二套正式正文或状态管理。

开始时记录正式正文、状态、可见清单及本次涉及素材的 SHA-256。按实际工作生成 candidate.md、research.md、changes.diff、preview/ 和 run.json；只有出图才生成 prompts/ 与 images/，只有草稿交付才生成 delivery/。初稿不存在时记录为缺失，不创建空白正式文章。

run.json 是私有回执：记录题号、模式、源文件摘要、实际使用的 Skill 及其入口摘要、执行环节、产物相对路径、真实检查结果和待确认事项。确认记录注明本会话中对应的用户授权及内容摘要，不凭空填“已确认”。不保存密钥、cookie、后台会话 token，不作为公共校验依赖，也不替代审核记录。

## 社区 Skill 的复用与交接

本 Skill 是编排入口，社区 Skill 是各阶段的实际执行规范。不能只读下面的能力名称就自行重写一套流程。每次先从运行时技能目录解析本轮需要的入口，完整读取对应 SKILL.md，再按它指定的参考文件、脚本或原生工具执行。具体解析、调用上下文、验收和失败处理见 [社区 Skill 复用契约](references/dependencies.md)，执行某阶段前读取对应条目。

| 阶段 | 实际复用 | 输入 | 输出与下一步 |
| --- | --- | --- | --- |
| 研究／初稿 | content-research-writer | 题库focus、证据要求、原稿或已确认选题、原始来源 | research.md与候选初稿 → 技术核验 |
| 中文润色 | humanizer-zh | 已核验初稿、首篇文风样本、保护项 | candidate.md与changes.diff → 配图或排版 |
| 技术图（按需） | archify | 机制事实、图类型、角色与控制边界 | 原生图源、交付回执与导出图 → image-map.json |
| 封面（按需） | baoyu-cover-image → 原生imagegen | 候选正文、项目EXTEND偏好 | 完整提示词与候选封面 → 素材映射 |
| 公众号排版 | dbs-wechat-html | 最终候选Markdown、选定style id | 行内样式HTML与图片占位 → 编排层生成带图阅读预览 |
| 保存草稿（授权后） | baoyu-post-to-wechat | 确认版HTML、已解析图片、封面和元数据 | 新建草稿回执 → 独立回读核对；更新能力须先实查 |

技术核验、图片映射、HTML交接和授权检查由编排层负责；不能把这些项目补充能力说成社区 Skill 自带功能。不机械调用全部依赖：只换封面就只加载封面相关能力，已有合格正文图则记录复用而不调用Archify。

每阶段向社区能力提供明确任务上下文：本次模式、输入文件、期望输出、项目文风及事实保护项、私有输出目录、已授权范围。不要把整个聊天记录、凭据或无关工作区传给依赖。

“调用”按当前运行时能力实现：有正式技能调用入口就使用；没有时由当前Codex读取其实际说明并按说明执行，过程不需要为每个Skill启动一个子Agent，也不存在通用的 `run-skill` 命令。纯说明型Skill由模型遵循规则完成；带脚本Skill必须实际运行相应入口才能报告脚本执行成功。

在run.json的每个stage记录skill、实际入口摘要、execution（instruction或script或native-tool）、inputs、outputs、status（planned/running/completed/skipped/blocked/failed）、checks及reason；出图记录实际后端，脚本记录退出码。未执行只写planned，读取说明不等于完成产物。下游消费通过检查的具体产物，不能凭“上一步完成”跳过文件及内容核对。

预览阶段结束时复核基线摘要，提供原版对照、主要修改及未验证事项。打开页面不等于完成视觉验收，浏览器策略阻止时如实报告，不换路径绕过访问限制。

## 采用与校验

采用前核对用户确认的候选摘要和正式稿基线；并发修改时先比较，不覆盖。仅将确认部分迁入正式路径，修正相对素材引用；正文变化先按既有 transition 合法退回 drafting，再依实际研究与审核推进。不得自动将 checks 全填 true、冒充真人审阅或把候选回执当 ready 证据。新增正式图遵守图源登记与第三方归属要求。网站可见性不随采用自动改变。

复用根目录现有脚本，不复制状态机：

```sh
node scripts/interview.mjs validate
node scripts/interview.mjs next-work
node scripts/interview.mjs hash AE-ID
node scripts/interview.mjs render
node scripts/interview.mjs delivery-check AE-ID
make validate-docs
```

以上 AE-ID 是题号占位，执行前换成清单中真实题号。只运行适用于改动的检查；纯私有预览检查 HTML、图片、引用及基线摘要，不声称完成正式 delivery-check。正式稿或规则变化按 AGENTS.md 执行相关检查。来源核实、自动检查、实际模型实验、真人审阅和微信后台验收分开记录。

交付简洁说明产物位置、改了什么、实际检查及限制。除非本轮明确要求，不更新公众号草稿、不提交或推送 Git。任何模式均禁止线上发布、群发和定时发布。
