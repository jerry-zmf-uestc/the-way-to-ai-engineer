"""Original editable Async Task diagrams; Python standard library only.
Rebuild: python3 public/diagrams/async-tasks.py
"""
from pathlib import Path
from html import escape


def canvas(title,desc):
    return [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1100 680" role="img" aria-labelledby="title desc"><title id="title">{escape(title)}</title><desc id="desc">{escape(desc)}</desc>', '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="1100" height="680" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">',f'<text x="30" y="42" font-size="25">{escape(title)}</text>']


def label(parts,x,y,s,size=17):
    parts.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(s)}</text>')


def arrow(parts,path,label_text,x,y):
    parts.append(f'<path d="{path}" stroke="#475569" stroke-width="2" fill="none" marker-end="url(#a)"/>')
    label(parts,x,y,label_text,16)


def box(parts,x,y,w,lines):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="75" rx="9" fill="#dbeafe" stroke="#64748b"/>')
    for i,line in enumerate(lines): label(parts,x+14,y+29+i*26,line)


def save(parts,name):
    Path(__file__).with_name(name+'.svg').write_text('\n'.join(parts+['</g></svg>']),encoding='utf-8')

p=canvas('Async Tasks：确认未调用或已退出，再提交终态','箭头是合法状态转换；版本冲突拒绝，更新不产生重叠attempt。')
box(p,30,110,225,['Start通过接纳','running · v1'])
box(p,420,110,235,['Update通过 · v2','updating'])
box(p,820,110,240,['旧版本已停止','running · v2'])
arrow(p,'M255 145H420','替换输入，取消旧attempt',265,102)
arrow(p,'M655 145H820','原名额内重跑',676,102)
box(p,50,370,225,['canceling','仍占用在途名额'])
box(p,420,370,235,['canceled','丢弃迟到文本'])
box(p,820,370,240,['completed / failed','或canceled（当前版本）'])
arrow(p,'M145 185V370','Cancel / Close',155,270)
arrow(p,'M535 185V275H260V370','Cancel',310,260)
arrow(p,'M845 185V225H170V370','Cancel v2',675,214)
arrow(p,'M275 407H420','确认停止',294,395)
arrow(p,'M940 185V370','当前版本返回',950,278)
arrow(p,'M50 185V315H885V370','未更新的v1也可完成或失败',530,305)
label(p,30,540,'所有提交在同一锁边界判断：取消 → 总超时 → 版本 → 错误 → 输出。')
label(p,30,578,'Update只允许一次；revision记录指令/取消变更，attempt记录实际执行。')
label(p,30,616,'入口已知取消/失效时跳过worker；v2可成为Attempt=1。')
label(p,30,650,'TTL仅删除终态记录；同进程状态不提供进程重启恢复。')
save(p,'async-task-state')

p=canvas('Async Tasks：请求、任务记录与worker各自的生命周期','纵向表示时间；Start返回后后台继续，查询读取任务记录。')
for x,title in [(140,'主请求 / 用户'),(540,'Task Store / Manager'),(930,'Worker')]:
    label(p,x-90,90,title)
    p.append(f'<path d="M{x} 110V590" stroke="#94a3b8" stroke-dasharray="5 5"/>')
arrow(p,'M140 145H540','Start(scope,key,input)',170,135)
arrow(p,'M540 180H930','登记与排程v1，不等于回调已进入',600,169)
arrow(p,'M540 215H140','返回Task ID；主请求可结束',180,204)
arrow(p,'M140 280H540','Check / List → 当前状态',180,269)
arrow(p,'M140 340H540','Update(expected=1,input) → v2',165,329)
arrow(p,'M540 380H930','取消旧attempt；状态updating',615,368)
arrow(p,'M930 425H540','旧版本未调用或退出；旧结果不提交',610,414)
arrow(p,'M540 470H930','调用v2（同一名额，Attempt另计）',600,459)
arrow(p,'M930 515H540','提交当前版本结果与终态',625,504)
arrow(p,'M140 560H540','Check → completed + output',170,549)
label(p,30,642,'Wait超时只结束等待；任务Cancel、总时限与应用Close另行管理执行。')
save(p,'async-task-sequence')
