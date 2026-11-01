"""Original editable strategy control diagram; Python3 standard library."""
from pathlib import Path
from html import escape
p=['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 660" role="img" aria-labelledby="title"><title id="title">同条件策略：子问题与有界修复关注不同缺口</title>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="660" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
def text(x,y,t,size=17):p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')
def box(x,y,w,lines,color='#dbeafe'):
 p.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="9" fill="{color}" stroke="#64748b"/>')
 for i,line in enumerate(lines):text(x+14,y+30+i*27,line)
def arrow(path,t,x,y):p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>');text(x,y,t,16)
text(30,42,'同条件策略：追问依赖观察，修复需要具体差距',25)
box(50,110,430,['子问题：timeout资料在哪里？','root → router → timeout'])
box(620,110,430,['初始计划：先读首参考','可能无结果、旧版本或缺少阶段'])
box(50,300,430,['中间答案决定后续问题','不直接把router当最终事实'],'#ffffff')
box(620,300,430,['Critique：缺哪些要求 / 版本不符？','不读取评估答案'],'#ffffff')
arrow('M260 200V300','最多两跳，共享读预算',285,258)
arrow('M835 200V300','依据执行证据',855,258)
box(50,490,430,['观察支持事实 → 独立评分','子问题分解也可能失败或遗漏'],'#dcfce7')
box(620,490,430,['已观察候选 → 有界尝试 → 再检查','保留已完成事实，不重置总预算'],'#dcfce7')
arrow('M260 390V490','来源与版本校验',285,444)
arrow('M835 390V490','一次修复；每缺项最多两候选',855,444)
text(30,627,'固定控制器只验证机制；同任务/语料/能力/预算比较，不冒充真实模型收益或论文复现。')
p.append('</g></svg>');Path(__file__).with_name('strategy-control-comparison.svg').write_text('\n'.join(p),encoding='utf-8')
