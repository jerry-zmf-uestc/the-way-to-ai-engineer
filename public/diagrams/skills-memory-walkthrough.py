"""Original case diagrams for skill loading and current user preferences."""
from html import escape
from pathlib import Path


def canvas(title, description):
    return [
        '<svg xmlns="http://www.w3.org/2000/svg" width="1080" height="660" viewBox="0 0 1080 660" role="img" aria-labelledby="title desc">',
        f'<title id="title">{escape(title)}</title><desc id="desc">{escape(description)}</desc>',
        '<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#23736b"/></marker></defs>',
        '<rect width="1080" height="660" fill="#ffffff"/>',
        '<g font-family="system-ui,sans-serif" fill="#163f3a">',
        f'<text x="32" y="44" font-size="26">{escape(title)}</text>',
    ]


def text(p, x, y, value, size=18):
    p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')


def box(p, x, y, width, lines, color='#e8f5f1'):
    height = 32 + 30 * len(lines)
    p.append(f'<rect x="{x}" y="{y}" width="{width}" height="{height}" rx="10" fill="{color}" stroke="#60968e"/>')
    for n, value in enumerate(lines):
        text(p, x + 16, y + 31 + 30 * n, value, 20 if n == 0 else 17)


def arrow(p, path):
    p.append(f'<path d="{path}" fill="none" stroke="#23736b" stroke-width="2" marker-end="url(#arrow)"/>')


def save(p, name):
    Path(__file__).with_name(name + '.svg').write_text('\n'.join(p + ['</g></svg>\n']), encoding='utf-8')


p = canvas('超时诊断：先选流程，再补参考', '任务先匹配获准元信息，激活诊断正文，再按需读取参考。Context组合指导和真实观察；包文件不自行调用模型或授予执行权限。')
text(p, 32, 82, '任务：这次接口超时，应该先查什么？')
box(p, 32, 116, 300, ['1 发现与选择', 'description匹配诊断任务', '只在获准包中产生候选'])
box(p, 390, 116, 300, ['2 激活正文', 'incident-diagnosis / v1', '读错误 → 比较证据 → 报告缺口'])
box(p, 748, 116, 300, ['3 按需读取', 'references/guide.md', '核对资源摘要与剩余预算'])
arrow(p, 'M332 177H390')
arrow(p, 'M690 177H748')
text(p, 32, 280, '以下是应用后续组装；Load本身不调用模型，也不执行步骤。', 18)
box(p, 32, 312, 300, ['方法：来自Skill', '正文与已读取参考', '告诉模型怎样组织诊断'])
box(p, 390, 312, 300, ['事实：来自受控观察', '错误描述、超时与重试记录', '决定哪些结论有证据'])
box(p, 748, 312, 300, ['Context组装请求', '当前任务 + 指导 + 证据', '保留来源，计算总输入预算'])
arrow(p, 'M332 405H360V463H898V434')
arrow(p, 'M690 373H748')
text(p, 374, 453, '实际取得的内容才进入请求', 16)
box(p, 390, 512, 658, ['Runtime推进 → 工具执行器核对当前许可', '模型提议run-command：应用仅批准read-doc，因此拒绝'], '#f0f8f6')
arrow(p, 'M898 434V512')
text(p, 32, 640, '记住：目录可见 ≠ 正文已加载 ≠ 规则已采用；Skill不提供缺失的事实。', 18)
save(p, 'skills-diagnosis-walkthrough')

p = canvas('用户偏好：修订防旧写，回查防旧读', '同一Tenant/User和response-style键从简洁修订1更新为详细修订2，删除形成空值墓碑修订3。旧缓存只提供键，当前输入由事实源的有效状态决定。')
text(p, 32, 82, '固定Tenant / User / response-style；以下为连续状态推演。')
box(p, 32, 116, 300, ['首次确认', 'concise / revision=1', '创建条件：expected=0'])
box(p, 390, 116, 300, ['用户更新', 'detailed / revision=2', '更新条件：expected=1'])
box(p, 748, 116, 300, ['用户删除', '空值 / revision=3 / Deleted', '删除条件：expected=2'])
arrow(p, 'M332 177H390')
arrow(p, 'M690 177H748')
text(p, 32, 278, '写入侧：旧任务仍用expected=1提交 → ErrConflict，不能覆盖修订2或3。', 18)
box(p, 32, 318, 300, ['旧缓存或索引', '可能仍保存concise@1', '只提取候选键，不信任旧值'], '#f3f6f5')
box(p, 390, 318, 300, ['Resolve回查事实源', '可信Scope + 候选键', '核对修订、删除与有效期'])
box(p, 748, 318, 300, ['Memory.Input投影', '更新后：detailed@2', '删除后：空集合'])
arrow(p, 'M332 379H390')
arrow(p, 'M690 379H748')
text(p, 32, 488, '读取侧：缓存TTL未到，也不能恢复已经更新或删除的事实。', 18)
box(p, 32, 522, 1016, ['Context按当前任务使用有效偏好', '空集合按当前要求或默认方式继续；读取失败另行处理。输入正确仍需验证回答是否采用。'], '#f0f8f6')
arrow(p, 'M898 440V522')
text(p, 32, 640, '回查只形成一次读取快照；严格使用或交付时，还需处理随后发生的撤销。', 18)
save(p, 'memory-preference-walkthrough')
