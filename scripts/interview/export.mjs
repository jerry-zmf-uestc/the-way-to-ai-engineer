import {workspaceFile,publicFile,repositoryRoot as root} from './workspace.mjs';
import fs from 'node:fs';
import path from 'node:path';
import {project} from '../interview.mjs';
project().validate();
const id=process.argv[2];
if(!/^AE-\d{3}$/.test(id||'')) throw new Error('用法：node scripts/interview/export.mjs AE-ID');
const data=JSON.parse(fs.readFileSync(path.join(root,'content/interview/questions.json'),'utf8'));
const q=data.questions.find(q=>q.id===id);
if(!q) throw new Error('未知题号');
const article=publicFile(root,q.article);
const source=fs.readFileSync(article,'utf8');
if(/TODO|待填写|待核实/.test(source)) throw new Error('正文尚未完成，不能导出');
const esc=s=>s.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
const inline=s=>esc(s).replace(/\*\*(.+?)\*\*/g,'<strong style="color:#087f8c">$1</strong>');
const title=source.split('\n')[0].replace(/^# /,'');
const blocks=source.trim().split(/\n\s*\n/),html=[];
for(const block of blocks){
 if(block.startsWith('# ')){html.push(`<h1 style="font-size:26px;line-height:1.5;color:#16324f">${inline(title)}</h1>`);continue;}
 if(block.startsWith('## ')){html.push(`<h2 style="font-size:21px;line-height:1.6;color:#16324f;margin:36px 0 16px;border-left:4px solid #087f8c;padding-left:12px">${inline(block.slice(3))}</h2>`);continue;}
 const img=block.match(/^!\[([^\]]*)\]\(([^)]+)\)$/);
 if(img){const bytes=fs.readFileSync(publicFile(root,path.relative(root,path.resolve(path.dirname(article),img[2]))));html.push(`<p style="margin:24px 0"><img style="display:block;max-width:100%;height:auto" alt="${esc(img[1])}" src="data:image/png;base64,${bytes.toString('base64')}"></p>`);continue;}
 if(block.startsWith('|')){
  const rows=block.split('\n').filter(l=>!/^\|[\s|:-]+\|$/.test(l));
  html.push('<table style="width:100%;border-collapse:collapse;font-size:15px;line-height:1.8">'+rows.map((r,i)=>'<tr>'+r.split('|').slice(1,-1).map(c=>`<${i?'td':'th'} style="border:1px solid #d4e1e8;padding:10px;text-align:left;${i?'':'background:#edf5f7;color:#16324f;'}">${inline(c.trim())}</${i?'td':'th'}>`).join('')+'</tr>').join('')+'</table>');continue;
 }
 html.push(`<p style="margin:18px 0;line-height:1.95;font-size:16px;color:#283847;overflow-wrap:anywhere">${block.split('\n').map(line=>/^https:\/\//.test(line)?`<a style="color:#087f8c" href="${esc(line)}">${esc(line)}</a>`:inline(line)).join('<br>')}</p>`);
}
const result=`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${esc(title)}</title></head><body style="background:#f4f7fa;margin:0"><main style="max-width:680px;margin:0 auto;padding:28px 24px 56px;background:white;font-family:-apple-system,BlinkMacSystemFont,'PingFang SC','Microsoft YaHei',sans-serif">${html.join('\n')}</main></body></html>`;
const output=workspaceFile(`exports/${id.toLowerCase()}.html`);
fs.mkdirSync(path.dirname(output),{recursive:true});fs.writeFileSync(output,result);
console.log(`Exported ${blocks.length} blocks, ${html.filter(s=>s.includes('<img ')).length} embedded images.`);
