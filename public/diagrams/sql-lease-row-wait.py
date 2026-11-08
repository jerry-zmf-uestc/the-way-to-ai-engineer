# coding: utf-8
"""Original comparison of the observed unchanged-row lock wait, standard library only."""
from html import escape
from pathlib import Path

parts = ['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1120 710" role="img" aria-labelledby="title desc"><title id="title">行锁等待跨越租约期限</title><desc id="desc">左右两种流程都先由另一连接持有目标行锁，提交请求等待，数据库时间越过租约期限，持锁事务不修改行并释放。旧UPDATE先判断期限，解除等待后仍接受；修复流程先锁定，再判断到期并拒绝更新。</desc><defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1120" height="710" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">''']

def text(x, y, value, size=15, color='#0f172a'):
    parts.append(f'<text x="{x}" y="{y}" text-anchor="middle" font-size="{size}" fill="{color}">{escape(value)}</text>')

def box(x, y, width, lines, color='#fff'):
    height = 34 + (len(lines) - 1) * 23
    parts.append(f'<rect x="{x}" y="{y}" width="{width}" height="{height}" rx="8" fill="{color}" stroke="#94a3b8"/>')
    for n, line in enumerate(lines):
        text(x + width / 2, y + 23 + n * 23, line)

def line(x1, y1, x2, y2, arrow=False, dashed=False):
    extra = ' marker-end="url(#arrow)"' if arrow else ''
    extra += ' stroke-dasharray="5 5"' if dashed else ''
    parts.append(f'<path d="M{x1} {y1}L{x2} {y2}" fill="none" stroke="#475569" stroke-width="2"{extra}/>')

text(560, 36, '同样等待到期，检查发生的位置决定能否提交', 24)
text(560, 65, '受控反例：持锁事务释放时未修改目标行；时间向下推进', 16)
for left, title, holder, submit in [(70, '旧流程：UPDATE先判断期限', 185, 410), (600, '修复：先锁定，再判断期限', 715, 940)]:
    parts.append(f'<rect x="{left}" y="86" width="450" height="565" rx="12" fill="#fff" stroke="#cbd5e1"/>')
    text(left + 225, 116, title, 19)
    text(holder, 150, '持锁连接', 16)
    text(submit, 150, '提交连接', 16)
    line(holder, 163, holder, 614, dashed=True)
    line(submit, 163, submit, 614, dashed=True)
    box(holder - 72, 180, 144, ['持有目标行锁'], '#e2e8f0')
    line(submit, 300, holder, 300, arrow=True)
    text((holder + submit) / 2, 289, '请求锁，进入等待', 14)
    parts.append(f'<rect x="{submit-7}" y="312" width="14" height="180" fill="#dbeafe" stroke="#64748b"/>')
    text(submit - 76, 378, '等待行锁', 14)
    box(holder - 72, 492, 144, ['释放，行未修改'], '#e2e8f0')
    line(holder, 535, submit, 535, arrow=True)
    text((holder + submit) / 2, 523, '等待结束', 14)

box(325, 231, 170, ['期限尚有效：条件通过'], '#fef3c7')
box(855, 231, 170, ['锁定CTE先执行'], '#dbeafe')
parts.append('<rect x="81" y="416" width="958" height="39" rx="6" fill="#fef3c7"/>')
line(88, 416, 1032, 416, dashed=True)
text(560, 443, '数据库时间已越过 lease_until', 17)
box(307, 569, 206, ['仍更新：过期结果被接受'], '#fee2e2')
box(844, 556, 192, ['此时再检查期限', '已到期：不更新'], '#dcfce7')
text(560, 680, '行更新后的条件复查 ≠ 任意锁等待后都会重新判断时钟', 17)
text(560, 704, '图只解释该SQL提交窗口，不表示外部动作已停止或整个任务已经恢复', 14)
parts.append('</g></svg>')
Path(__file__).with_suffix('.svg').write_text(''.join(parts), encoding='utf-8')
