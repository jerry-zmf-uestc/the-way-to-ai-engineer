"""Original editable model mechanism diagram; Python3 standard library."""
from pathlib import Path
from html import escape
p=['<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 720" role="img" aria-labelledby="title"><title id="title">因果Attention与KV缓存：新Query仍读取历史</title>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="720" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
def text(x,y,t,size=18):p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')
def box(x,y,w,lines,color='#dbeafe'):
 p.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="9" fill="{color}" stroke="#64748b"/>')
 for i,line in enumerate(lines):text(x+14,y+30+i*27,line,17)
def arrow(path,t,x,y):p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>');text(x,y,t,16)
text(30,42,'因果Attention：每行只读取当前位置与过去',25)
for i in range(4):
 text(175+i*92,105,f'K{i+1}',18)
 text(90,153+i*48,f'Q{i+1}',18)
 for j in range(4):
  color='#dbeafe' if j<=i else '#e2e8f0'
  p.append(f'<rect x="{155+j*92}" y="{127+i*48}" width="74" height="34" rx="4" fill="{color}"/>')
  text(170+j*92,150+i*48,'可读' if j<=i else '遮罩',15)
text(565,135,'score = dot(Q,K) / sqrt(d)',18)
text(565,177,'softmax → 以权重组合V',18)
text(565,219,'改变未来Token不影响此前输出',18)
text(565,261,'注意力权重不等于事实正确率',18)
box(35,380,280,['新增Token与位置','只计算新增Q / K / V'])
box(400,380,290,['旧K/V + 新K/V','新Q仍读取全部可见历史'])
box(795,380,265,['本步输出','从logits到采样另一步'])
arrow('M315 425H400','追加',342,411)
arrow('M690 425H795','计算',723,411)
box(270,555,560,['缓存身份：权重 / 分词器 / 位置配置 / 相同前缀','编辑历史或身份改变 → 拒绝旧缓存，重新计算'],'#fef3c7')
arrow('M545 555V470','匹配才复用',567,522)
text(30,685,'固定二维单头数值只验证机制；不是训练模型，也不测真实延迟或Provider前缀缓存。',17)
p.append('</g></svg>');Path(__file__).with_name('model-inference.svg').write_text('\n'.join(p),encoding='utf-8')
