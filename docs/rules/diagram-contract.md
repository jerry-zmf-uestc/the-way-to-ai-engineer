# 图示与正式素材规范

课程图的现有Python/SVG源与重建登记继续使用public/diagrams/sources.json，不改变其字节重建、安全结构与替代文字检查。面试图登记在public/diagrams/interview/sources.json；明确来源格式、源文件、正式SVG/PNG与摘要。不同渲染链分别验证，不将Archify冒充可由现有Python生成器重建。

公开图源和正式图必须可定位；默认离线检查文件集与摘要、SVG静态边界及正文图片引用。摘要只证明登记产物一致，不证明源语义或可重建；Archify的重新生成与视觉验收仍需显式工具环境，不能以摘要校验替代。运行回执、原生HTML、预览与字体/渲染临时材料保存在仓库外。

## 面试图风格

简短单标题、简体中文、白底与浅色语义分组，必要术语保留英文。去掉AGENT ENGINEERING、题号、ARCHIFY眉题和底部教学说明，不加渐变、阴影、网格及装饰图标。图例与节点含义重复时隐藏，颜色不作为唯一信息载体。

职责泳道与主流程清晰，反馈/重规划独立回路；保留权限、预算、拒绝、验收与退出，不能为简洁删分支。标题简短，正文解释适用边界，图示与真实Trace分开。裁切不得截断箭头、标签和节点。

使用Archify时读取已安装Skill，保留上游内容，仅按本规范定制；默认classic、meta.legend.mode为hidden（无额外信息时）、icon为none，省略不必要subtitle/viewBox，优先自动布局。每次工作用独立私有.archify目录，遵循finalize及实际视觉审查。浏览器策略禁止时不绕过，记录未执行项，交付PNG实际查看。非原创材料保留来源、许可和归属。

第一题正式素材在public/diagrams/interview/ae-001，JSON源在sources子目录；封面在public/images/interview/ae-001。当前正式版本已由用户确认，迁移只校对图片字节，不重画或更新公众号。重新生成可使用公开JSON交给Archify在私有工作目录交付，正式替换须审查后更新登记和摘要。

## 既有技术图与原生结构

技术图采用青绿、浅青绿与白底表达角色和分组，不把颜色作为唯一标识。迁入既有位图不重绘，保留来源及原摘要；有历史样式差异时标明，不借迁移覆盖原件。`frozen-static-v1`来源JSON必须含schema_version=1、kind=frozen-static、title、provenance、original_sha256和rebuildable=false，只关联一张静态PNG/JPEG/SVG；检查不证明当前微信版本或可重建。Archify既有契约保持不变。

请求位置、数据形态不是流程图时可采用正文内原生结构。安全白名单仅允许section/p/mark/br及request-diagram/request-group/request-highlight/request-label/request-code类，不接受脚本、事件、任意样式、外链或平台DOM属性。文本中的JSON和尖括号转义；代码与工具字段只作明确示意，不暗示固定role/位置。原生结构保存在唯一正文中，渠道预览共用，不另存脱离正文的图事实。
