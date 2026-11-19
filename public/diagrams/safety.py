"""Original editable Safety diagrams; Python3 standard library."""
from pathlib import Path
from html import escape


def canvas(title):
 return [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 650" role="img" aria-labelledby="title"><title id="title">{escape(title)}</title>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="650" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">',f'<text x="30" y="42" font-size="25">{escape(title)}</text>']


def text(p,x,y,t,size=17):p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')


def box(p,x,y,w,lines,color='#dbeafe'):
 p.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="9" fill="{color}" stroke="#64748b"/>')
 for i,line in enumerate(lines):text(p,x+14,y+30+i*27,line)


def arrow(p,path,t,x,y):
 p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>');text(p,x,y,t,16)


def save(p,name):Path(__file__).with_name(name+'.svg').write_text('\n'.join(p+['</g></svg>']),encoding='utf-8')

p=canvas('Safety：资料与模型提议不能改变执行权限')
box(p,30,135,230,['检索资料 / MCP结果','不可信正文和指令'],'#fef3c7')
box(p,330,135,230,['Model','工具名与参数提议'],'#fef3c7')
box(p,635,135,240,['应用Gate','注册 / Schema / Scope'])
arrow(p,'M260 180H330','数据',275,162)
arrow(p,'M560 180H635','提议',580,162)
box(p,590,340,410,['可信应用策略','批准ID / 分类 / 期限 / 总预算'])
arrow(p,'M755 340V225','应用设置，不来自资料',790,290)
box(p,60,435,420,['Reader：批准资料的只读能力','返回后复核权限，结果仍untrusted'],'#dcfce7')
arrow(p,'M635 200H575V390H275V435','通过才执行',355,379)
box(p,590,485,410,['拒绝与有界审计','不记录原参数、正文或原始错误'],'#fee2e2')
arrow(p,'M875 200H1050V530H1000','拒绝',1005,448)
text(p,30,620,'权限来自可信应用；HITL只能在允许范围内决定执行，无法把未注册动作变成可执行。')
save(p,'safety-boundary')

p=canvas('Safety Gate：执行前拒绝，返回后再次检查')
box(p,30,110,290,['计入尝试预算','工具、参数、Scope与分类'])
box(p,430,110,290,['执行Reader','不持有Gate锁'])
box(p,825,110,240,['返回后校验','取消 / 撤销 / 期限'])
arrow(p,'M320 155H430','允许',351,141)
arrow(p,'M720 155H825','结果或错误',743,141)
box(p,60,385,420,['拒绝或失败 → 空Evidence','私有错误不进入公开响应'],'#fee2e2')
box(p,590,385,420,['校验ID与大小 → untrusted证据','不是正确性或秘密检测证明'],'#dcfce7')
arrow(p,'M175 200V385','执行前拒绝',190,278)
arrow(p,'M910 200V290H345V385','返回后停止 / 失败',485,278)
arrow(p,'M980 200V385','可释放',991,314)
text(p,30,566,'撤销禁止迟到结果释放，但不回滚已发生的读取；任意阻塞函数仍需下游协作。')
text(p,30,605,'审计按白名单字段记录；4次尝试与一次预算耗尽决定构成有界记录。')
save(p,'safety-gate-flow')
