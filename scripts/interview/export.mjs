import {workspaceFile,publicFile,repositoryRoot as root} from './workspace.mjs';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {project} from '../interview.mjs';
import {renderArticle} from './preview.mjs';
project().validate();
const [id,...flags]=process.argv.slice(2),wechat=flags.includes('--wechat');
if(!/^AE-\d{3}$/.test(id||'')||flags.some(f=>f!=='--wechat')||flags.length>1)throw new Error('用法：node scripts/interview/export.mjs AE-ID [--wechat]');
const data=JSON.parse(fs.readFileSync(path.join(root,'content/interview/questions.json'),'utf8'));
const q=data.questions.find(q=>q.id===id);if(!q)throw new Error('未知题号');
const article=publicFile(root,q.article),source=fs.readFileSync(article,'utf8');
let adaptation={};
if(wechat){
 const file=workspaceFile(`wechat/${id}/adaptation.json`);
 if(!fs.existsSync(file))throw new Error('缺少渠道映射，先登记栏目和章内编号');
 adaptation=JSON.parse(fs.readFileSync(file,'utf8'));
 const hash=crypto.createHash('sha256').update(source).digest('hex');
 if(adaptation.schema_version!==1||adaptation.id!==id||adaptation.article!==q.article||adaptation.article_sha256!==hash||!adaptation.title?.trim()||!Array.isArray(adaptation.omit_headings))throw new Error('渠道映射过期或无效，不能导出旧版本');
}
const html=renderArticle(source,{root,article,wechat,title:adaptation.title,omitHeadings:adaptation.omit_headings});
const output=workspaceFile(`exports/${id.toLowerCase()}${wechat?'-wechat':''}.html`);
fs.mkdirSync(path.dirname(output),{recursive:true});
// Never overwrite an existing user preview. Changed versions are kept under their content digest.
let target=output;
if(fs.existsSync(output)&&fs.readFileSync(output,'utf8')!==html)target=output.replace(/\.html$/,`-${crypto.createHash('sha256').update(html).digest('hex').slice(0,12)}.html`);
if(!fs.existsSync(target))fs.writeFileSync(target,html,{flag:'wx'});
console.log(`Exported ${(html.match(/<img /g)||[]).length} embedded images. ${wechat?'本地渠道预览，未操作公众号。':''}`);
console.log(target);
