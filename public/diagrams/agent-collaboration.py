"""Original editable collaboration diagrams, Python3 standard library.
Rebuild: python3 public/diagrams/agent-collaboration.py
"""
from pathlib import Path
from html import escape

def draw(name,title,subtitle,top,left,right,bottom,notes):
 parts=[f'<svg xmlns="http://www.w3.org/2000/svg" width="1000" height="650" viewBox="0 0 1000 650" role="img" aria-labelledby="title desc"><title id="title">{escape(title)}</title><desc id="desc">{escape(subtitle)}</desc>',
 '<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L10 5 L0 10Z" fill="#475569"/></marker></defs><rect width="1000" height="650" rx="16" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
 def text(x,y,t,size=18):parts.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')
 def box(x,y,w,lines,color):
  parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="10" fill="{color}" stroke="#64748b"/>')
  for n,line in enumerate(lines):text(x+18,y+32+n*27,line,19 if n==0 else 16)
 def arrow(path):parts.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#arrow)"/>')
 text(30,40,title,24);text(30,74,subtitle,16)
 box(210,106,580,top,'#dbeafe');box(45,265,420,left,'#ffffff');box(535,265,420,right,'#ffffff');box(210,425,580,bottom,'#dcfce7')
 arrow('M350 196 V225 H255 V265');arrow('M650 196 V225 H745 V265');arrow('M255 355 V393 H350 V425');arrow('M745 355 V393 H650 V425')
 for n,note in enumerate(notes):text(30,559+n*28,note,16)
 parts.append('</g></svg>\n');Path(__file__).with_name(name+'.svg').write_text('\n'.join(parts),encoding='utf-8')
draw('delegation-boundary','Delegation：职责、能力与结果归属','实线表示委派与结果流；角色不自动继承所有工具或完整父历史。',
 ['主任务：Scope / Parent / expected / 总预算','委派前登记子任务ID→角色，并选入必要Context'],
 ['timeout-reader','仅read-timeout，取得超时事实与来源'],
 ['retry-reader','仅read-retry，取得重试事实与来源'],
 ['归属与集合检查 → 冲突汇总','核对父/子任务、角色与来源；Missing仍由父级验收'],
 ['收齐结果不等于目标完成；子调用保留父控制权，Handoff改变活动角色。','固定Go角色示例验证分工，不证明真实多LLM效果或并行速度。'])
draw('dag-scheduling','DAG：就绪、并发与汇总','依赖全completed才启动后续；调度器单写状态，worker只返回结果。',
 ['校验图 → 有界FIFO就绪队列','parallel限制在途；queueLimit限制等待，满时显式背压'],
 ['timeout：独立节点','pending → queued → running → completed'],
 ['retry：独立节点','失败则阻断summary；独立任务可继续'],
 ['summary：依赖timeout与retry','只接收有效结果副本；取消后收齐在途，不提交迟到文本'],
 ['running是派发登记，不证明回调已进入；Handler仍需协作退出。','单运行FIFO不证明多租户公平，真实延迟与总消耗需要独立测量。'])
