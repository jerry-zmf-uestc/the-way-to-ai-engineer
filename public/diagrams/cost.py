"""Original editable Cost diagrams; Python3 standard library."""
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

p=canvas('Cost：在途预留与实际结算分开')
box(p,30,120,270,['reserved：锁定价格与上界','Held增加，Used不变'])
box(p,415,120,270,['sent：跨过发送边界','取消不证明零消耗'])
box(p,800,120,270,['settled：已知实际usage','Held释放，Used增加'],'#dcfce7')
arrow(p,'M300 160H415','Begin通过',319,145)
arrow(p,'M685 160H800','Settle',721,145)
box(p,30,370,270,['not_sent：发送前取消','释放Held，实际为零'],'#ffffff')
box(p,415,370,270,['unknown：等待对账','保留Held，不写零费用'],'#fef3c7')
box(p,800,370,270,['overrun：记下实际费用','冻结新接纳，不截账单'],'#fee2e2')
arrow(p,'M155 210V370','Begin时取消',171,292)
arrow(p,'M540 210V370','缺少可信usage → 对账',365,292)
arrow(p,'M685 405H740V250H875V210','后来得到usage',755,322)
arrow(p,'M635 210V285H950V370','结算超界',810,272)
arrow(p,'M685 425H800','对账超界',710,413)
text(p,30,566,'可接纳额度 = Limit − Used − Held；重复相同usage不再次收费，不同重报拒绝。')
text(p,30,605,'调用失败也可能有已知usage；本地账本控制后续接纳，Provider费用另行对账。')
save(p,'cost-reservation')

p=canvas('虚构数据：单任务便宜，不一定每成功任务便宜')
box(p,60,120,440,['cheap：2个任务，各60单位','1成功 + 1失败，总费用120'],'#ffffff')
box(p,600,120,440,['strong：2个任务，各90单位','2成功，总费用180'],'#ffffff')
box(p,60,320,440,['每成功任务 = 120 / 1 = 120','失败费用不能从分子删除'],'#fef3c7')
box(p,600,320,440,['每成功任务 = 180 / 2 = 90','高单次费用仍可能更低总成本'],'#dcfce7')
arrow(p,'M280 210V320','全部尝试 / 成功数',292,275)
arrow(p,'M820 210V320','全部尝试 / 成功数',832,275)
text(p,30,495,'比较必须使用相同任务集、单位与独立成功标签，并满足质量和延迟条件。')
text(p,30,535,'费用未知 → 报告覆盖率；成功为零 → 指标未定义，不显示零成本。')
text(p,30,585,'这里只演示计算关系，不是现实模型、路由、缓存或价格的效果结论。')
save(p,'cost-per-success')

p=canvas('持久费用账本：确认保存以后，才进入下一步')
box(p,40,75,280,['应用 / adapter','调用方绑定Run与Call'])
box(p,420,75,280,['私有账本文件','单写者、同步与替换'])
box(p,800,75,270,['外部服务 / 受控替身','执行与usage另有窗口'])
for x in (180,560,935):
 p.append(f'<path d="M{x} 170V535" stroke="#cbd5e1" stroke-width="2" fill="none"/>')
arrow(p,'M180 200H560','Reserve：保存reserved / Held50',192,188)
arrow(p,'M560 240H180','提交确认',300,228)
arrow(p,'M180 280H560','Begin：保存sent意图',192,268)
arrow(p,'M560 320H180','提交确认后才允许调用',224,308)
arrow(p,'M180 380H935','执行只读调用',192,368)
arrow(p,'M935 430H180','返回可信usage（本实验为合成值）',588,418)
arrow(p,'M180 480H560','Settle：保存Actual并释放Held',192,468)
arrow(p,'M560 520H180','提交确认',300,508)
for y,label,color in (
 (338,'退出①：sent已保存，调用尚未开始','#fef3c7'),
 (438,'退出②：调用已发生，结算尚未提交','#fef3c7'),
 (533,'退出③：已结算，继续之前退出','#dcfce7')):
 p.append(f'<rect x="590" y="{y}" width="440" height="30" rx="5" fill="{color}"/>')
 text(p,604,y+21,label,16)
text(p,30,591,'①②恢复unknown / Held50，不重发原Call；③保留settled / Used40 / Held0。',16)
text(p,30,624,'单Call合成示例：预留50、调用后消费40；费用文件恢复不代替执行Checkpoint。',16)
save(p,'cost-persistence')
