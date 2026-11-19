"""Original editable HITL diagrams; Python3 standard library."""
from pathlib import Path
from html import escape


def canvas(title):
 return [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 680" role="img" aria-labelledby="title"><title id="title">{escape(title)}</title>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="680" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">',f'<text x="30" y="42" font-size="25">{escape(title)}</text>']


def text(p,x,y,t,size=17):p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')


def box(p,x,y,w,lines,color='#dbeafe'):
 p.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="9" fill="{color}" stroke="#64748b"/>')
 for i,line in enumerate(lines):text(p,x+14,y+30+i*27,line)


def arrow(p,path,t,x,y):
 p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>');text(p,x,y,t,16)


def save(p,name):Path(__file__).with_name(name+'.svg').write_text('\n'.join(p+['</g></svg>']),encoding='utf-8')

p=canvas('HITL：批准绑定快照，修改必须重新审核')
box(p,30,130,260,['waiting：动作已保存','Run / Scope / v1 / 指纹'])
box(p,420,130,260,['approved：人确认v1','当前权限仍是外层约束'])
box(p,815,130,260,['executing：先提交在途','调用只读Reader'])
arrow(p,'M290 170H420','明确批准',324,153)
arrow(p,'M680 170H815','复核应用授权',688,153)
arrow(p,'M550 130V85H165V130','Owner修改 → v2与新指纹 → waiting',280,72)
box(p,30,420,340,['rejected / canceled','不执行，不伪造成功结果'],'#fee2e2')
box(p,720,420,355,['completed / failed','canceled / expired','返回后校验，再提交'],'#ffffff')
arrow(p,'M155 220V420','拒绝 / 取消',169,318)
arrow(p,'M530 220V345H300V420','Owner取消',345,332)
arrow(p,'M940 220V420','输出 / 错误 / 停止',952,318)
text(p,30,584,'决定/修改/恢复均核对版本与指纹；过期不继续，waiting不代表授权永久有效。')
text(p,30,625,'重启看见executing → 结果未知；不能自动重跑，显式只读重试仍受尝试与权限限制。')
save(p,'hitl-state')

p=canvas('HITL：等待无需占着执行者，恢复依赖持久状态')
for x,title in [(150,'调用方 / Runtime'),(550,'审批状态存储'),(950,'人 / 对应角色')]:
 text(p,x-90,95,title);p.append(f'<path d="M{x} 115V590" stroke="#94a3b8" stroke-dasharray="5 5"/>')
arrow(p,'M150 155H550','提交具体Action与范围',220,143)
arrow(p,'M550 205H150','waiting已保存；请求可结束',200,193)
arrow(p,'M950 265H550','查询并阅读动作 / 版本 / 指纹',615,253)
arrow(p,'M950 335H550','明确approve / reject / modify',595,323)
arrow(p,'M150 405H550','恢复：核对授权 / 期限 / 决定',205,393)
arrow(p,'M550 465H150','获准Action；先保存executing',200,453)
arrow(p,'M150 535H550','结果提交（取消或过期不释放）',190,523)
text(p,30,614,'批准/拒绝由Reviewer提交；修改/取消由Owner提交，修改后需要重新审核。')
text(p,30,652,'业务能力仍由应用注册；人的决定不能替模型新增工具，也不等于结果质量通过。')
save(p,'hitl-sequence')
