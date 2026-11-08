import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath} from 'node:url';
import {isDeepStrictEqual} from 'node:util';
import {workspaceFile,publicFile,repositoryRoot,workspaceRoot} from './workspace.mjs';
import {renderArticle,staticImageData} from './preview.mjs';
import {project} from '../interview.mjs';

export const digest=bytes=>crypto.createHash('sha256').update(bytes).digest('hex');
const json=file=>JSON.parse(fs.readFileSync(file,'utf8'));
const bytes=file=>fs.existsSync(file)?fs.readFileSync(file):null;
const hash=file=>fs.existsSync(file)?digest(fs.readFileSync(file)):null;
const encode=value=>JSON.stringify(value,null,2)+'\n';
const assert=(condition,message)=>{if(!condition)throw new Error(message);};
const validDate=s=>typeof s==='string'&&Number.isFinite(Date.parse(s));
const ranks={design:1,runnable:2,measured:3};
const checks=['facts','engineering','clarity','sources','safety'];
const sections={'reasoning':'Reasoning&Planning','tool-calling':'Tool Calling',memory:'Context&Memory',rag:'RAG',eval:'Eval&Observability',runtime:'Agent Runtime/Harness','multi-agent':'Multi-Agent',mcp:'MCP/External Tools',production:'Production Engineering','system-design':'Agent System Design'};
const fingerprint=q=>digest(encode({title:q.title,module:q.module,article:q.article,order:q.order,prerequisites:q.prerequisites,focus:q.focus,minimum_evidence:q.minimum_evidence,acceptance_case:q.acceptance_case,supplemental_topics:q.supplemental_topics||[]}));
function atomic(file,data){fs.mkdirSync(path.dirname(file),{recursive:true});const tmp=file+'.'+crypto.randomUUID()+'.tmp';try{fs.writeFileSync(tmp,data,{flag:'wx'});fs.renameSync(tmp,file);}finally{if(fs.existsSync(tmp))fs.unlinkSync(tmp);}}
function safeText(s){assert(typeof s==='string'&&!/[\u0000-\u0008]/.test(s),'文本格式无效');assert(!/(?:\/Users\/|\/Volumes\/|sk-[A-Za-z0-9_-]{12,}|[?&](?:token|ticket|cookie)=|-----BEGIN [A-Z ]*PRIVATE KEY-----)/i.test(s),'内容含本机路径或秘密形态，拒绝归档');return s;}
const decode=s=>s.replace(/&#(\d+);/g,(_,n)=>String.fromCodePoint(Number(n))).replaceAll('&lt;','<').replaceAll('&gt;','>').replaceAll('&quot;','"').replaceAll('&#39;',"'").replaceAll('&amp;','&');
const plain=s=>decode(s.replace(/<br\s*\/?\s*>/g,'\n').replace(/<[^>]*>/g,'')).replace(/\s+/g,' ').trim();

// Stable comparison projection. It checks recorded content, not the authenticity of platform observations.
export function contentProjection(source,{root,article,media}){
 const rendered=renderArticle(source,{root,article,wechat:true,imageResolver:target=>{
  const dest=path.relative(root,path.resolve(path.dirname(article),target)).split(path.sep).join('/');
  const entry=media.find(m=>m.destination===dest);assert(entry,'正文图片未在媒体清单登记');return staticImageData(entry.file);
 }});
 const body=rendered.split('<main>')[1].split('</main>')[0].replace(/^<p class="caption">本地渠道预览[^<]*<\/p>/,'');
 const codes=[...body.matchAll(/<pre data-lang="([^"]*)"><code>([\s\S]*?)<\/code><\/pre>/g)].map(m=>({language:m[1],text:decode(m[2])}));
 const tables=[...body.matchAll(/<table>([\s\S]*?)<\/table>/g)].map(m=>[...m[1].matchAll(/<tr>([\s\S]*?)<\/tr>/g)].map(r=>[...r[1].matchAll(/<(?:td|th)>([\s\S]*?)<\/(?:td|th)>/g)].map(c=>plain(c[1]))));
 const images=[...source.matchAll(/!\[([^\]]*)\]\(([^)]+)\)/g)].map(m=>{const dest=path.relative(root,path.resolve(path.dirname(article),m[2])).split(path.sep).join('/');return {alt:m[1],destination:dest,sha256:media.find(x=>x.destination===dest).sha256};});
 const native=[...source.matchAll(/<section class="request-diagram">[\s\S]*?(?=\n\n|$)/g)].map(m=>plain(m[0]));
 const text=plain(body.replace(/<pre[\s\S]*?<\/pre>/g,'').replace(/<table>[\s\S]*?<\/table>/g,'').replace(/<img[^>]*>/g,''));
 const headings=[...body.matchAll(/<h([123])>([\s\S]*?)<\/h\1>/g)].map(m=>({level:Number(m[1]),text:plain(m[2])}));
 const paragraphs=[...body.matchAll(/<p class="(body-text|caption|references)">([\s\S]*?)<\/p>/g)].map(m=>({kind:m[1],text:plain(m[2])}));
 const sequence=[...body.matchAll(/<(h[123]|pre|table|img)\b|<p class="(body-text|caption|references)"/g)].map(m=>m[1]||m[2]);
 const covers=media.filter(m=>['cover','square_cover'].includes(m.role)).map(m=>({role:m.role,destination:m.destination,sha256:m.sha256}));
 const links=[...body.matchAll(/<a href="([^"]+)"[^>]*>([\s\S]*?)<\/a>/g)].map(m=>({text:plain(m[2]),href:decode(m[1])}));
 return {title:source.split('\n')[0].slice(2).trim(),text,headings,paragraphs,sequence,codes,tables,images,native,covers,links};
}

