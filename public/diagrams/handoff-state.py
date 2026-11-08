# coding: utf-8
"""Original editable static Handoff lifecycle diagram, standard library only."""
from pathlib import Path
from html import escape
parts=['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 450" role="img" aria-labelledby="title desc"><title id="title">Handoff控制权与执行状态</title><desc id="desc">ready时可原子移交Owner与Revision，running时拒绝移交；执行验证成功返回ready，取消或失败进入终态。Scope、任务和共享账本不改变。</desc><defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="960" height="450" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">''']
def text(x,y,s,size=16):parts.append(f'<text x="{x}" y="{y}" text-anchor="middle" font-size="{size}">{escape(s)}</text>')
def box(x,y,w,lines,color='#fff'):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="75" rx="10" fill="{color}" stroke="#64748b"/>')
    for n,s in enumerate(lines):text(x+w/2,y+30+n*23,s)
def arrow(d):parts.append(f'<path d="{d}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>')
text(480,35,'控制权移交改变Owner，任务身份与资源归属保持一致',22)
box(35,90,220,['ready / 当前Owner','等待执行或移交'],'#dbeafe')
box(375,90,210,['running','领取轮次 / Revision+1'],'#fef3c7')
box(700,90,225,['ready / 新Owner','移交后Revision+1'],'#dbeafe')
arrow('M255 115H375');text(315,96,'执行领取',14)
arrow('M375 150H255');text(315,177,'结果验证',14)
arrow('M145 90V65H812V90');text(480,58,'批准边 + Scope / Owner / Revision校验',14)
arrow('M700 140H655V210H480V165');text(660,190,'新Owner执行',14)
box(350,285,250,['failed / canceled','迟到或未验证事实不提交'],'#fee2e2')
box(35,285,220,['completed','显式结束 / 禁止再接管'],'#dcfce7')
arrow('M430 165V285');text(525,245,'错误 / 取消',14)
arrow('M240 165V250H330V322H350');text(280,275,'轮次耗尽',14)
arrow('M145 165V285');text(203,238,'显式完成',14)
text(770,282,'running期间拒绝移交与重复领取',15)
text(770,313,'Hops限制移交，Turns限制执行',15)
text(480,410,'任务ID / Scope / 已验证事实 / 共享工具账本不随Owner重置',17)
text(480,438,'可信身份不由ID/Owner自报；单实例内存控制，不证明跨进程唯一Owner',15)
parts.append('</g></svg>')
Path(__file__).with_suffix('.svg').write_text(''.join(parts),encoding='utf-8')

# Separate commit windows from the in-memory ownership lifecycle above.
parts = ['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1160 570" role="img" aria-labelledby="title desc"><title id="title">Handoff的三处提交窗口</title><desc id="desc">执行意图先保存；工具调用前保存预算预留，工具返回后保存有界内容，角色校验后提交结果与静止快照。三处退出窗口分别保留预留但未知返回、已知返回但无角色提交、已提交角色结果但通知可能丢失；结果记录不自动证明任务完成。</desc><defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1160" height="570" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">''']

def commit_box(x, title, line1, line2, color):
    parts.append(f'<rect x="{x}" y="105" width="230" height="105" rx="10" fill="{color}" stroke="#64748b"/>')
    text(x + 115, 136, title, 18)
    text(x + 115, 166, line1, 15)
    text(x + 115, 192, line2, 15)

text(580, 35, '预算预留、工具返回与角色提交是三份不同证据', 24)
text(580, 68, '同一私有记录 / 排他写者持有整个读取、执行与保存过程', 16)
commit_box(25, '执行意图', 'phase=executing', '先保存，再进入角色', '#e2e8f0')
commit_box(315, '预算预留', 'Outcome=reserved', 'Used + 尝试号 / 角色 / 工具', '#fef3c7')
commit_box(605, '工具返回', 'returned / error / rejected', '有界 Raw / 稳定 ErrorCode', '#dbeafe')
commit_box(895, '角色结果与快照', 'RoleResults + Stable', 'phase=stable，仍需任务验收', '#dcfce7')
for x in (255, 545, 835):
    arrow(f'M{x} 156H{x + 60}')
text(285, 235, '调用前', 14)
text(575, 235, '返回后', 14)
text(865, 235, '校验后', 14)

rows = [
    ('窗口 A：预留后退出', '已确定：预算登记已提交', '未确定：工具是否返回、实际费用', '#fef3c7'),
    ('窗口 B：返回后退出', '已确定：有界返回记录已提交', '未确定：角色结果；不能自动重复读取', '#dbeafe'),
    ('窗口 C：提交后失联', '已确定：角色结果与版本已提交', '未确定：通知送达；先查询可信记录', '#dcfce7'),
]
for n, (label, known, unknown, color) in enumerate(rows):
    y = 275 + n * 72
    parts.append(f'<rect x="25" y="{y}" width="1100" height="58" rx="8" fill="{color}" stroke="#94a3b8"/>')
    text(170, y + 35, label, 17)
    text(490, y + 35, known, 16)
    text(900, y + 35, unknown, 15)
text(580, 525, 'error / rejected 不产生成功观察；保存确认失败不交付内存结果', 17)
text(580, 551, '当前入口拒绝未知重做；租约、显式恢复与真实费用仍需另行验证', 16)
parts.append('</g></svg>')
Path(__file__).with_name('handoff-commit-windows.svg').write_text(''.join(parts), encoding='utf-8')
