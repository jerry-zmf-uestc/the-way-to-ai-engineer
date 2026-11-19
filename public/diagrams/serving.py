"""Original editable Serving diagrams; Python3 standard library."""
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

p=canvas('Serving：入口、任务与执行的边界')
box(p,30,130,260,['HTTP请求','身份 / JSON / 幂等键'])
box(p,420,130,270,['Task Manager','Scope / ID / 版本 / 名额'])
box(p,820,130,250,['只读Worker','任务总时限 / 协作取消'])
arrow(p,'M290 170H420','接纳',330,155)
arrow(p,'M690 170H820','独立寿命',725,155)
arrow(p,'M555 220V305H160V220','202 + Location；请求结束',225,292)
arrow(p,'M940 220V395H555V220','退出后提交终态',665,382)
box(p,50,465,450,['GET /tasks/{id}：只查授权范围','连接断开不取消已接纳任务'],'#ffffff')
box(p,590,465,450,['Request ID ≠ Task ID ≠ 幂等键','当前实现无持久队列 / 重启恢复'],'#ffffff')
text(p,30,600,'身份提供方把请求映射为可信Scope；demo bearer表只是可重复的教学替身。')
save(p,'serving-boundary')

p=canvas('关闭服务：拒绝新工作与真实退出分开')
box(p,30,115,280,['SIGINT / SIGTERM','同一关闭预算'])
box(p,420,115,300,['readiness = false','拒绝新Start / Update'])
arrow(p,'M310 155H420','开始关闭',332,142)
box(p,70,315,400,['取消活跃任务，等待worker退出','迟到文本不能提交'],'#ffffff')
box(p,630,315,400,['HTTP Shutdown','停止监听，等待活跃handler'],'#ffffff')
arrow(p,'M500 205V265H270V315','任务边界',295,252)
arrow(p,'M640 205V265H830V315','连接边界',770,252)
box(p,300,500,500,['两者退出才是drained','超时返回错误；不能强行抢占Go函数'],'#ffffff')
arrow(p,'M270 405V455H410V500','确认',304,443)
arrow(p,'M830 405V455H700V500','确认',751,443)
text(p,30,628,'策略为取消只读任务并收齐；若需完成当前任务，应另行设计自然排空与持久接管。')
save(p,'serving-shutdown')
