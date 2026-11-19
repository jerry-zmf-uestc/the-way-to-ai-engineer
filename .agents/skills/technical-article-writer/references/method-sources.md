# 方法来源与项目适配

核对日期：2026-11-11。下面是独立撰写的项目方法整理，不包含上游Skill全文、脚本或模板；本地运行不依赖另行安装这两个Skill。

| 来源 | 固定版本与入口 | 吸收范围 |
| --- | --- | --- |
| Anthropic doc-coauthoring | `2c5958ecac7cee2b583967a2c4981d0f4a75c579`，[入口](https://github.com/anthropics/skills/blob/2c5958ecac7cee2b583967a2c4981d0f4a75c579/skills/doc-coauthoring/SKILL.md) | 收集上下文、按章节迭代、读者问题检验 |
| Jim Liu baoyu-article-illustrator | `ca703c2d7c2c4399a1dc0cf82224f8551e05a96d`，Skill 1.117.4，[入口](https://github.com/JimLiu/baoyu-skills/blob/ca703c2d7c2c4399a1dc0cf82224f8551e05a96d/skills/baoyu-article-illustrator/SKILL.md) | 图位/作用分析、类型×风格×配色、提示词留档、图文插入与实物检查 |

项目适配：按当前请求与已有材料判断是否追问，不搬入强制风格确认、固定多轮问卷、每节配额、额外后端安装或特定平台命令。读者检验不默认启动其他代理；真实体验、演示观察与作者观点明确区分。课程、每日一题与产品观察遵守各自的正文、证据和归档规范。

权利说明：baoyu仓库根LICENSE为MIT；本适配未复制其源文本。此次检查的Anthropic固定版本没有找到根目录或doc-coauthoring目录的许可证文件，因此不推断其许可、不复制或重新许可其全文。后续若引入任何上游原文、模板或程序，应先核实相应许可并保留要求的声明。
