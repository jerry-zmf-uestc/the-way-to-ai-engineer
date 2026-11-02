"""Original editable local encoding diagram; Python3 standard library."""
from pathlib import Path
from html import escape
p=['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 640" role="img" aria-labelledby="title"><title id="title">真实编码：相同处理空间与独立任务标签</title>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="640" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
def text(x,y,t,size=17):p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')
def box(x,y,w,lines,color='#dbeafe'):
 p.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="9" fill="{color}" stroke="#64748b"/>')
 for i,line in enumerate(lines):text(x+14,y+30+i*27,line)
def arrow(path,t,x,y):p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>');text(x,y,t,16)
text(30,42,'真实编码：权重和处理固定，效果在运行外评估',25)
box(30,125,250,['公开合成资料 / 当前查询','权重与处理链固定'])
box(425,125,260,['分词器 + ONNX CPU','mask / 截断 / 隐藏向量'])
box(820,125,250,['mask mean + L2','384维 / Space身份'])
arrow('M280 170H425','输入',330,153);arrow('M685 170H820','处理',733,153)
box(60,395,400,['合法语料中的dense精确排序','与同任务词法基线比较'],'#dcfce7')
box(645,395,400,['Scorer：独立标签 + 实际排名','保留失败 / 无资料 / 未定义指标'],'#fef3c7')
arrow('M940 215V295H260V395','编码身份与向量',487,281)
arrow('M460 440H645','命中ID与顺序',474,423)
text(30,550,'标签来自独立任务定义，不由编码器、检索得分或最终答案生成。')
text(30,584,'默认合成对照在排名时限范围；真实导出路径在编码前过滤，身份授权仍须另核对。',16)
text(30,615,'近邻存在不代表有依据，小语料不证明普遍优势或回答质量。',16)
p.append('</g></svg>');Path(__file__).with_name('neural-encoding.svg').write_text('\n'.join(p),encoding='utf-8')
