"""Original editable strategy diagrams; Python3 standard library.
Rebuild: python3 public/diagrams/agent-strategies.py
"""
from pathlib import Path
from html import escape

def render(name,title,subtitle,left,right,left_note,right_note):
    parts=[f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 670" role="img" aria-labelledby="title desc"><title id="title">{escape(title)}</title><desc id="desc">{escape(subtitle)}</desc>',
    '<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L10 5 L0 10Z" fill="#475569"/></marker></defs><rect width="1000" height="670" rx="16" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']
    def text(x,y,value,size=18):parts.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')
    def box(x,y,head,body,color):
        parts.append(f'<rect x="{x}" y="{y}" width="420" height="83" rx="10" fill="{color}" stroke="#64748b"/>');text(x+18,y+30,head,20);text(x+18,y+60,body,16)
    def arrow(x,y):parts.append(f'<path d="M{x} {y} V{y+30}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#arrow)"/>')
    text(30,40,title,24);text(30,74,subtitle,16)
    for x,stages,note in [(40,left,left_note),(540,right,right_note)]:
        for n,(head,body,color) in enumerate(stages):
            y=106+n*119;box(x,y,head,body,color)
            if n<len(stages)-1:arrow(x+210,y+83)
        text(x,602,note,16)
    text(30,642,'任务与执行记录独立评分；教学策略与固定事实不证明真实LLM效果。',16)
    parts.append('</g></svg>\n');Path(__file__).with_name(name+'.svg').write_text('\n'.join(parts),encoding='utf-8')
render('react-feedback','ReAct：观察如何改变下一步','两列使用相同问题、资料、工具与查询目录，只比较反馈后的动作选择。',[
 ('固定策略：查询E42','读取同一工具与资料范围','#dbeafe'),
 ('观察：没有命中','工具完成，不是取得了答案','#ffffff'),
 ('固定分支：结束并拒答','未使用可用的替代查询','#fee2e2'),
 ('任务评分：目标未完成','需要的超时参数仍未取得','#ffffff'),
],[
 ('观察驱动策略：查询E42','相同初始请求与能力','#dbeafe'),
 ('观察：没有命中','确认缺少超时参数证据','#ffffff'),
 ('改变动作：查询timeout','受工具配额与轮次限制','#dbeafe'),
 ('获得证据：再决策并结束','输出关联来源，独立核对任务','#dcfce7'),
],'调用较少，不一定完成任务。','多一次动作，需要比较收益与代价。')
render('planning-revision','Planning：修订未完成工作','计划v1失败后形成候选v2；完成项的定义和证据不能静默改写。',[
 ('计划v1：查retry → 查timeout','演示顺序，非参数数据依赖','#dbeafe'),
 ('retry：completed','保留事实2与guide@v1来源','#dcfce7'),
 ('timeout：failed','查询E42没有取得事实','#fee2e2'),
 ('反馈：缺少timeout证据','提出新计划，不清空已有结果','#ffffff'),
],[
 ('候选v2：校验版本与依赖','非法图或改写完成项就拒绝','#dbeafe'),
 ('retry：沿用completed','定义不变，结果保持','#dcfce7'),
 ('timeout：pending → 执行','查询改为timeout，再核对来源','#dbeafe'),
 ('节点完成 → 任务评分','全completed不替代目标正确性','#dcfce7'),
],'未修订流程：失败后停止。','总尝试与修订次数分别有上限。')
