"""Original responsibility maps for six layers and governance evidence."""
from html import escape
from pathlib import Path


def canvas(title, description, height):
    return [f'<svg xmlns="http://www.w3.org/2000/svg" width="1120" height="{height}" viewBox="0 0 1120 {height}" role="img" aria-labelledby="title desc">',
            f'<title id="title">{escape(title)}</title><desc id="desc">{escape(description)}</desc>',
            '<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#23736b"/></marker></defs>',
            f'<rect width="1120" height="{height}" fill="#ffffff"/><g font-family="system-ui,sans-serif" fill="#163f3a">',
            f'<text x="32" y="45" font-size="27">{escape(title)}</text>']


def text(p, x, y, value, size=18):
    p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')


def box(p, x, y, width, height, title, lines, color='#e8f5f1'):
    p.append(f'<rect x="{x}" y="{y}" width="{width}" height="{height}" rx="10" fill="{color}" stroke="#60968e"/>')
    text(p, x+16, y+30, title, 21)
    for i, line in enumerate(lines):
        text(p, x+16, y+59+i*27, line, 17)


def arrow(p, d):
    p.append(f'<path d="{d}" fill="none" stroke="#23736b" stroke-width="2" marker-end="url(#arrow)"/>')


def save(p, name):
    Path(__file__).with_name(name+'.svg').write_text('\n'.join(p+['</g></svg>\n']), encoding='utf-8')


p = canvas('Agent六层：职责与最终检查点', '六层是职责视角，不是六个服务或串行调用阶段。控制配置交给Harness组装，Runtime推进动作，Environment落实资源边界，Data/Resource提供有身份和版本的数据；Observability、Eval、Security横切全部边界。', 870)
text(p, 32, 82, '按职责定位代码；框内是拥有的决定或状态，不代表固定部署方式。')
rows = [
    ('Product / Interface', ['用户目标、请求接纳、确认与取消意图；页面显示实际任务状态']),
    ('Control Plane', ['可信定义、授权范围、配额与部件版本 → 解析本次运行配置']),
    ('Harness', ['按配置组装Provider、Context、工具、Skills、存储与观测接口']),
    ('Agent Runtime', ['Run / Step / Attempt、计划、预算、动作推进与提交事实']),
    ('Execution Environment', ['实际进程、文件、网络与CPU/内存限制；确认退出并清理']),
    ('Data / Resource', ['Session、Checkpoint、Memory、索引、Workspace及录制归档']),
]
for i, (title, lines) in enumerate(rows):
    box(p, 32, 110+i*108, 808, 88, title, lines)
box(p, 872, 110, 216, 628, '横切治理', [], '#f0f8f6')
for y, title, line1, line2 in [
    (182, 'Observability', '关联实际运行', '记录失败与不完整'),
    (350, 'Eval', '独立标准与证据', '验证行为及交付'),
    (518, 'Security', '当前授权与隔离', '检查数据和动作出口'),
]:
    text(p, 888, y, title, 20)
    text(p, 888, y+35, line1, 17)
    text(p, 888, y+64, line2, 17)
arrow(p, 'M436 306V326')
text(p, 450, 321, '运行配置', 13)
arrow(p, 'M436 414V434')
text(p, 450, 429, '本次Run的组件', 13)
arrow(p, 'M436 522V542')
text(p, 450, 537, '受限执行请求', 13)
text(p, 32, 778, '数据与资源供相关层按身份、版本和有效期读写，不是执行链的最后一个函数。')
text(p, 32, 814, '控制路径决定允许怎样运行；观察与资料进入决策输入，不能反向成为授权。')
text(p, 32, 848, '任务结束、进程退出、结果提交与业务验收分别取证。', 17)
save(p, 'agent-six-layer-boundaries')

p = canvas('治理：同一运行证据，承担不同判断', '固定任务与资产后产生一次受控运行，保存执行证据。Testing检查契约，Evaluation按独立参照评价质量与消耗，Acceptance检查当前候选交付资格，可信发布策略综合判断。三类检查可独立组合，不代表已实现统一模拟或发布平台。', 760)
text(p, 32, 82, '职责关系示意：检查可以组合，Simulation不是所有测试或Eval的前置步骤。')
box(p, 32, 120, 300, 120, '任务与资产', ['目标 / 用途 / 场景 / 版本', '允许能力与原预算'])
box(p, 410, 120, 300, 120, '一次受控运行', ['实际输入 → 提议 → 工具观察', '状态、结果与已知/未知消耗'])
box(p, 788, 120, 300, 120, '运行证据', ['Case / Run / Call关联', '原始输出、终态与资产版本'])
arrow(p, 'M332 180H410')
arrow(p, 'M710 180H788')
# One evidence branch supplies three distinct responsibilities.
p.append('<path d="M938 240V300H182" fill="none" stroke="#23736b" stroke-width="2"/>')
for x in (182, 560, 938):
    arrow(p, f'M{x} 300V365')
text(p, 32, 278, '同一证据保持关联；评分标签和独立参照不进入被测模型输入。', 17)
box(p, 32, 365, 300, 147, 'Testing：契约是否成立', ['参数、授权、状态与竞争窗口', '正常和失败路径分别检查', '通过不等于模型任务有效'])
box(p, 410, 365, 300, 147, 'Evaluation：质量怎样', ['运行结果 + 独立标准/参照', '支持、完整性、费用与不确定', '评分器自身需要校准'])
box(p, 788, 365, 300, 147, 'Acceptance：能否交付', ['当前目标 / 候选 / 政策版本', '结果与判定绑定，失败不释放', '当前通过不替代独立回归'])
p.append('<path d="M182 512V560H938V512M560 512V560" fill="none" stroke="#23736b" stroke-width="2"/>')
arrow(p, 'M560 560V605')
box(p, 32, 605, 1056, 92, '可信发布与处置', ['综合质量、安全、兼容性与资源证据；按授权启用、拒绝、降级或回退，失败沉淀为有用途记录的回归案例'], '#f0f8f6')
text(p, 32, 736, '图示不表示本仓库已有统一用户模拟器、跨存储审计平台或自动发布服务。', 17)
save(p, 'governance-evidence-decisions')
