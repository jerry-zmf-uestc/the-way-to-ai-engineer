"""Original editable diagrams. Rebuild with Python 3, standard library only:
python3 public/diagrams/harness-foundation.py
Static SVG has no script, font download or remote resources.
"""
from pathlib import Path
from html import escape


def diagram(name, title, subtitle, stages, notes):
    height = 118 + len(stages) * 110 + len(notes) * 28 + 30
    parts = [f'<svg xmlns="http://www.w3.org/2000/svg" width="900" height="{height}" viewBox="0 0 900 {height}" role="img" aria-labelledby="title desc">',
             f'<title id="title">{escape(title)}</title><desc id="desc">{escape(subtitle)}</desc>',
             '<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L10 5 L0 10Z" fill="#475569"/></marker></defs>',
             f'<rect width="900" height="{height}" rx="16" fill="#f8fafc"/>', '<g font-family="system-ui,sans-serif" fill="#0f172a">']
    def text(x,y,value,size=17):
        parts.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')
    text(30,40,title,24);text(30,72,subtitle,16)
    for i,(heading,body) in enumerate(stages):
        y=100+i*110
        parts.append(f'<rect x="55" y="{y}" width="790" height="80" rx="10" fill="{"#dbeafe" if i%2==0 else "#ffffff"}" stroke="#64748b"/>')
        text(80,y+29,heading,20);text(80,y+58,body,16)
        if i<len(stages)-1:
            parts.append(f'<path d="M450 {y+80} V{y+104}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#arrow)"/>')
            if i == 2 and name == 'runtime-lifecycle':
                text(475, y+101, '暂时错误且可重试', 14)
            if i == 2 and name == 'session-version':
                text(475, y+101, '版本冲突', 14)
    for i,note in enumerate(notes):text(30,118+len(stages)*110+i*28,note,16)
    parts.append('</g></svg>\n')
    Path(__file__).with_name(name+'.svg').write_text('\n'.join(parts),encoding='utf-8')

