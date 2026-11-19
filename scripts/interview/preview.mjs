import fs from 'node:fs';
import path from 'node:path';
import {publicFile} from './workspace.mjs';

export const escapeHTML=s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&#39;');
const classes=new Set(['request-diagram','request-group','request-highlight','request-label','request-code']);
export function safeNativeHTML(source){
  // Only the author's static request cards are accepted. No arbitrary styles, URLs or DOM attributes.
  const stack=[];let output='',cursor=0;
  for(const match of source.matchAll(/<[^>]*>/g)){
    const text=source.slice(cursor,match.index);
    if(/[<>]/.test(text))throw new Error('原生结构文本必须转义');
    output+=text;cursor=match.index+match[0].length;
    const tag=match[0];
    if(/^<br\s*\/?\s*>$/i.test(tag)){output+='<br>';continue;}
    const end=tag.match(/^<\/(section|p|mark)>$/);
    if(end){if(stack.pop()!==end[1])throw new Error('原生结构标签不成对');output+=tag;continue;}
    const start=tag.match(/^<(section|p|mark)(?: class="([a-z-]+)")?>$/);
    if(!start||start[2]&&!classes.has(start[2]))throw new Error('原生结构含未允许标签或属性');
    stack.push(start[1]);output+=tag;
  }
  const tail=source.slice(cursor);if(/[<>]/.test(tail)||stack.length)throw new Error('原生结构不完整');
  return output+tail;
}
export function imageData(root,article,target){
  if(/^(?:[a-z]+:|\/\/|\/)/i.test(target))throw new Error('配图必须使用仓库正式相对路径');
  const relative=path.relative(root,path.resolve(path.dirname(article),target));
  if(!relative.startsWith('public/diagrams/interview/')&&!relative.startsWith('public/images/interview/'))throw new Error('配图不在正式素材目录');
  return staticImageData(publicFile(root,relative));
}
export function staticImageData(file){
  const bytes=fs.readFileSync(file);
  let mime;
  if(bytes.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])))mime='image/png';
  else if(bytes[0]===255&&bytes[1]===216&&bytes[2]===255)mime='image/jpeg';
  else if(file.endsWith('.svg')){
    const svg=bytes.toString('utf8');
    if(!/<svg\b/.test(svg)||/<(?:script|foreignObject|image|animate|set)\b|\son\w+\s*=|(?:href|src)\s*=|@import|url\(|<!DOCTYPE|<!ENTITY/i.test(svg))throw new Error('SVG非静态安全素材');
    mime='image/svg+xml';
  } else throw new Error('图片文件格式无法识别');
  return `data:${mime};base64,${bytes.toString('base64')}`;
}
function urlSafe(target){try{const u=new URL(target);if(u.protocol!=='https:')throw new Error();return target;}catch{throw new Error('外链仅允许有效HTTPS地址');}}
function inline(source,root,article,imageResolver){
  // Match supported syntax before escaping; never interpret generated code as HTML.
  const rx=/!\[([^\]]*)\]\(([^)]+)\)|\[((?:[^\[\]]|\[[^\]]*\])+)\]\(([^)]+)\)|`([^`]+)`|\*\*([^*]+)\*\*|<mark>([\s\S]*?)<\/mark>|<br\s*\/?\s*>/g;
  let out='',cursor=0;
  for(const m of source.matchAll(rx)){
    out+=escapeHTML(source.slice(cursor,m.index));cursor=m.index+m[0].length;
    if(m[1]!==undefined){if(!m[1].trim())throw new Error('图片缺少替代文字');out+=`<img alt="${escapeHTML(m[1])}" src="${imageResolver?imageResolver(m[2]):imageData(root,article,m[2])}">`;}
    else if(m[3]!==undefined){const url=m[4];if(/^[a-z][a-z0-9+.-]*:/i.test(url)){out+=`<a href="${escapeHTML(urlSafe(url))}" rel="noreferrer">${escapeHTML(m[3])}</a>`;}else{const file=publicFile(root,path.relative(root,path.resolve(path.dirname(article),url.split('#')[0])));if(!fs.existsSync(file))throw new Error('正文相对链接缺失');out+=escapeHTML(m[3]);}}
    else if(m[5]!==undefined)out+=`<code>${escapeHTML(m[5])}</code>`;
    else if(m[6]!==undefined)out+=`<strong>${escapeHTML(m[6])}</strong>`;
    else if(m[7]!==undefined)out+=`<mark>${escapeHTML(m[7])}</mark>`;
    else out+='<br>';
  }
  return out+escapeHTML(source.slice(cursor));
}
function cells(row){return row.trim().replace(/^\|/,'').replace(/\|$/,'').split(/(?<!\\)\|/).map(c=>c.trim().replace(/\\\|/g,'|'));}
export function renderArticle(source,{root,article,wechat=false,title=null,omitHeadings=[],imageResolver=null}={}){
  if(!source.startsWith('# ')||/TODO|待填写|待核实/.test(source))throw new Error('正文不完整，不能预览');
  if(/<whiteboard\b|<table\b|<(?:script|iframe|style|object)\b/i.test(source))throw new Error('未转换白板、原生表格或活动HTML不能预览');
  const lines=source.replace(/\r\n/g,'\n').split('\n'),html=[];
  const pageTitle=title||lines[0].slice(2).trim();let skip=false,first=true,refs=false;
  for(let i=0;i<lines.length;){
    const line=lines[i];
    if(!line.trim()){i++;continue;}
    const heading=line.match(/^(#{1,3}) (.+)$/);
    if(heading){const text=heading[2];skip=omitHeadings.includes(text);if(skip){i++;continue;}if(first){html.push(`<header>${escapeHTML(pageTitle)}</header>`);first=false;}else{refs=text==='参考资料'||text==='参考文档';html.push(`<h${heading[1].length}>${inline(text,root,article,imageResolver)}</h${heading[1].length}>`);}i++;continue;}
    if(skip){i++;continue;}
    if(line.startsWith('```')){
      const language=line.slice(3).trim();if(!/^[a-z0-9+-]*$/i.test(language))throw new Error('代码语言标识无效');
      const code=[];i++;while(i<lines.length&&!/^```\s*$/.test(lines[i]))code.push(lines[i++]);
      if(i===lines.length)throw new Error('代码围栏未闭合');i++;
      html.push(`<pre data-lang="${escapeHTML(language)}"><code>${escapeHTML(code.join('\n'))}</code></pre>`);continue;
    }
    if(line.startsWith('<section class="request-diagram">')){
      const block=[];do{block.push(lines[i++]);}while(i<lines.length&&lines[i].trim());html.push(safeNativeHTML(block.join('\n')));continue;
    }
    if(line.startsWith('|')){
      const rows=[];while(i<lines.length&&lines[i].startsWith('|'))rows.push(lines[i++]);
      if(rows.length<2||!/^\|[\s|:-]+\|$/.test(rows[1]))throw new Error('表格缺少分隔行');
      const width=cells(rows[0]).length,data=rows.filter((_,n)=>n!==1).map(cells);
      if(!width||data.some(r=>r.length!==width))throw new Error('表格列数不一致');
      const cols=width===2?[25,75]:width===4?[16,28,28,28]:Array(width).fill(100/width);
      html.push(`<div class="table-scroll columns-${width}"><table><colgroup>`+cols.map(w=>`<col style="width:${w}%">`).join('')+'</colgroup>'+data.map((r,n)=>'<tr>'+r.map(c=>`<${n?'td':'th'}>${inline(c,root,article,imageResolver)}</${n?'td':'th'}>`).join('')+'</tr>').join('')+'</table></div>');continue;
    }
    if(/^\s*[-*] |^\d+\. /.test(line)){
      const ordered=/^\d+\. /.test(line),items=[];
      while(i<lines.length&&(ordered?/^\d+\. /:/^\s*[-*] /).test(lines[i]))items.push(lines[i++].replace(ordered?/^\d+\. /:/^\s*[-*] /,''));
      html.push(`<${ordered?'ol':'ul'} class="${refs?'references':''}">`+items.map(t=>'<li>'+inline(t,root,article,imageResolver)+'</li>').join('')+`</${ordered?'ol':'ul'}>`);continue;
    }
    if(line.startsWith('> ')){html.push('<blockquote>'+inline(line.slice(2),root,article,imageResolver)+'</blockquote>');i++;continue;}
    const para=[];while(i<lines.length&&lines[i].trim()&&!/^(?:#{1,3} |```|\||<section|> |[-*] |\d+\. )/.test(lines[i]))para.push(lines[i++]);
    if(!para.length)throw new Error('无法解析正文块');
    const text=para.join('\n');if(/<(?!mark>|\/mark>|br\s*\/?\s*>)/i.test(text))throw new Error('不支持的HTML正文');
    const style=/^图\d+[：:]/.test(text)?'caption':refs?'references':'body-text';
    html.push(`<p class="${style}">`+inline(text,root,article,imageResolver).replace(/\n/g,'<br>')+'</p>');
  }
  const size=wechat?15:16;
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${escapeHTML(pageTitle)}</title><style>
body{margin:0;background:#fff;color:#333;font-family:"PingFang SC","Microsoft YaHei",sans-serif}main{max-width:680px;margin:auto;padding:24px 20px 48px;overflow-wrap:anywhere}header{font-size:${wechat?20:24}px;line-height:${wechat?'30px':'36px'};font-weight:600;margin-bottom:24px}p,li{font-size:${size}px;line-height:${wechat?'28.5px':'1.95'};letter-spacing:.2px}p{margin:0 0 14px}h1{font-size:${wechat?20:26}px;line-height:30px;margin:32px 0 14px}h2{font-size:${wechat?18:21}px;line-height:27px;margin:28px 0 12px}h3{font-size:16px;line-height:25.6px;margin:22px 0 10px}h1,h2,h3{font-weight:600;color:#1f2329}a{color:#2d6a5b;overflow-wrap:anywhere}mark{background:transparent;color:#2d6a5b;font-weight:600}img{max-width:100%;height:auto;display:block;margin:18px auto}pre{font:14px/26px Menlo,Consolas,monospace;overflow-x:auto;background:#f5f6f7;padding:12px;white-space:pre}pre code{font:inherit;white-space:pre}code{font:14px Menlo,Consolas,monospace}.table-scroll{overflow-x:auto;margin:18px 0}table{width:100%;border-collapse:collapse;table-layout:fixed;font-size:14px;line-height:23.8px}td,th{border:1px solid #d0d7de;padding:8px;text-align:left;vertical-align:top;overflow-wrap:anywhere}th{background:#f5f6f7}td img{min-width:0;width:100%}.caption{font-size:13px;line-height:23.4px;color:#777777;text-align:center}.references,.references li{font-size:14px;line-height:25.2px;color:#667085;letter-spacing:0}blockquote{color:#667085;margin:12px 0;padding-left:12px;border-left:2px solid #d0d7de}.request-diagram{border:1px solid #cbe3dc;background:#f1f8f5;padding:16px;margin:18px 0}.request-group{border:1px solid #b8d7cf;background:white;padding:12px;margin:8px 0}.request-highlight{border:2px solid #0f766e;background:#e5f5ef;padding:12px;margin:8px 0}.request-label{font-size:14px;line-height:24px;color:#2d6a5b;margin-bottom:6px}.request-code{font:14px/24px Menlo,Consolas,monospace;white-space:pre-wrap;overflow-wrap:anywhere;margin:0}
@media(max-width:500px){main{padding:20px 16px}table{min-width:340px}.columns-3 table{min-width:540px}.columns-4 table{min-width:620px}.request-diagram{padding:10px}.request-group,.request-highlight{padding:8px}}@media print{main{max-width:none}pre{white-space:pre-wrap}.table-scroll{overflow:visible}}
</style></head><body><main>${wechat?'<p class="caption">本地渠道预览；不代表公众号已保存或原生代码验收。</p>':''}${html.join('\n')}</main></body></html>`;
}
