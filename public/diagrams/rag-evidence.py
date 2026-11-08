"""Original editable RAG branches; rebuild with Python 3 standard library."""
from html import escape
from pathlib import Path

p = ['<svg xmlns="http://www.w3.org/2000/svg" width="1100" height="790" viewBox="0 0 1100 790" role="img" aria-labelledby="title desc">',
     '<title id="title">RAG：回答与拒答有不同验证出口</title>',
     '<desc id="desc">输入非法返回错误，无证据或结构化冲突由应用拒答；合法证据进入模型。草稿解析后，回答核对引用与支持，模型拒答核对互斥契约。任何关键失败不交付部分答案，任务效果另由Eval判断。</desc>',
     '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs>',
     '<rect width="1100" height="790" rx="16" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">']


def text(x, y, value, size=17):
    p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')


def box(x, y, width, lines, color='#dbeafe'):
    height = 30 + len(lines) * 27
    p.append(f'<rect x="{x}" y="{y}" width="{width}" height="{height}" rx="10" fill="{color}" stroke="#64748b"/>')
    for n, value in enumerate(lines):
        text(x+15, y+30+n*27, value, 19 if n == 0 else 16)


def arrow(path, x, y, label):
    p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>')
    text(x, y, label, 15)


text(30, 40, 'RAG：回答与拒答有不同验证出口', 25)
text(30, 73, '图示对应给定证据的Run；追加检索由外层Loop与总预算另行管理。', 16)
box(250, 105, 600, ['输入检查：Scope / 大小 / 来源 / 版本', '证据先筛选；非法或越权不进入模型'])
box(30, 210, 190, ['应用拒答', 'no_evidence', 'conflicting_evidence'], '#fef3c7')
arrow('M250 155H125V210', 35, 185, '无证据 / 明确冲突')
box(250, 250, 600, ['Model：问题 + 合法证据 → 原始草稿', '返回后检查取消；失败不释放残余文本'])
arrow('M550 189V250', 570, 219, '合法且可用')
box(250, 380, 600, ['Parse：有界JSON与互斥分支', '回答1–16个Claim；拒答只允许稳定原因'])
arrow('M550 334V380', 570, 359, '完整草稿')
box(30, 510, 440, ['回答：出处 → 摘录归属 → 支持', 'unsupported拒绝；unknown待复核'], '#ffffff')
box(550, 510, 300, ['模型拒答', '无Claim；原因契约通过'], '#ffffff')
arrow('M430 464V487H250V510', 265, 504, 'Claims')
arrow('M690 464V510', 707, 491, 'Abstained')
box(885, 300, 195, ['错误 / 待复核', '越权、调用失败', '协议、取消', '不发布部分答案', '不伪装成拒答'], '#fee2e2')
arrow('M850 148H980V300', 867, 227, '非法输入')
arrow('M850 292H865V360H885', 865, 279, '失败')
arrow('M850 423H885', 0, 0, '')
box(250, 660, 600, ['返回：经验证回答 / 有来源性质的拒答', '正确拒答、误拒答与任务完成度由Eval另查'], '#dcfce7')
arrow('M125 321H15V708H250', 32, 641, '应用拒答，不调用Model')
arrow('M250 594V660', 269, 631, '通过')
arrow('M700 594V660', 715, 631, '契约通过，不证明题目无解')
text(30, 773, '应用拒答不调用Model；模型拒答不调用事实Judge。所有路径仍受取消与权限边界约束。', 16)
p.append('</g></svg>\n')
Path(__file__).with_suffix('.svg').write_text('\n'.join(p), encoding='utf-8')
