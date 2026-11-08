"""Original information operations and conditional lifecycle changes."""
from html import escape
from pathlib import Path


def canvas(title, description):
    return ['<svg xmlns="http://www.w3.org/2000/svg" width="1000" height="690" viewBox="0 0 1000 690" role="img" aria-labelledby="title desc">',
            f'<title id="title">{escape(title)}</title><desc id="desc">{escape(description)}</desc>',
            '<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs>',
            '<rect width="1000" height="690" rx="16" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">',
            f'<text x="30" y="40" font-size="25">{escape(title)}</text>']


def text(p, x, y, value, size=16):
    p.append(f'<text x="{x}" y="{y}" font-size="{size}">{escape(value)}</text>')


def box(p, x, y, width, lines, color='#dbeafe'):
    height = 30 + 27*len(lines)
    p.append(f'<rect x="{x}" y="{y}" width="{width}" height="{height}" rx="10" fill="{color}" stroke="#64748b"/>')
    for n, value in enumerate(lines):
        text(p, x+15, y+30+27*n, value, 19 if n == 0 else 16)


def arrow(p, path, x, y, label):
    p.append(f'<path d="{path}" fill="none" stroke="#475569" stroke-width="2" marker-end="url(#a)"/>')
    text(p, x, y, label, 15)


def save(p, name):
    Path(__file__).with_name(name+'.svg').write_text('\n'.join(p+['</g></svg>\n']), encoding='utf-8')


p = canvas('Memory：读取与生命周期变更分开', 'CAS写入有效事实；有效记录按需召回，到期不返回。删除需独立授权，清空值并保留修订；再确认基于墓碑修订写入。读取不会触发删除。')
text(p, 30, 73, '箭头标注操作或触发条件；召回不自动删除，也不自动延长信息期限。')
box(p, 30, 120, 280, ['候选与具体授权', 'Scope / 值 / 操作 / 期限', '字段检查 → CAS'])
box(p, 355, 120, 290, ['有效记录', 'Value / Source / Revision', 'ExpiresAt，Deleted=false'])
arrow(p, 'M310 168H355', 312, 109, '写入')
box(p, 690, 120, 280, ['到期记录', 'Recall / Resolve不返回', '物理行未必已清理'], '#fef3c7')
arrow(p, 'M645 168H690', 650, 109, '到期')
box(p, 355, 330, 290, ['按需召回或候选回查', '当前Scope / 期限 / 状态', '返回快照，再由Context选择'], '#dcfce7')
arrow(p, 'M500 231V330', 520, 277, '读取；不修改Revision')
box(p, 30, 500, 280, ['删除墓碑', 'Value与Source清空', 'Deleted=true；Revision增加'], '#fee2e2')
arrow(p, 'M355 197H330V550H310', 30, 484, '删除需独立授权与当前Revision')
arrow(p, 'M310 565H685V203H645', 350, 594, '重新确认 + 墓碑Revision → CAS写入')
text(p, 30, 636, '旧候选只定位；必须回查事实源。过期、删除与数据库失败不混成同一结果。')
text(p, 30, 670, '这不是全局原子授权；SQL行为、语义召回与长期任务收益分别验证。')
save(p, 'memory-lifecycle')

p = canvas('Workspace：读取不自动清理产物', '应用发布带身份、摘要与期限的产物。有效期内按需读取身份与范围明确的页面，到期拒绝访问；应用清理成功才删除文件和释放字节，保留ID墓碑。')
text(p, 30, 73, '可信Scope绑定每任务Root；模型只取得注册ID，物理路径由应用管理。')
box(p, 30, 120, 280, ['应用发布', '有界文本 → 私有文件', 'ID / 摘要 / 大小 / 期限'])
box(p, 355, 120, 290, ['有效产物目录', '登记ID，不静默复用', '应用持有内容与访问状态'])
arrow(p, 'M310 168H355', 312, 109, '登记')
box(p, 690, 120, 280, ['按需读取页面', 'ArtifactID / Digest', 'StartLine / NextLine', 'Complete只说明到达EOF'], '#dcfce7')
arrow(p, 'M645 168H690', 650, 109, '读取')
box(p, 355, 330, 290, ['到期：Read拒绝，List不展示', '不会自动删除物理文件', '已有字节仍计入Used'], '#fef3c7')
arrow(p, 'M500 231V330', 520, 277, '期限到达')
box(p, 355, 520, 290, ['应用Cleanup', '删除成功才释放Used', 'revoked墓碑保留，ID不复用'], '#ffffff')
arrow(p, 'M500 441V520', 520, 477, '可信维护过程')
text(p, 30, 484, '清理失败：保留记录与配额，可复验', 15)
text(p, 30, 651, '内存catalog不提供重启恢复或多实例配额；Root不是进程沙箱。')
text(p, 30, 677, '页面与摘要不提供授权，也不证明模型读过全部内容或已完成任务。')
save(p, 'workspace-artifacts')

p = canvas('Memory：删除缓存不能阻止旧读取回填', '受控顺序：A读到修订1后暂停；事实源更新为修订2并清除缓存；A晚到写回修订1。候选回查仍以当前Scope和事实源修订为准。这不是Redis或分布式竞态的运行证据。')
text(p, 30, 73, '缓存仍在TTL内，也可能已经过时；箭头编号表示先后顺序。')
box(p, 30, 110, 280, ['读请求A', '先取得旧快照，随后回填'])
box(p, 355, 110, 290, ['权威事实源', 'Scope / Key / Revision'])
box(p, 690, 110, 280, ['派生缓存', '清除不意味着旧请求停止'])
for x in (170, 500, 835):
    p.append(f'<path d="M{x} 194V620" stroke="#94a3b8" stroke-width="1" fill="none"/>')
arrow(p, 'M170 235H500', 195, 222, '1. 请求读取偏好')
arrow(p, 'M500 285H170', 195, 272, '2. 返回revision=1；尚未回填')
box(p, 355, 315, 290, ['新确认提交revision=2', '新值detailed'], '#dcfce7')
arrow(p, 'M500 425H835', 520, 412, '3. 更新以后清除缓存')
arrow(p, 'M170 490H835', 240, 477, '4. A迟到回填revision=1 / concise')
arrow(p, 'M835 545H500', 515, 532, '5. 候选键回查当前事实源')
box(p, 355, 565, 290, ['回查取得revision=2', '再核对范围、删除与期限'], '#dcfce7')
box(p, 690, 565, 280, ['缓存旧值尚未到期', '不能直接当作当前记忆'], '#fee2e2')
text(p, 30, 665, '受控顺序反例；回查增加存储成本，也不消除读取之后与模型消费之间的窗口。')
save(p, 'memory-cache-race')
