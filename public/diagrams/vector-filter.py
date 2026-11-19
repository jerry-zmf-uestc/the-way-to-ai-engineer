"""Original editable vector filtering diagram; Python3 standard library."""
from pathlib import Path
from html import escape
p=['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 660" role="img" aria-labelledby="title"><title id="title">SQL输出权限与ANN候选扫描分别判断</title>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="660" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
def text(x,y,t,size=17):p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')
def box(x,y,w,lines,color='#dbeafe'):
 p.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="9" fill="{color}" stroke="#64748b"/>')
 for n,line in enumerate(lines):text(x+14,y+30+n*27,line)
def arrow(path,t,x,y):p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>');text(x,y,t,16)
text(30,42,'向量检索：输出范围正确，不代表候选已经填满',25)
text(30,96,'精确模式：限定合法集合后计算全部距离',18)
box(30,130,260,['可信Scope','tenant / space / version'])
box(420,130,260,['范围内精确距离','全部可见行参与排序'])
box(810,130,260,['top-k','对照基线'])
arrow('M290 175H420','过滤',338,160);arrow('M680 175H810','排序',726,160)
text(30,296,'共享HNSW：物理候选扫描与SQL过滤可能分阶段执行',18)
box(30,335,260,['ANN候选窗口','共享索引的近邻'])
box(420,335,260,['SQL范围过滤','未匹配的候选被删除'],'#fef3c7')
box(810,335,260,['范围匹配的命中','可能少于k条'],'#dcfce7')
arrow('M290 380H420','候选',334,365);arrow('M680 380H810','结果',726,365)
arrow('M540 425V500H160V425','已配置迭代补扫，仍受扫描与请求限制',235,488)
text(30,566,'EXPLAIN确认实际路径；输出再核对Scope与大小，不能先交给模型再过滤。')
text(30,613,'相对精确Recall衡量近似索引损失；语义Recall与答案支持仍需独立任务标签。')
p.append('</g></svg>');Path(__file__).with_name('vector-filter.svg').write_text('\n'.join(p),encoding='utf-8')
