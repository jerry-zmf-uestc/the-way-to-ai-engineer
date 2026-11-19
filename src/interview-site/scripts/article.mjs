// Add presentation semantics only; never change the archived author's text.
export function formatArticle(prose){
  let references=false;
  for(const node of prose.children){
    if(/^H[1-3]$/.test(node.tagName))references=['参考资料','参考文档'].includes(node.textContent.trim());
    if(['P','UL','OL'].includes(node.tagName)){
      if(references)node.classList.add('references');
      else if(node.tagName==='P'&&/^图\s*\d+[：:]/.test(node.textContent.trim()))node.classList.add('caption');
    }
  }
}