export function issueProject(root=repositoryRoot,env=process.env){
 root=fs.realpathSync(root);
 const privateRoot=workspaceRoot(env);let ancestor=privateRoot;while(!fs.existsSync(ancestor))ancestor=path.dirname(ancestor);const actual=path.join(fs.realpathSync(ancestor),path.relative(ancestor,privateRoot));assert(actual!==root&&!actual.startsWith(root+path.sep),'私有工作区不能位于目标仓库');
 const catalogue=()=>json(publicFile(root,'content/interview/questions.json'));
 const question=id=>{assert(/^AE-\d{3}$/.test(id||''),'题号格式无效');const q=catalogue().questions.find(q=>q.id===id);assert(q,'未知题号');return q;};
 const privateFile=rel=>workspaceFile(rel,env);
 const base=id=>'wechat/'+id;
 function ownFile(id,relative){assert(typeof relative==='string'&&!path.isAbsolute(relative)&&!relative.split(/[\\/]/).includes('..'),'工作文件必须位于本题私有目录');const file=privateFile(base(id)+'/'+relative),dir=fs.realpathSync(privateFile(base(id)));let ancestor=file;while(!fs.existsSync(ancestor))ancestor=path.dirname(ancestor);const resolved=path.join(fs.realpathSync(ancestor),path.relative(ancestor,file));assert(resolved.startsWith(dir+path.sep),'工作文件通过符号链接离开本题目录');return file;}
 const wfFile=id=>privateFile(base(id)+'/workflow.json');
 function load(id){const q=question(id),w=json(wfFile(id));assert(w.schema_version===1&&w.id===id,'本期记录无效');assert(w.topic_sha256===fingerprint(q),'本期主题或验收已变化，请先明确修订方案');return {q,w};}
 function lock(fn){const file=privateFile('wechat/.issue.lock');fs.mkdirSync(path.dirname(file),{recursive:true});let fd;try{fd=fs.openSync(file,'wx');return fn();}finally{if(fd!==undefined){fs.closeSync(fd);fs.unlinkSync(file);}}}
 function registry(){const file=privateFile('wechat/registry.json');return fs.existsSync(file)?json(file):{schema_version:2,items:{}};}
 function workflows(){const dir=privateFile('wechat');return fs.existsSync(dir)?fs.readdirSync(dir).filter(id=>/^AE-\d{3}$/.test(id)&&fs.existsSync(wfFile(id))).map(id=>({id,...json(wfFile(id))})):[];}
 function next(){const data=catalogue(),r=registry(),ws=workflows();const q=[...data.questions].sort((a,b)=>a.order-b.order).find(q=>!r.items[q.id]&&!ws.some(w=>w.id===q.id)&&!fs.existsSync(publicFile(root,q.article)));return {next:q?{id:q.id,order:q.order,title:q.title,prerequisites:q.prerequisites}:null,pending:[...data.questions].sort((a,b)=>a.order-b.order).filter(q=>r.items[q.id]||ws.some(w=>w.id===q.id)||fs.existsSync(publicFile(root,q.article))).map(q=>({id:q.id,title:q.title,phase:ws.find(w=>w.id===q.id)?.phase||'legacy_unverified'})),note:'按列表顺序；前置仅提示准备资料，历史待核验不阻挡新稿。'};}
 function start(id,{skipReason=null}={}){return lock(()=>{
  const q=question(id);assert(!fs.existsSync(wfFile(id)),'本期已存在，使用status或revise续接');const n=next().next;const reg=registry(),old=reg.items[id];
  if(!old&&!fs.existsSync(publicFile(root,q.article))&&n?.id!==id)assert(skipReason?.trim(),'非下一题：须填写作者确认的跳题理由');
  const section=old?.section||sections[q.module];assert(section,'缺少章节显示名');
  const occupied=[...Object.values(reg.items),...workflows()].filter(x=>x.section===section).map(x=>Number(x.section_number));
  const number=old?.section_number||String(Math.max(0,...occupied)+1).padStart(2,'0');assert(/^\d{2}$/.test(number)&&number!=='00','章节编号超出范围');
  assert(!workflows().some(w=>w.section===section&&w.section_number===number),'章内编号重复');
  const title=old?.title||`Agent 每日一题｜${section} ${number}：${q.title}`;
  const w={schema_version:1,id,topic_sha256:fingerprint(q),section,section_number:number,title,legacy_title:!!old,phase:'writing',revision:1,base_article_sha256:hash(publicFile(root,q.article)),base_mapping_sha256:digest(encode(old||null)),history:[{phase:'writing',at:new Date().toISOString(),reason:skipReason||'本期准备'}]};
  const dir=privateFile(base(id)+'/working');assert(!fs.existsSync(dir)&&!fs.existsSync(privateFile(base(id)+'/plan.md')),'已有私有working或plan，拒绝覆盖');fs.mkdirSync(dir,{recursive:true});
  const source=bytes(publicFile(root,q.article));const template=fs.readFileSync(publicFile(root,'docs/templates/interview-article.md'),'utf8').replace(/^# .*\n/,'# '+title+'\n').replace(/\{\{(id|title|focus|proposal_id|minimum_evidence|acceptance_case)\}\}/g,(_,key)=>q[key]);
  atomic(path.join(dir,'article.md'),source?source.toString('utf8').replace(/^# .*\r?\n/,'# '+title+'\n'):template);atomic(path.join(dir,'media-manifest.json'),encode({schema_version:1,items:[]}));
  atomic(path.join(dir,'research.md'),fs.readFileSync(publicFile(root,'docs/templates/interview-research.md'),'utf8').replace(/\{\{(id|title|focus|proposal_id|minimum_evidence|acceptance_case)\}\}/g,(_,key)=>q[key]));
  atomic(path.join(dir,'review.json'),encode({id,reviewer:'',reviewed_at:null,article_sha256:null,media_sha256:null,notes:'',evidence_level:null,evidence_notes:'',checks:Object.fromEntries(checks.map(k=>[k,false]))}));
  atomic(privateFile(base(id)+'/plan.md'),`# ${title}\n\n本期主题：${q.title}\n\n重点：${q.focus}\n\n最低证据：${q.minimum_evidence}\n\n前置资料：${q.prerequisites.join('、')||'无'}\n\nTODO：本期提纲、图和代码需求、授权范围及验收。\n`);atomic(wfFile(id),encode(w));return {id,title,phase:w.phase,note:'仅创建私有候选，未创建正式正文或操作公众号。'};
 });}
 function candidate(id){const {q,w}=load(id);const source=fs.readFileSync(privateFile(base(id)+'/working/article.md'),'utf8');safeText(source);assert(source.split('\n')[0]==='# '+w.title,'候选标题与锁定本期不一致');
  const mf=json(privateFile(base(id)+'/working/media-manifest.json'));assert(mf.schema_version===1&&Array.isArray(mf.items),'媒体清单无效');const seen=new Set();
  const media=mf.items.map(m=>{assert(['header','diagram','cover','square_cover'].includes(m.role)&&m.alt?.trim()&&m.provenance?.trim(),'媒体角色/替代文字/来源缺失');assert(typeof m.destination==='string'&&!m.destination.includes('..')&&!path.isAbsolute(m.destination),'媒体目标无效');
   assert(m.destination.startsWith(`public/${m.role==='diagram'?'diagrams':'images'}/interview/${id.toLowerCase()}/`)&&/\.(?:png|jpe?g|svg)$/.test(m.destination),'媒体目标必须是本题正式图片');assert(!seen.has(m.destination),'重复媒体目标');seen.add(m.destination);
   const file=m.source==='repository'?publicFile(root,m.destination):ownFile(id,m.source);assert(fs.existsSync(file)&&hash(file)===m.sha256,'媒体文件缺失或摘要过期');staticImageData(file);const magic=fs.readFileSync(file).subarray(0,8);if(m.destination.endsWith('.png'))assert(magic.equals(Buffer.from([137,80,78,71,13,10,26,10])),'PNG目标与实际格式不符');if(/\.jpe?g$/.test(m.destination))assert(magic[0]===255&&magic[1]===216&&magic[2]===255,'JPEG目标与实际格式不符');if(m.destination.endsWith('.svg'))assert(fs.readFileSync(file,'utf8').includes('<svg'),'SVG目标与实际格式不符');safeText(encode({alt:m.alt,provenance:m.provenance}));
   if(m.role==='diagram'&&m.source!=='repository'){assert(m.diagram_source&&typeof m.diagram_source==='object','新技术图需提供冻结来源');const d=m.diagram_source;assert(d.schema_version===1&&d.kind==='frozen-static'&&d.title?.trim()&&d.provenance?.trim()&&d.original_sha256===m.sha256&&d.rebuildable===false,'技术图来源不完整');safeText(encode(d));}
   return {...m,file};});
  const article=publicFile(root,q.article),projection=contentProjection(source,{root,article,media});
  for(const img of projection.images){const m=media.find(x=>x.destination===img.destination);assert(m.alt===img.alt,'正文替代文字与媒体清单不一致');}
  assert(media.some(m=>m.role==='header'&&projection.images.some(i=>i.destination===m.destination))&&media.some(m=>m.role==='cover'),'需要正文头图与横封');
  return {q,w,source,media,projection,article_sha256:digest(source),media_sha256:digest(encode(mf))};
 }
 function status(id){const {w}=load(id);let current;try{const c=candidate(id);current={article_sha256:c.article_sha256,media_sha256:c.media_sha256,validation_stale:!!w.article_sha256&&(w.article_sha256!==c.article_sha256||w.media_sha256!==c.media_sha256)};}catch(e){current={blocked:e.message};}return {id,title:w.title,phase:w.phase,revision:w.revision,...current};}
 function validateReview(c){const r=json(privateFile(base(c.q.id)+'/working/review.json')),research=fs.readFileSync(privateFile(base(c.q.id)+'/working/research.md'),'utf8');assert(research.trim()&&!/TODO|待填写|待核实/.test(research),'研究记录未完成');assert(r.id===c.q.id&&r.reviewer?.trim()&&/^\d{4}-\d{2}-\d{2}$/.test(r.reviewed_at||'')&&validDate(r.reviewed_at)&&new Date(r.reviewed_at).toISOString().slice(0,10)===r.reviewed_at,'审核身份或日期无效');assert(r.article_sha256===c.article_sha256&&r.media_sha256===c.media_sha256,'审核未绑定当前正文及媒体');assert(r.notes?.trim()&&r.evidence_notes?.trim()&&!/TODO|待填写|待核实/.test(r.notes+r.evidence_notes)&&ranks[r.evidence_level]>=ranks[c.q.minimum_evidence]&&checks.every(k=>r.checks?.[k]===true),'审核或最低证据不足');return r;}
 function phase(w,name,extra={}){return {...w,...extra,phase:name,history:[...w.history,{phase:name,at:new Date().toISOString()}]};}
 function review(id){return lock(()=>{const c=candidate(id);assert(c.w.phase==='writing','review仅用于writing阶段，修订请先revise');const r=validateReview(c);const version='r'+c.w.revision+'-'+c.article_sha256.slice(0,16)+'-'+c.media_sha256.slice(0,16);const dir=privateFile(base(id)+'/versions/'+version);fs.mkdirSync(dir,{recursive:true});
  for(const [name,data] of [['article.md',c.source],['research.md',fs.readFileSync(privateFile(base(id)+'/working/research.md'))],['review.json',encode(r)],['projection.json',encode(c.projection)],['media-manifest.json',fs.readFileSync(privateFile(base(id)+'/working/media-manifest.json'))]]){const f=path.join(dir,name);if(fs.existsSync(f))assert(digest(fs.readFileSync(f))===digest(data),'冻结版本内容冲突');else fs.writeFileSync(f,data,{flag:'wx'});}
  for(const m of c.media){const f=path.join(dir,'media',m.sha256+path.extname(m.destination));fs.mkdirSync(path.dirname(f),{recursive:true});if(fs.existsSync(f))assert(hash(f)===m.sha256,'冻结媒体冲突');else fs.writeFileSync(f,fs.readFileSync(m.file),{flag:'wx'});}
  const w=phase(c.w,'reviewed',{version,article_sha256:c.article_sha256,media_sha256:c.media_sha256,review_sha256:digest(encode(r)),research_sha256:hash(privateFile(base(id)+'/working/research.md'))});atomic(wfFile(id),encode(w));return status(id);
 });}
 function validateFrozen(c){const dir=privateFile(base(c.q.id)+'/versions/'+c.w.version);for(const [name,h] of [['article.md',c.article_sha256],['review.json',c.w.review_sha256],['research.md',c.w.research_sha256],['projection.json',digest(encode(c.projection))]])assert(hash(path.join(dir,name))===h,'冻结版本文件已变化');for(const m of c.media)assert(hash(path.join(dir,'media',m.sha256+path.extname(m.destination)))===m.sha256,'冻结媒体已变化');}
 function archivedCurrent(c){assert(hash(publicFile(root,c.q.article))===c.w.archived_sha256,'归档版本漂移');for(const m of c.media)assert(hash(publicFile(root,m.destination))===m.sha256,'归档媒体漂移');const item=registry().items[c.q.id];assert(item?.body_sha256===c.article_sha256&&item.title===c.w.title,'归档渠道登记漂移');}
 function current(id,stages){const c=candidate(id);assert(stages.includes(c.w.phase),'当前阶段不允许此动作');assert(c.w.article_sha256===c.article_sha256&&c.w.media_sha256===c.media_sha256,'正文/媒体变化使验收失效，先revise');validateReview(c);assert(digest(encode(json(privateFile(base(id)+'/working/review.json'))))===c.w.review_sha256&&hash(privateFile(base(id)+'/working/research.md'))===c.w.research_sha256,'冻结审核或研究记录已变化');validateFrozen(c);return c;}
 function validateSaved(c,e){safeText(encode(e));assert(e.schema_version===1&&e.id===c.q.id&&e.article_sha256===c.article_sha256&&e.media_sha256===c.media_sha256,'保存证据版本不符');assert(e.collector?.trim()&&e.target?.trim()&&validDate(e.saved_at),'保存采集者/目标/时间缺失');const observations=e.observations;assert(Array.isArray(observations)&&observations.length===2,'必须包含保存重开与读者预览两个观察');
  assert(new Set(observations.map(o=>o.kind)).size===2&&observations.some(o=>o.kind==='reopened')&&observations.some(o=>o.kind==='reader_preview'),'观察类型缺失');
  for(const o of observations){assert(validDate(o.at)&&Date.parse(o.at)>=Date.parse(e.saved_at)&&o.evidence?.trim(),'观察时间/来源描述缺失');assert(isDeepStrictEqual(o.content,c.projection),'回读正文、代码、表、图或原生结构与候选不一致');assert(['typography','images','tables','code','cover'].every(k=>o.visual_checks?.[k]===true),'视觉验收未全部完成');}
  assert(observations[0].evidence!==observations[1].evidence,'两个观察不能使用同一证据描述');
  // Input remains an operator-supplied record; consistency is not platform authentication.
 }
 function saved(id,relative){return lock(()=>{const c=current(id,['reviewed']);const e=json(ownFile(id,relative));validateSaved(c,e);const file=privateFile(base(id)+'/versions/'+c.w.version+'/saved.json');assert(!fs.existsSync(file),'此版本保存证据已冻结');fs.writeFileSync(file,encode(e),{flag:'wx'});atomic(wfFile(id),encode(phase(c.w,'wechat_saved_verified',{saved_sha256:hash(file)})));return {id,phase:'wechat_saved_verified',note:'已核对操作方输入的回读记录；工具未联网、不认证平台，必须基于实际保存观察。'};});}
 function savedCurrent(c){const f=privateFile(base(c.q.id)+'/versions/'+c.w.version+'/saved.json');assert(hash(f)===c.w.saved_sha256,'保存证据已变化');validateSaved(c,json(f));}
 function resolveTarget(t){assert(t.scope==='repository'||t.scope==='private','事务范围无效');return t.scope==='repository'?publicFile(root,t.path):privateFile(t.path);}
 function transactionFile(id){return privateFile(base(id)+'/archive-transaction.json');}
 function resumeUnlocked(id){const journal=json(transactionFile(id));assert(journal.schema_version===1&&journal.id===id&&journal.status==='prepared','没有待完成归档事务');
  const c=candidate(id);const seal=json(privateFile(base(id)+'/versions/'+journal.version+'/archive-manifest.json'));assert(seal.id===id&&seal.version===journal.version&&seal.targets_sha256===digest(encode(journal.targets)),'事务清单与冻结归档计划不一致');assert(c.w.version===journal.version&&c.article_sha256===c.w.article_sha256&&c.media_sha256===c.w.media_sha256,'事务候选已变化，拒绝恢复');validateReview(c);assert(digest(encode(json(privateFile(base(id)+'/working/review.json'))))===c.w.review_sha256&&hash(privateFile(base(id)+'/working/research.md'))===c.w.research_sha256,'冻结审核或研究记录已变化');validateFrozen(c);savedCurrent(c);
  const allowedRepo=new Set([c.q.article,'public/diagrams/interview/sources.json','content/interview/state.json',...c.media.filter(m=>m.source!=='repository').flatMap(m=>m.role==='diagram'?[m.destination,m.destination+'.source.json']:[m.destination])]);
  const allowedPrivate=new Set([`research/${id}.md`,`reviews/${id}.json`,'wechat/registry.json',base(id)+'/adaptation.json',base(id)+'/workflow.json']);
  assert(Array.isArray(journal.targets)&&new Set(journal.targets.map(t=>t.scope+'/'+t.path)).size===journal.targets.length,'事务目标无效');
  assert(journal.targets.every(t=>(t.scope==='repository'?allowedRepo:allowedPrivate).has(t.path)),'事务目标超出本期归档范围');
  const articleTarget=journal.targets.find(t=>t.scope==='repository'&&t.path===c.q.article);assert(articleTarget?.after_sha256===c.article_sha256,'事务正文不属于当前候选');
  for(const m of c.media.filter(m=>m.source!=='repository'))assert(journal.targets.some(t=>t.scope==='repository'&&t.path===m.destination&&t.after_sha256===m.sha256),'事务媒体不属于当前候选');
  for(const t of journal.targets){const f=resolveTarget(t),h=hash(f);assert(h===t.before_sha256||h===t.after_sha256,'归档目标发生并发变化，拒绝覆盖');assert(digest(Buffer.from(t.data,'base64'))===t.after_sha256,'事务载荷摘要不符');}
  for(const t of journal.targets.filter(t=>t.path!==base(id)+'/workflow.json')){const f=resolveTarget(t);if(hash(f)!==t.after_sha256){assert(hash(f)===t.before_sha256,'写入前目标并发变化');atomic(f,Buffer.from(t.data,'base64'));assert(hash(f)===t.after_sha256,'归档写入未确认');}}
  project(root,env).validate();project(root,env).deliveryCheck(id);project(root,env).render();
  const final=journal.targets.find(t=>t.scope==='private'&&t.path===base(id)+'/workflow.json');assert(final,'缺少归档阶段提交');assert(hash(resolveTarget(final))===final.before_sha256||hash(resolveTarget(final))===final.after_sha256,'归档阶段发生并发变化');atomic(resolveTarget(final),Buffer.from(final.data,'base64'));
  journal.status='completed';atomic(transactionFile(id),encode(journal));return {id,phase:'archived',note:'多文件日志式确认完成；非跨文件原子事务，未操作公众号/网站部署。'};
 }
 function archive(id){return lock(()=>{if(fs.existsSync(transactionFile(id))&&json(transactionFile(id)).status==='prepared')throw new Error('存在未完成事务，先inspect再resume-archive');const c=current(id,['wechat_saved_verified']);savedCurrent(c);assert(hash(publicFile(root,c.q.article))===c.w.base_article_sha256,'正式正文发生并发修改，拒绝覆盖');const reg=registry(),old=reg.items[id];assert(digest(encode(old||null))===c.w.base_mapping_sha256,'原渠道映射已变化');const dsFile='public/diagrams/interview/sources.json',ds=json(publicFile(root,dsFile)),targets=[];
  function add(scope,rel,data){const f=resolveTarget({scope,path:rel});targets.push({scope,path:rel,before_sha256:hash(f),after_sha256:digest(data),data:Buffer.from(data).toString('base64')});}
  for(const m of c.media){if(m.source==='repository')continue;const dest=publicFile(root,m.destination);assert(!fs.existsSync(dest)||hash(dest)===m.sha256,'已有正式媒体不同，拒绝覆盖');add('repository',m.destination,fs.readFileSync(m.file));if(m.role==='diagram'){
   const src=m.destination+'.source.json',data=encode(m.diagram_source);assert(!fs.existsSync(publicFile(root,src))||hash(publicFile(root,src))===digest(data),'既有图源不同，拒绝覆盖');add('repository',src,data);
   const row=ds.diagrams.find(d=>d.source===src);if(row)assert(encode(row.outputs)===encode([m.destination]),'图登记冲突');else ds.diagrams.push({format:'frozen-static-v1',source:src,outputs:[m.destination]});ds.sha256[src]=digest(data);ds.sha256[m.destination]=m.sha256;
  }}
  add('repository',c.q.article,c.source);add('repository',dsFile,encode(ds));const state=json(publicFile(root,'content/interview/state.json')),item=state.items[id];assert(item.status!=='published','已发表版本不可直接覆盖；先明确独立修订范围');
  const steps={planned:['researching','drafting','reviewing','ready'],researching:['drafting','reviewing','ready'],drafting:['reviewing','ready'],reviewing:['ready'],ready:['drafting','reviewing','ready']};
  for(const target of steps[item.status]||[]){item.history.push({from:item.status,to:target,at:new Date().toISOString()});item.status=target;}add('repository','content/interview/state.json',encode(state));
  const r=validateReview(c);add('private',`research/${id}.md`,fs.readFileSync(privateFile(base(id)+'/working/research.md')));add('private',`reviews/${id}.json`,encode(r));
  reg.items[id]={...old,article:c.q.article,section:c.w.section,section_number:c.w.section_number,title:c.w.title,body_sha256:c.article_sha256,platform_status:'verified',wechat_verified_at:json(privateFile(base(id)+'/versions/'+c.w.version+'/saved.json')).saved_at,adaptation:id+'/adaptation.json',evidence:[id+'/versions/'+c.w.version+'/saved.json'],workflow:id+'/workflow.json'};delete reg.items[id].local_status;reg.schema_version=2;
  add('private','wechat/registry.json',encode(reg));add('private',base(id)+'/adaptation.json',encode({schema_version:1,id,article:c.q.article,article_sha256:c.article_sha256,title:c.w.title,omit_headings:[],note:'同一归档正文，仅平台排版不同。'}));
  add('private',base(id)+'/workflow.json',encode(phase(c.w,'archived',{archived_sha256:c.article_sha256})));
  assert(new Set(targets.map(t=>t.scope+'/'+t.path)).size===targets.length,'事务目标重复');
  const backup=privateFile(base(id)+'/versions/'+c.w.version+'/before-archive');fs.mkdirSync(backup,{recursive:true});for(let i=0;i<targets.length;i++){const b=bytes(resolveTarget(targets[i]));if(b)fs.writeFileSync(path.join(backup,String(i)),b,{flag:'wx'});}
  fs.writeFileSync(privateFile(base(id)+'/versions/'+c.w.version+'/archive-manifest.json'),encode({id,version:c.w.version,targets_sha256:digest(encode(targets))}),{flag:'wx'});
  atomic(transactionFile(id),encode({schema_version:1,id,status:'prepared',version:c.w.version,targets}));return resumeUnlocked(id);
 });}
 function resume(id){return lock(()=>resumeUnlocked(id));}
 function revise(id){return lock(()=>{const {w}=load(id);assert(!(fs.existsSync(transactionFile(id))&&json(transactionFile(id)).status==='prepared'),'未完成归档事务必须先处理');const next={...w,revision:w.revision+1,base_article_sha256:hash(publicFile(root,question(id).article)),base_mapping_sha256:digest(encode(registry().items[id]||null))};for(const k of ['version','article_sha256','media_sha256','saved_sha256','archived_sha256','site_sha256','review_sha256','research_sha256'])delete next[k];atomic(wfFile(id),encode(phase(next,'writing')));return status(id);});}
 function enableSite(id,{publicConfirmed=false}={}){return lock(()=>{assert(publicConfirmed,'GitHub Pages是公开网站，须显式--public确认');const c=current(id,['archived','site_ready']);assert(!fs.existsSync(transactionFile(id))||json(transactionFile(id)).status==='completed','归档事务未确认完成');savedCurrent(c);archivedCurrent(c);project(root,env).deliveryCheck(id);const file=publicFile(root,'content/interview/website.json'),old=json(file);assert([1,2].includes(old.schema_version),'网站schema无效');
  const entries=old.schema_version===2?old.articles:[...old.visible_questions].map(id=>{assert(id==='AE-001','旧schema只兼容原首篇');const q=question(id),source=fs.readFileSync(publicFile(root,q.article),'utf8');const media=[...source.matchAll(/!\[[^\]]*\]\(([^)]+)\)/g)].map(m=>path.relative(root,path.resolve(root,path.dirname(q.article),m[1])).split(path.sep).join('/'));return {id,title:source.split('\n')[0].slice(2),article_sha256:digest(source),media_sha256:Object.fromEntries(media.map(rel=>[rel,hash(publicFile(root,rel))])),origin:'legacy'};});
  const e={id,title:c.w.title,article_sha256:c.article_sha256,media_sha256:Object.fromEntries(c.media.map(m=>[m.destination,m.sha256])),origin:'wechat-archive'};const next=entries.filter(e=>e.id!==id).concat(e).sort((a,b)=>question(a.id).order-question(b.id).order);atomic(file,encode({schema_version:2,articles:next}));atomic(wfFile(id),encode(phase(c.w,'site_ready',{site_sha256:digest(encode(e))})));return {id,phase:'site_ready',note:'仅修改本地可见记录；未提交、推送或部署，公开构建不认证平台保存。'};});}
 function snapshot(id){const c=candidate(id);return {id,title:c.w.title,article_sha256:c.article_sha256,media_sha256:c.media_sha256,expected_content:c.projection,note:'候选预期结构，仅用于与独立采集的保存回读比较；不得复制成实际观察证明。'};}
 function inspect(id){question(id);const file=transactionFile(id);assert(fs.existsSync(file),'尚无归档事务');const j=json(file);return {id,status:j.status,version:j.version,targets:j.targets.map(t=>({scope:t.scope,path:t.path,before_sha256:t.before_sha256,after_sha256:t.after_sha256,current_sha256:hash(resolveTarget(t))})),note:'只读查看事务，不处理未知写入、不删除锁或覆盖并发文件。'};}
 function preview(id){const c=candidate(id);const html=renderArticle(c.source,{root,article:publicFile(root,c.q.article),wechat:true,imageResolver:target=>{const dest=path.relative(root,path.resolve(root,path.dirname(c.q.article),target)).split(path.sep).join('/');const m=c.media.find(m=>m.destination===dest);assert(m,'媒体未登记');return staticImageData(m.file);}});const f=privateFile(`exports/${id.toLowerCase()}-issue-${digest(html).slice(0,12)}.html`);if(!fs.existsSync(f)){fs.mkdirSync(path.dirname(f),{recursive:true});fs.writeFileSync(f,html,{flag:'wx'});}return f;}
 function check(id){const c=candidate(id);assert(!fs.existsSync(transactionFile(id))||json(transactionFile(id)).status==='completed','归档事务未确认完成');assert(['writing','reviewed','wechat_saved_verified','archived','site_ready'].includes(c.w.phase),'本期阶段无效');if(c.w.phase!=='writing'){assert(c.w.article_sha256===c.article_sha256&&c.w.media_sha256===c.media_sha256,'本期验证摘要过期');validateReview(c);assert(digest(encode(json(privateFile(base(id)+'/working/review.json'))))===c.w.review_sha256&&hash(privateFile(base(id)+'/working/research.md'))===c.w.research_sha256,'冻结审核或研究记录已变化');validateFrozen(c);}if(['wechat_saved_verified','archived','site_ready'].includes(c.w.phase))savedCurrent(c);if(['archived','site_ready'].includes(c.w.phase))archivedCurrent(c);if(c.w.phase==='site_ready'){const site=json(publicFile(root,'content/interview/website.json'));assert(site.schema_version===2&&site.articles.some(e=>e.id===id&&digest(encode(e))===c.w.site_sha256),'网站准入记录漂移');}return {id,ok:true,phase:c.w.phase,note:'本地结构与记录一致性，不认证远端状态或技术事实。'};}
 return {next,start,status,candidate,review,saved,archive,resume,revise,enableSite,preview,check,workflows,inspect,snapshot};
}
if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 try{const [command,id,...args]=process.argv.slice(2);if(!['resume-archive','inspect','status'].includes(command))project().validate();const p=issueProject();let out;
  if(command==='next'&&!id)out=p.next();
  else if(command==='start'){assert(args.length===0||(args.length===2&&args[0]==='--skip-reason'),'用法start ID [--skip-reason 作者确认理由]');out=p.start(id,{skipReason:args[1]});}
  else if(command==='record-saved'){assert(args.length===1,'record-saved ID 本题私有相对JSON路径');out=p.saved(id,args[0]);}
  else if(command==='site-enable'){assert(args.length===1&&args[0]==='--public','site-enable ID --public');out=p.enableSite(id,{publicConfirmed:true});}
  else{assert(args.length===0,'多余参数');const map={status:'status',check:'check',review:'review',archive:'archive','resume-archive':'resume',revise:'revise',preview:'preview',inspect:'inspect',snapshot:'snapshot'};assert(map[command],'用法next | start ID | status ID | check ID | preview ID | review ID | record-saved ID JSON | archive ID | resume-archive ID | revise ID | site-enable ID --public');out=p[map[command]](id);}
  console.log(typeof out==='string'?out:encode(out));
 }catch(e){console.error(e.message);process.exitCode=1;}
}
