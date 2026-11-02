"""Original editable diagram source; Python 3 standard library only.
Run from repository root: python3 public/diagrams/agent-loop.py
The SVG contains static shapes and text, no scripts or external resources.
"""
from pathlib import Path
from html import escape

parts = ['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 660" role="img" aria-labelledby="title desc">
<title id="title">Agent Loop：Harness位置与决策流程</title>
<desc id="desc">Loop读取Session、调用Model、执行Tool并回填观察。没有工具提议且Final非空时保存并返回；取消、协议失败、模型错误或预算耗尽时终止。返回文本不等于完成业务任务。</desc>
<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="#475569"/></marker></defs>
<rect width="960" height="660" rx="16" fill="#f8fafc"/>
<g font-family="system-ui, sans-serif" fill="#0f172a">''']

def label(x, y, text, size=16, center=True):
    parts.append(f'<text x="{x}" y="{y}" font-size="{size}" text-anchor="{"middle" if center else "start"}">{escape(text)}</text>')

def box(x, y, w, h, lines, color='#ffffff', dashed=False):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{color}" stroke="#64748b"'+(' stroke-dasharray="6 5"' if dashed else '')+'/>')
    for i, text in enumerate(lines):
        label(x+w/2, y+h/2+(i-(len(lines)-1)/2)*23+5, text)

def arrow(path, dashed=False):
    parts.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#arrow)"'+(' stroke-dasharray="6 5"' if dashed else '')+'/>')

label(30, 36, '01  Loop在Harness中的位置', 21, False)
box(30, 62, 900, 56, ['Runtime：状态、预算与错误策略  /  Checkpoint：执行恢复  /  Streaming：运行呈现'], '#eef2ff', True)
box(60, 166, 190, 72, ['Session', '消息历史与快照'])
box(350, 156, 240, 92, ['Agent Loop', '决定继续还是终止'], '#dbeafe')
box(700, 138, 200, 52, ['Model：一轮响应'])
box(700, 212, 200, 52, ['Tool：受控执行'])
arrow('M 250 194 L 350 194'); label(300, 184, '读取', 13)
arrow('M 350 224 L 250 224'); label(300, 248, '追加观察', 13)
arrow('M 590 180 L 650 180 L 650 164 L 700 164')
arrow('M 700 150 L 630 150 L 630 168 L 590 168')
arrow('M 590 218 L 655 218 L 655 238 L 700 238')
arrow('M 700 252 L 635 252 L 635 236 L 590 236')
label(480, 295, '实线：Loop的调用与数据流；虚线框：相邻部件的职责', 14)
label(30, 337, '02  一次运行如何推进', 21, False)
box(40, 367, 150, 60, ['检查取消', '与剩余轮次'])
box(265, 367, 165, 60, ['调用Model', '读取消息快照'])
box(505, 367, 170, 60, ['响应分支', 'Final / ToolCalls'])
box(745, 367, 170, 60, ['无工具且Final非空', '保存并返回'], '#dcfce7')
arrow('M 190 397 L 265 397'); arrow('M 430 397 L 505 397'); arrow('M 675 397 L 745 397')
label(710, 385, 'Final', 13)
box(505, 493, 170, 70, ['记录提议', '执行工具'])
box(265, 493, 165, 70, ['回填观察', 'ToolCallID配对'])
arrow('M 590 427 L 590 493'); label(655, 467, 'ToolCalls', 13)
arrow('M 505 528 L 430 528')
arrow('M 265 528 L 115 528 L 115 427'); label(180, 517, '下一轮', 13)
box(40, 592, 875, 42, ['取消 / 超时 / 模型错误 / 空响应 / 轮次耗尽 → 返回错误'], '#fee2e2')
label(775, 525, '工具取消直接终止', 14)
label(775, 548, '普通失败回填error观察', 14)
parts.append('</g></svg>\n')
Path(__file__).with_suffix('.svg').write_text('\n'.join(parts), encoding='utf-8')