def harness_overview():
    parts = ['<svg xmlns="http://www.w3.org/2000/svg" width="900" height="780" viewBox="0 0 900 780" role="img" aria-labelledby="title desc"><title id="title">Harness职责与决策回路</title><desc id="desc">模型响应分为工具提议与最终文本。工具观察回到输入，最终文本返回后仍需业务验收。Runtime管理每轮预算、取消与提交。</desc><defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="900" height="780" rx="16" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
    def text(x, y, value, size=17):
        parts.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')
    def box(x, y, w, lines, color='#fff'):
        parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="80" rx="10" fill="{color}" stroke="#64748b"/>')
        for n, line in enumerate(lines):
            text(x+18, y+31+n*27, line)
    def arrow(path):
        parts.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>')
    text(30, 40, 'Agent Harness：职责与决策回路', 24)
    text(30, 72, '箭头表示输入、响应与观察的流向；工具提议与最终文本分别处理。', 16)
    parts.append('<rect x="30" y="94" width="840" height="46" rx="10" fill="#eef2ff" stroke="#64748b" stroke-dasharray="6 5"/>')
    text(48, 123, 'Runtime：在每轮边界检查预算、取消与提交；虚线框表示贯穿约束。', 16)
    box(110, 180, 490, ['Context / Session → Model', '选择本轮输入；Provider适配与响应检查'], '#dbeafe')
    box(110, 310, 490, ['Agent Loop：解释响应分支', '有工具提议：执行；没有提议：检查Final'])
    box(650, 310, 220, ['无工具且Final非空', '返回文本，再验任务'], '#dcfce7')
    box(110, 440, 490, ['Tool → Environment', '查找能力、校验与授权；受控执行'])
    box(110, 570, 490, ['观察回填 → 下一轮输入', '按调用ID关联结果，保留错误与来源'], '#dbeafe')
    arrow('M355 260V310')
    arrow('M355 390V440')
    text(370, 420, '工具提议', 15)
    arrow('M355 520V570')
    arrow('M600 350H650')
    arrow('M110 610H55V220H110')
    text(110, 690, '观察进入下一轮；取消、协议失败或预算耗尽则终止。', 16)
    text(30, 729, '最终文本不证明任务完成；授权与事实支持由应用检查。', 16)
    text(30, 757, 'Memory保存授权长期信息，Checkpoint保存执行进度；按需引入。', 16)
    parts.append('</g></svg>\n')
    Path(__file__).with_name('harness-overview.svg').write_text('\n'.join(parts), encoding='utf-8')


harness_overview()
diagram('model-contract','Model：输入、协议与业务校验','各层成功只证明本层契约，不能替后续判断。',[
 ('应用输入 → Provider请求','消息与工具说明；context、大小与输出上限'),
 ('HTTP响应 → 状态分类','请求失败 / 非成功状态 / 成功响应；错误体未必是JSON'),
 ('协议响应 → 程序结果','解码、响应分支、空结果与截断检查；成功响应保留usage'),
 ('业务消费 → 效果判断','字段校验 → 引用与事实校验 → 任务评估'),
],['文本生成与工具提议需要不同的响应分支。','失败或取消可能缺少usage，不能据此按零费用结算。','fake验证协议；真实模型任务集评估效果。'])
diagram('tool-boundary','Tool：从提议到受控执行','每层拒绝都会结束本次调用，不能绕过校验进入Handler。',[
 ('模型提议：调用ID / 名字 / 参数','内容不可信；工具说明不赋予执行权限'),
 ('应用注册表：查找与校验','未知工具拒绝；必填、类型、额外字段检查'),
 ('可信业务上下文：授权与配额','实验使用启动时只读能力清单；真实系统按身份检查范围'),
 ('Handler：执行 → 关联观察','context传入下游；错误与结果保留身份；取消后不提交迟到文本'),
],['参数形状合法仍可能业务失败；错误不是空结果。','普通错误可反馈；取消不能撤销已经发生的动作。','本Lab演示静态清单与参数校验；动态授权、配额由应用扩展。'])
diagram('runtime-lifecycle','Runtime：状态与每次尝试的边界','合法运行进入Running；非法定义或预取消可从Pending直接终止。',[
 ('进入运行：验证策略与动作','非法定义 → Failed；已取消 → Canceled；有效 → Running'),
 ('尝试前：取消与预算检查','模型 / 工具次数分别预留；失败与重试仍计次数'),
 ('执行返回：登记消耗，再判断结果','取消 / 超限 / 永久错误 → 终止；成功 → 接受结果'),
 ('暂时错误 + RetrySafe：有限退避','等待可取消；有剩余尝试与总配额才回到尝试前'),
],['全部动作提交 → Completed；过程失败仍保留已知消耗与部分结果。','返回后统计无法阻止本次超额；内存事件不等于持久恢复。'])

diagram('session-version','Session：快照、竞争与生命周期','读取和提交都使用可信身份范围；模型与工具调用发生在锁外。',[
 ('读取：两个请求获得版本 v','每个调用方持有独立消息与参数快照'),
 ('计算：基于同一版本产生追加批次','结果可能过时；版本检查不能防止重复外部执行'),
 ('原子提交：检查 expected == current','一个批次更新为新修订号；另一批次收到冲突'),
 ('冲突处理：重新读取，再判断适用性','提交、重规划或终止；不能盲目重做工具'),
],['过期后同ID重建必须使用新代次或修订号，旧请求不能重新有效。','内存Store不提供跨进程持久化或Checkpoint。'])
diagram('context-selection','Context：构建本轮输入视图','原始历史保持不变；估算、协议完整性与任务效果分别验证。',[
 ('验证历史：角色与调用ID','下一轮模型输入需要完整的工具提议和观察组'),
 ('保护必要输入：规则 + 当前用户轮','Window - OutputReserve是简化输入预算；不够则失败'),
 ('选择旧轮：完整候选逐次计数','保留时间顺序，不拆开调用关联；省略不等于删除'),
 ('交付输入：消息快照 + 选择信息','真实计数核对容量，任务评估核对关键信息是否足够'),
],['旧约束可能比新文本重要，需显式保护并检查来源。','摘要与卸载另有信息损失、权限和生命周期，不在本选择器中实现。'])

diagram('retrieval-pipeline','Retrieval：索引与查询分离','知识结构总览；范围、版本与物理扫描顺序按各后端契约核对。',[
 ('索引准备：解析 → 切分 → 向量化 → 发布版本','保存片段ID、来源、权限、Space与原文；更新版本不能悄悄复用ID'),
 ('在线查询：可信范围与版本过滤','查询向量必须匹配文档空间；非法输入与空结果分别处理'),
 ('两路召回：词法排名 + 向量排名','每路candidateK限制候选；原始分数尺度不直接相加'),
 ('融合 → 可选重排 → 最终topK','保留路线排名与来源；输入长度还需由Context限制'),
],['基础Search先按Tenant过滤；版本选择与后端ANN扫描分别设计。','固定向量验证算法，真实编码另验效果；命中不证明回答被支持。'])
diagram('eval-pipeline','Eval：执行事实与评分判断分开','未知、失败与缺失保留在报告中，不能绕过验收。',[
 ('任务与环境：版本化Case、资料和权限','development / regression / adversarial / production-like分别组织用途'),
 ('Runner：身份、截止时间与执行记录','工具失败、取消和usage不由模型自报；全部Case保留'),
 ('Scorer：规则 + 独立支持标签或Judge','passed / failed / review分开；评分器也需要反例和校准'),
 ('报告与比较：分母、覆盖、配对退化与消耗','同任务与可比配置比较；新增复杂度需要收益证据'),
],['合成夹具验证评分管道，不是模型质量基准。','真实效果仍需真实任务、重复试验及真人校准，少量成功不能证明稳定提升。'])

diagram('checkpoint-window','Checkpoint：外部动作与本地提交窗口','in_flight只说明准备调用；中断后可能不知道外部动作是否已经完成。',[
 ('先校验请求并领取；再校验存储状态与指纹','已有锁不自动偷取；预算来自持久状态'),
 ('持久记录：Attempts + in_flight','此处退出可能尚未发出工具请求'),
 ('外部动作返回：结果还在内存','此处退出留下结果未知窗口；只读重做也会重复调用'),
 ('原子提交：结果 + 新游标','提交后退出从下一项继续；同步错误需按未知结果处理'),
],['本实验验证进程硬退出，不承诺断电或分布式exactly-once。','Session保存历史，Checkpoint保存恢复位置，Trace帮助调查，职责分开。'])
def trace_recording():
    parts = ['<svg xmlns="http://www.w3.org/2000/svg" width="1000" height="680" viewBox="0 0 1000 680" role="img" aria-labelledby="title desc"><title id="title">Trace与Replay：事件和响应档案分别记录</title><desc id="desc">同一调用分别投影诊断事件和私有响应档案。Replay只消费校验后的响应档案，按类型、名字、输入指纹和顺序匹配，不调用实时工具。</desc><defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1000" height="680" rx="16" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
    def text(x, y, value, size=16):
        parts.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')
    def box(x, y, w, lines, color='#fff'):
        parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="10" fill="{color}" stroke="#64748b"/>')
        for n, line in enumerate(lines):
            text(x+18, y+31+n*27, line, 18 if n==0 else 16)
    def arrow(path):
        parts.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>')
    text(30, 40, 'Trace与Replay：事件和响应档案分别记录', 24)
    text(30, 74, '箭头表示记录内容与消费关系，不表示跨进程观测平台的完整实现。')
    box(250, 110, 500, ['调用边界：Run / Step / Call', '开始、实际返回与稳定错误；计数口径明确'], '#dbeafe')
    box(40, 290, 430, ['诊断Event：关联、序号与指纹', '不记录原prompt、响应或内部错误文本'])
    box(530, 290, 430, ['私有Archive：请求指纹与录制响应', '错误保留类别；失败响应置空'], '#fef3c7')
    arrow('M370 200V240H255V290')
    arrow('M630 200V240H745V290')
    box(40, 450, 430, ['定位调用与执行路径', '指纹不是匿名化；事件也需要隐私判断'])
    box(530, 450, 430, ['Load校验 → Replay逐项匹配', 'Kind / Name / InputHash / 顺序', '返回录制值或错误，不执行live'], '#dbeafe')
    arrow('M255 380V450')
    arrow('M745 380V450')
    text(30, 590, '本Lab为串行内存录制；载荷授权、加密与保留策略须另行实现。')
    text(30, 621, '缺记录、错请求或顺序不匹配均失败；不访问真实服务补齐。')
    text(30, 652, 'Replay不是Checkpoint恢复；生产Span传播、采集与可靠审计另有契约。')
    parts.append('</g></svg>\n')
    Path(__file__).with_name('trace-recording.svg').write_text('\n'.join(parts), encoding='utf-8')


trace_recording()
diagram('eino-comparison','Eino：相同契约的两种组合','直接调用与真实compose.Chain使用同一业务函数，不用框架名代替效果判断。',[
 ('输入校验：Input → 合法请求','空白或超大输入在查询前拒绝，context进入每个阶段'),
 ('资料查询：请求 → Evidence','Direct直接调用；Chain在retrieve节点调用相同Lookup'),
 ('回答构造：Evidence → Answer','有来源的非空固定证据；不是自由生成事实判断'),
 ('对照结果：文本、来源、错误原因与取消','Compile验证组合；Invoke仍需业务和任务检查'),
],['框架不自动提供权限、总重试预算、Checkpoint或完整Trace。','固定工作流与动态Loop取决于控制权；锁定版本后验证实际接口。'])
diagram('mcp-lifecycle','MCP：协商、发现与授权分开','Host选择可信Server与批准范围；模型不能自己连接任意地址或启动程序。',[
 ('stdio建立：可信本地子进程','stdin/stdout仅JSON-RPC；日志stderr；请求与响应有界'),
 ('协商：initialize → 版本/能力 → initialized','不支持版本或缺tools能力拒绝，准备完成才操作'),
 ('发现：tools/list → Schema检查 → 本地批准','新工具不自动授权；namespace是本地适配约定'),
 ('调用：tools/call → RPC或工具结果 → 观察','isError不是成功资料；取消结束等待，不保证远端回滚'),
],['固定协议模式不加载模型；真实路由另验效果，不承诺完整SDK/HTTP/OAuth。','协议测试后适配Tool/Loop，再独立评估任务效果与费用。'])

diagram('skills-loading','Skills：发现、激活与资源分开','能力包提供知识，不自动新增Handler、文件根或工具批准。',[
 ('目录元信息：名称、描述、版本','有界解析与格式检查；发现不等于激活'),
 ('任务选择：规则或模型 → 本地批准','教学词重合只验证路由边界，不证明真实语义效果'),
 ('激活正文：快照、摘要与加载预算','失败不提交半份指令；重复激活不重复扣预算'),
 ('按需资源：登记路径与内容摘要','已激活包的references内读取；不递归执行脚本'),
],['工具权限由应用清单强制执行，正文和allowed-tools不能扩大能力。','版本、资源、输入预算与任务效果分别检查。'])
diagram('eino-react-steps','Eino ReAct：图步骤与模型轮次','一次只读工具调用：三个图步骤包含两次模型调用，END不另算模型轮次。',[
 ('图步骤 1：chat / 模型轮次 1','assistant提议lookup(c1)；有工具调用时分支进入tools'),
 ('图步骤 2：tools / 应用执行','校验与授权 → 读取原文 → tool(c1)观察回填'),
 ('图步骤 3：chat / 模型轮次 2','模型收到配对观察 → 非工具响应 → END → 应用检查交付'),
],['MaxStep=2会阻止第三个图步骤，不能撤销第二步的读取。','这里是固定响应夹具；框架真实执行不证明模型推理效果。','Handler返回错误时默认图中断，错误反馈与修复由应用选择。'])


diagram('strategy-candidate-selection','一轮受限动作选择：接口与任务分开验证','候选来自已观察资料；模型提供排序，应用保留执行与交付检查。',[
 ('已读事实 / 已发现ID / attempted','排除未发现与已尝试ID，构造合法动作或初始计划候选'),
 ('实际模型前向：条件log probability','每候选记录Token与均值分数；不是自由生成，也不是正确率'),
 ('应用执行：预算 / 来源 / 观察更新','选中的合法动作仍可能拒答或证据不足；读取进入下一轮观察'),
 ('独立任务评分：覆盖 / 拒答 / 完成','语法合法、事实已齐与任务通过分别报告，保留失败分母'),
],['初始计划仅执行当时的候选，后续新链接需要另行重规划。','一个决策可包含多次前向，不能按生成JSON长度虚构usage。'])
