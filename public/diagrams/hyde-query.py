# coding: utf-8
"""Original static query-flow diagram; rebuild with Python 3 standard library."""
from pathlib import Path
from html import escape
parts=['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 430" role="img" aria-labelledby="title desc"><title id="title">直接查询与假想文档查询</title><desc id="desc">两种查询使用相同编码器并检索相同真实资料。假想文档不进入证据库，比较排名与新增生成消耗。</desc><defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="960" height="430" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">''']
def text(x,y,s,size=17):
    parts.append(f'<text x="{x}" y="{y}" text-anchor="middle" font-size="{size}">{escape(s)}</text>')
def box(x,y,w,lines,color='#fff'):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="72" rx="10" fill="{color}" stroke="#64748b"/>')
    for n,line in enumerate(lines):text(x+w/2,y+29+n*23,line)
def arrow(d):parts.append(f'<path d="{d}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#arrow)"/>')
text(480,35,'HyDE改变查询表示，真实资料仍是检索与证据来源',22)
box(25,157,130,['原始问题'])
box(220,68,205,['直接编码问题','MiniLM / 同一Space'],'#dbeafe')
box(220,225,205,['生成假想文档','SmolLM2 / 未验证'],'#fef3c7')
box(485,225,180,['编码假想文档','同一MiniLM'],'#dbeafe')
box(730,135,205,['相同真实资料','Scope / 版本过滤'],'#dcfce7')
arrow('M155 180H185V104H220');arrow('M155 206H185V261H220')
arrow('M425 104H835V135');arrow('M425 261H485');arrow('M665 261H835V207')
text(530,179,'只改变查询向量',15)
text(480,345,'比较：同问题、同语料、同标签 → Recall / 排序 / 生成Token / 延迟',17)
text(480,385,'假想文本不写入资料库，不转为引用，不作为回答证据',17)
parts.append('</g></svg>')
Path(__file__).with_suffix('.svg').write_text(''.join(parts),encoding='utf-8')
