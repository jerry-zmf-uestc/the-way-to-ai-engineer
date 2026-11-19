# coding: utf-8
"""Original post-training data/optimization figure; standard-library static SVG."""
from pathlib import Path
from html import escape
parts=['''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 490" role="img" aria-labelledby="title desc"><title id="title">训练目标与应用执行边界</title><desc id="desc">预训练后可进行监督微调；典型奖励模型RLHF与直接偏好优化使用不同训练路径。应用仍检查协议、事实、权限和预算。</desc><defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10Z" fill="#475569"/></marker></defs><rect width="960" height="490" fill="#f8fafc"/><g font-family="system-ui,sans-serif" fill="#0f172a">''']
def text(x,y,s,size=16):parts.append(f'<text x="{x}" y="{y}" text-anchor="middle" font-size="{size}">{escape(s)}</text>')
def box(x,y,w,lines,color='#fff'):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="76" rx="10" fill="{color}" stroke="#64748b"/>')
    for n,s in enumerate(lines):text(x+w/2,y+31+n*24,s)
def arrow(d):parts.append(f'<path d="{d}" stroke="#475569" stroke-width="2" fill="none" marker-end="url(#a)"/>')
text(480,36,'训练改变模型倾向，应用决定结果能否交付或执行',22)
box(45,70,210,['预训练','语料上的Token预测'],'#dbeafe')
box(375,70,210,['SFT','指令与示范响应'],'#dbeafe')
arrow('M255 108H375')
box(65,205,350,['典型奖励模型RLHF路线','偏好排序 → 奖励模型 → RL优化'],'#fef3c7')
box(545,205,350,['DPO路线','偏好对 + 策略/参考对数概率'],'#fef3c7')
arrow('M430 146V175H240V205');arrow('M530 146V175H720V205')
box(230,350,500,['应用调用后的独立判断','协议 / 证据支持 / 工具授权 / 预算'],'#dcfce7')
arrow('M240 281V320H360V350');arrow('M720 281V320H600V350')
text(480,458,'偏好损失不是事实正确率；数据、目标与部署任务需分别检验',17)
parts.append('</g></svg>')
Path(__file__).with_suffix('.svg').write_text(''.join(parts),encoding='utf-8')
