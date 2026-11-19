export function enhanceReading(root, clipboard, pageUrl, base) {
  const document = root.ownerDocument;
  for (const pre of root.querySelectorAll('pre')) {
    const code = pre.querySelector('code');
    if (!code || pre.dataset.copyEnhanced) continue;
    pre.dataset.copyEnhanced = 'true';
    const wrapper = document.createElement('div');
    wrapper.className = 'reading-codeblock';
    const toolbar = document.createElement('div');
    toolbar.className = 'code-toolbar';
    const status = document.createElement('span');
    status.className = 'copy-status';
    status.setAttribute('role', 'status');
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'copy-code';
    button.textContent = '复制代码';
    button.addEventListener('click', async () => {
      if (button.disabled) return;
      button.disabled = true;
      status.textContent = '正在复制';
      try {
        if (!clipboard?.writeText) throw new Error('clipboard unavailable');
        await clipboard.writeText(code.textContent ?? '');
        status.textContent = '已复制';
      } catch {
        status.textContent = '未复制，请选中代码手动复制';
      } finally {
        button.disabled = false;
      }
    });
    toolbar.append(status, button);
    pre.before(wrapper);
    wrapper.append(toolbar, pre);
  }
  for (const img of root.querySelectorAll('img')) {
    if (img.closest('a')) continue;
    let url;
    try { url = new URL(img.getAttribute('src') ?? '', pageUrl); } catch { continue; }
    if (url.origin !== new URL(pageUrl).origin || !url.pathname.startsWith(`${base}/diagrams/`) || !url.pathname.endsWith('.svg')) continue;
    const link = document.createElement('a');
    link.className = 'diagram-original';
    link.href = url.href;
    link.target = '_blank';
    link.rel = 'noopener';
    link.setAttribute('aria-label', `查看原图：${img.getAttribute('alt') || '图示'}（新窗口）`);
    const hint = document.createElement('span');
    hint.className = 'diagram-hint';
    hint.textContent = '查看原图 ↗';
    img.before(link);
    link.append(img, hint);
  }
}
