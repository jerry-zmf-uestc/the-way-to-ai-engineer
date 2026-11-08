import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {workspaceRoot,workspaceFile,publicFile,repositoryRoot} from './workspace.mjs';
import {issueProject} from './issue.mjs';
import {safeNativeHTML} from './preview.mjs';
const digest=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
export function checkWechat(root=repositoryRoot,env=process.env){
 const catalogue=JSON.parse(fs.readFileSync(path.join(root,'content/interview/questions.json'),'utf8'));
 const state=JSON.parse(fs.readFileSync(path.join(root,'content/interview/state.json'),'utf8'));
 const file=workspaceFile('wechat/registry.json',env);
 if(!fs.existsSync(file))throw new Error('私有公众号登记不存在');
 const registry=JSON.parse(fs.readFileSync(file,'utf8')),errors=[],pending=[],numbers=new Set();
 if(registry.schema_version!==2||!registry.items||Array.isArray(registry.items))throw new Error('公众号登记schema无效');
 for(const [id,item] of Object.entries(registry.items)){
  const q=catalogue.questions.find(q=>q.id===id);
  if(!q||item.article!==q.article){errors.push(`${id} 正文路径与题库不一致`);continue;}
  if(!item.section?.trim()||!/^\d{2}$/.test(item.section_number)||item.section_number==='00'||!item.title?.startsWith(`Agent 每日一题｜${item.section} ${item.section_number}：`))errors.push(`${id} 栏目标题或章内编号无效`);
  const key=item.section+' '+item.section_number;if(numbers.has(key))errors.push(`${id} 栏目编号重复`);numbers.add(key);
  const article=publicFile(root,q.article);
  if(!fs.existsSync(article)){errors.push(`${id} 正文缺失`);continue;}
  if(item.body_sha256!==digest(article))errors.push(`${id} 登记正文摘要过期`);
  if(item.platform_status!=='unverified'&&item.platform_status!=='verified')errors.push(`${id} 平台核验状态无效`);
  if(item.platform_status==='verified'&&!item.wechat_verified_at)errors.push(`${id} 缺少平台核验时间`);
  if(item.platform_status==='unverified')pending.push(`${id} 当前公众号草稿与发表记录未核验`);
  for(const rel of item.evidence||[]){
   try{if(!fs.existsSync(workspaceFile('wechat/'+rel,env)))errors.push(`${id} 登记证据缺失`);}catch{errors.push(`${id} 登记证据路径越界`);}
  }
  try{
   const adaptation=JSON.parse(fs.readFileSync(workspaceFile('wechat/'+item.adaptation,env),'utf8'));
   if(adaptation.id!==id||adaptation.article!==q.article||adaptation.article_sha256!==digest(article)||adaptation.title!==item.title)errors.push(`${id} 渠道适配绑定过期`);
  }catch{errors.push(`${id} 缺少有效渠道适配`);}
  const text=fs.readFileSync(article,'utf8');
  for(const block of text.matchAll(/<section class="request-diagram">[\s\S]*?(?=\n\n|$)/g)){try{safeNativeHTML(block[0]);}catch{errors.push(`${id} 原生请求图不安全`);}}
  if(/<whiteboard\b|https:\/\/feishu\.cn\/file\//.test(text))errors.push(`${id} 存在未适配媒体`);
  const research=workspaceFile(`research/${id}.md`,env),review=workspaceFile(`reviews/${id}.json`,env);
  if(!fs.existsSync(research)||!fs.existsSync(review))pending.push(`${id} 研究/审核记录缺失`);
  else{
   const r=JSON.parse(fs.readFileSync(review,'utf8'));
   if(r.article_sha256!==digest(article))errors.push(`${id} 审核摘要过期`);
   const ranks={design:1,runnable:2,measured:3};
   const ready=r.id===id&&r.reviewer?.trim()&&r.evidence_notes?.trim()&&r.notes?.trim()&&ranks[r.evidence_level]>=ranks[q.minimum_evidence]&&['facts','engineering','clarity','sources','safety'].every(k=>r.checks?.[k]===true);
   if(!ready){pending.push(`${id} 当前内容/工程审核尚未通过`);if(['ready','published'].includes(state.items[id].status))errors.push(`${id} ready/published缺乏合格私有审核`);}
  }
 }
 for(const w of issueProject(root,env).workflows()){
  if(w.phase==='writing'){pending.push(`${w.id} 本期写作中，尚未审核、保存验收与归档`);continue;}
  try{const result=issueProject(root,env).check(w.id);if(['reviewed','wechat_saved_verified'].includes(result.phase))pending.push(`${w.id} 本期尚未完成保存验收与归档`);}catch(e){errors.push(`${w.id} 本期版本检查失败：${e.message}`);}
 }
 return {schema_version:1,ok:errors.length===0,errors,pending,note:'只读本地登记检查；不操作公众号，不证明事实或真实模型收益。'};
}
