"""Original editable Streaming diagrams. Rebuild with Python3 standard library."""
from pathlib import Path
from html import escape


def start(title):
    return [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 620" role="img" aria-labelledby="title"><title id="title">{escape(title)}</title>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="620" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">',f'<text x="30" y="45" font-size="25">{escape(title)}</text>']


def text(p,x,y,t,size=17):p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(t)}</text>')


def box(p,x,y,w,lines,color='#dbeafe'):
    p.append(f'<rect x="{x}" y="{y}" width="{w}" height="90" rx="9" fill="{color}" stroke="#64748b"/>')
    for i,line in enumerate(lines):text(p,x+12,y+30+i*27,line)


def arrow(p,path,label,x,y):
    p.append(f'<path d="{path}" stroke="#475569" stroke-width="2" fill="none" marker-end="url(#a)"/>')
    text(p,x,y,label,16)


def save(p,name):Path(__file__).with_name(name+'.svg').write_text('\n'.join(p+['</g></svg>']),encoding='utf-8')

p=start('Streaming：事件、传输与页面各有职责')
box(p,30,135,215,['runAgent','事件与完成原因'])
box(p,310,135,215,['无缓冲channel','发送或ctx取消'])
box(p,590,135,215,['HTTP writer','run_id / seq / SSE'])
box(p,865,135,205,['浏览器','校验与安全呈现'])
arrow(p,'M245 180H310','事件',261,163)
arrow(p,'M525 180H590','消费',540,163)
arrow(p,'M805 180H865','网络',820,163)
arrow(p,'M960 225V320H695V225','关闭连接 → 请求取消',720,306)
arrow(p,'M695 225V365H140V225','写失败 / 超时 → cancel → 发送者退出',250,352)
box(p,70,430,455,['一次连接只负责当前流','write/flush成功 ≠ 用户收到终态'],'#ffffff')
box(p,590,430,440,['恢复需要独立状态与事件存储','当前Console不自动重跑、不支持重放'],'#ffffff')
text(p,30,573,'当前Console采用请求寿命；异步任务另有自己的状态，不照搬连接取消策略。')
text(p,30,603,'发送者、网络写入与工具分别协作退出；连接关闭不证明客户端收到done。',16)
save(p,'streaming-pipeline')

p=start('页面终态：只有明确事件才能结束当前运行')
box(p,55,110,255,['接收当前连接事件','running / 部分文本'])
box(p,650,110,340,['done(stop)','正常结束；质量另行评估'],'#dcfce7')
box(p,650,295,340,['done(error / timeout / cancelled)','失败或取消；保留部分输出'],'#fee2e2')
box(p,650,480,340,['断连 / 协议无效 / 本地停止','完成未知；关闭当前连接'],'#fef3c7')
arrow(p,'M310 155H650','同run_id，seq连续',350,143)
arrow(p,'M185 200V340H650','明确非成功终态',350,327)
arrow(p,'M125 200V525H650','没有可信终态确认',350,512)
text(p,250,265,'旧连接事件直接忽略。')
text(p,250,303,'JSON或序号错误不能补猜结果。')
text(p,30,570,'id与seq仅用于当前流归属和顺序；没有持久事件账本，不能据此恢复旧任务。')
save(p,'streaming-terminal')
