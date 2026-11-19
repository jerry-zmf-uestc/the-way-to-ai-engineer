# coding: utf-8
"""Original conceptual Reflexion feedback flow; Python standard library only."""
from pathlib import Path
from html import escape
parts=['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 460" role="img" aria-labelledby="title desc"><title id="title">执行反馈、语言经验与下一次尝试</title><desc id="desc">执行者使用任务和经验，环境返回实际观察，评价者检查任务，反思器形成经验供下次输入选择；不更新权重或授予权限。概念图不代表实验已实现跨任务语言记忆。</desc><defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="960" height="460" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">''']
def label(x,y,s,size=16):parts.append(f'<text x="{x}" y="{y}" font-size="{size}" text-anchor="middle">{escape(s)}</text>')
def box(x,y,w,lines,color='#fff'):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="78" rx="10" fill="{color}" stroke="#64748b"/>')
    for n,s in enumerate(lines):label(x+w/2,y+30+n*24,s)
def arrow(d):parts.append(f'<path d="{d}" stroke="#475569" stroke-width="2" fill="none" marker-end="url(#a)"/>')
label(480,35,'Reflexion概念：用反馈与经验影响后续尝试',22)
box(25,90,210,['执行者 Actor','任务 + 已选经验'],'#dbeafe')
box(360,90,235,['环境与评价者','实际观察 / 任务反馈'],'#dbeafe')
box(720,90,215,['反思器','具体差距与建议'],'#fef3c7')
arrow('M235 129H360');label(298,108,'动作与结果',14)
arrow('M595 129H720');label(657,108,'失败与约束',14)
box(360,270,270,['有范围的经验缓冲','来源 / 版本 / 失效条件'],'#fef3c7')
arrow('M827 168V309H630');label(735,292,'记录语言经验',14)
arrow('M360 309H130V168');label(143,273,'下次输入选择',14)
label(480,402,'权重、执行权限与评估答案不从经验缓冲继承',17)
label(480,436,'概念流程；已有局部语言反馈链，可靠采用与跨任务收益仍待验',16)
parts.append('</g></svg>')
Path(__file__).with_suffix('.svg').write_text(''.join(parts),encoding='utf-8')
