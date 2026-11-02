"""Original editable SVG; Python 3 standard library, rebuild next to this source."""
from pathlib import Path

SVG = """<svg xmlns="http://www.w3.org/2000/svg" width="900" height="440" viewBox="0 0 900 440" role="img" aria-labelledby="title desc">
  <title id="title">有界搜索：评价错误与保留宽度</title>
  <desc id="desc">根状态产生dead和bridge两个候选，启发式错误地给dead更高分。宽度一剪掉bridge后耗尽；宽度二保留bridge，可在下一层发现answer。生成节点上限二仍会阻止第三个节点，宽度不创造新预算。</desc>
  <defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0 L10 5 L0 10Z" fill="#475569"/></marker></defs>
  <rect width="900" height="440" rx="16" fill="#f8fafc"/>
  <g font-family="system-ui,sans-serif" fill="#0f172a">
    <text x="30" y="40" font-size="24">有界搜索：高分候选也可能走不通</text>
    <text x="30" y="73" font-size="16">每个分支保存独立路径；最终验证与启发式评分分开。</text>
    <text x="45" y="145" font-size="17">深度 0</text>
    <text x="340" y="95" font-size="17">深度 1</text>
    <text x="665" y="240" font-size="17">深度 2</text>
    <rect x="30" y="165" width="200" height="80" rx="10" fill="#ffffff" stroke="#64748b"/>
    <text x="50" y="199" font-size="20">root：初始状态</text>
    <text x="50" y="226" font-size="16">提出两个候选</text>
    <rect x="325" y="110" width="230" height="80" rx="10" fill="#ffedd5" stroke="#64748b"/>
    <text x="345" y="142" font-size="20">dead：评分 9</text>
    <text x="345" y="171" font-size="16">没有后继，未满足目标</text>
    <rect x="325" y="255" width="230" height="80" rx="10" fill="#dbeafe" stroke="#64748b"/>
    <text x="345" y="287" font-size="20">bridge：评分 3</text>
    <text x="345" y="317" font-size="16">下一层仍有可用候选</text>
    <rect x="650" y="255" width="220" height="80" rx="10" fill="#dcfce7" stroke="#64748b"/>
    <text x="670" y="288" font-size="20">answer</text>
    <text x="670" y="317" font-size="16">独立目标验证通过</text>
    <g fill="none" stroke="#475569" stroke-width="2" marker-end="url(#arrow)">
      <path d="M230 205 H278 V150 H325"/>
      <path d="M230 205 H278 V295 H325"/>
      <path d="M555 295 H650"/>
    </g>
    <text x="30" y="376" font-size="17">width=1剪掉bridge；width=2保留它，但不保证其他任务也能找到解。</text>
    <text x="30" y="406" font-size="17">nodes=2已被首层候选用完；生成answer仍需剩余总预算。</text>
  </g>
</svg>
"""

Path(__file__).with_suffix(".svg").write_text(SVG, encoding="utf-8")
