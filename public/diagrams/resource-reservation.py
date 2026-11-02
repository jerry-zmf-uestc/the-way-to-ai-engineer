# coding: utf-8
"""Original static resource-cycle/reservation figure; Python standard library."""
from pathlib import Path
from html import escape
parts=['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 380" role="img" aria-labelledby="title desc"><title id="title">依赖无环与资源等待成环</title><desc id="desc">独立任务分别持有DB和CPU并等待对方资源。调度器先检查全部声明资源，全部可用后预留，不让任务持有半份资源等待。</desc><defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="960" height="380" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">''']
def text(x,y,s,size=16):parts.append(f'<text x="{x}" y="{y}" font-size="{size}" text-anchor="middle">{escape(s)}</text>')
def box(x,y,w,lines,color='#fff'):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="82" rx="10" fill="{color}" stroke="#64748b"/>')
    for n,s in enumerate(lines):text(x+w/2,y+31+n*24,s)
def arrow(d):parts.append(f'<path d="{d}" stroke="#475569" stroke-width="2" fill="none" marker-end="url(#arrow)"/>')
text(245,35,'独立任务仍可形成资源环',21);text(730,35,'一次预留全部声明资源',21)
box(30,105,175,['任务A持有DB','等待CPU'],'#fee2e2')
box(285,105,175,['任务B持有CPU','等待DB'],'#fee2e2')
arrow('M205 122H245V78H370V105');text(273,65,'等待B的CPU',14)
arrow('M285 170H245V220H118V187');text(180,245,'等待A的DB',14)
box(550,70,355,['检查DB与CPU的剩余槽','任一不足：保持排队'],'#dbeafe')
box(550,205,355,['全部可用：统一预留再派发','收到该worker返回，才释放预留'],'#dcfce7')
arrow('M728 152V205');text(825,183,'满足条件',14)
text(480,330,'仅约束声明资源；Handler自行获取其它资源仍需额外设计',17)
parts.append('</g></svg>')
Path(__file__).with_suffix('.svg').write_text(''.join(parts),encoding='utf-8')
