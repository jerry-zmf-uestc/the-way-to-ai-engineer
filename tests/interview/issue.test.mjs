import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import {issueProject,digest} from '../../scripts/interview/issue.mjs';
import {repositoryRoot as ROOT} from '../../scripts/interview/workspace.mjs';
import {loadInterview,renderInterview} from '../../scripts/interview-site.mjs';
const write=(p,j)=>{fs.mkdirSync(path.dirname(p),{recursive:true});fs.writeFileSync(p,typeof j==='string'?j:JSON.stringify(j,null,2)+'\n');};
function fixture(t){const top=fs.mkdtempSync(path.join(os.tmpdir(),'issue-contract-')),root=path.join(top,'repo'),privateRoot=path.join(top,'private');fs.mkdirSync(root);fs.mkdirSync(privateRoot);t.after(()=>fs.rmSync(top,{recursive:true,force:true}));
 for(const rel of ['content','docs','public/diagrams/interview','public/images/interview']){fs.mkdirSync(path.dirname(path.join(root,rel)),{recursive:true});fs.cpSync(path.join(ROOT,rel),path.join(root,rel),{recursive:true});}
 const data=JSON.parse(fs.readFileSync(path.join(root,'content/interview/questions.json')));
 // Reset only the copied fixture issue, so real publication cannot change this new-issue test baseline.
 const sample=data.questions.find(q=>q.id==='AE-100');
 fs.rmSync(path.join(root,sample.article),{force:true});
 for(const base of ['public/diagrams/interview','public/images/interview'])fs.rmSync(path.join(root,base,'ae-100'),{recursive:true,force:true});
 const stateFile=path.join(root,'content/interview/state.json'),state=JSON.parse(fs.readFileSync(stateFile));
 state.items['AE-100']={status:'planned',history:[],published_at:null,published_url:null};write(stateFile,state);
 const sourceFile=path.join(root,'public/diagrams/interview/sources.json'),sources=JSON.parse(fs.readFileSync(sourceFile));
 sources.diagrams=sources.diagrams.filter(d=>!d.source.startsWith('public/diagrams/interview/ae-100/'));
 for(const key of Object.keys(sources.sha256))if(key.startsWith('public/diagrams/interview/ae-100/'))delete sources.sha256[key];write(sourceFile,sources);
 for(const lab of new Set(data.questions.flatMap(q=>q.lab_refs))){for(const name of ['README.md','meta.yml']){fs.mkdirSync(path.join(root,lab),{recursive:true});fs.copyFileSync(path.join(ROOT,lab,name),path.join(root,lab,name));}}
 write(path.join(privateRoot,'wechat/registry.json'),{schema_version:2,items:{'AE-001':{section:'Reasoning&Planning',section_number:'01',title:'历史首篇'},'AE-081':{section:'Reasoning&Planning',section_number:'02',title:'历史Planning'}}});
 return {root,privateRoot,data,p:issueProject(root,{INTERVIEW_WORKSPACE:privateRoot})};}
function complete(f,id='AE-100'){const {p,privateRoot}=f;p.start(id);const title=p.status(id).title,dir=path.join(privateRoot,'wechat',id,'working');const image=path.join(dir,'header.jpg');fs.writeFileSync(image,Buffer.from([255,216,255,217]));
 const m={role:'header',source:'working/header.jpg',destination:`public/images/interview/${id.toLowerCase()}/header.jpg`,alt:'主题头图',provenance:'合成测试图；非实际公众号素材',sha256:digest(fs.readFileSync(image))};
 write(path.join(dir,'media-manifest.json'),{schema_version:1,items:[m,{...m,role:'cover',destination:`public/images/interview/${id.toLowerCase()}/cover.jpg`,alt:'横封'}]});
 write(path.join(dir,'article.md'),`# ${title}\n\n![主题头图](../../../${m.destination})\n\n# 引入问题\n\n这是合成测试候选，不是实际稿件。\n\n## 小结\n\n| 知识点 | 结论 |\n| --- | --- |\n| 状态 | 区分保存与完成 |\n\n`+'```python\nx = 1\n\nassert x == 1\n```\n');
 write(path.join(dir,'research.md'),'合成测试研究：只验证固定控制契约，不证明真实平台、事实或模型。');const c=p.candidate(id);write(path.join(dir,'review.json'),{id,reviewer:'合成测试fixture，非真人',reviewed_at:'2026-11-05',article_sha256:c.article_sha256,media_sha256:c.media_sha256,notes:'固定测试声明，不作为文章审核',evidence_level:'design',evidence_notes:'仅验证输入绑定与门禁',checks:{facts:true,engineering:true,clarity:true,sources:true,safety:true}});return {dir,c};}
function evidence(f,id){const c=f.p.candidate(id);return {schema_version:1,id,collector:'合成fixture，不是实际操作人',target:'合成草稿对象',saved_at:'2026-11-05T00:00:00Z',article_sha256:c.article_sha256,media_sha256:c.media_sha256,observations:[{kind:'reopened',at:'2026-11-05T00:01:00Z',evidence:'合成保存重开读数',content:c.projection,visual_checks:{typography:true,images:true,tables:true,code:true,cover:true}},{kind:'reader_preview',at:'2026-11-05T00:02:00Z',evidence:'合成独立读者页读数',content:c.projection,visual_checks:{typography:true,images:true,tables:true,code:true,cover:true}}]};}
test('next issue uses catalogue order, separates legacy work and preserves channel sequence',t=>{const f=fixture(t);assert.equal(f.p.next().next.id,'AE-100');const article=f.data.questions.find(q=>q.id==='AE-100').article;const out=f.p.start('AE-100');assert.ok(out.title.includes('Reasoning&Planning 03：'+f.data.questions.find(q=>q.id==='AE-100').title));assert.ok(!fs.existsSync(path.join(f.root,article)));assert.throws(()=>f.p.start('AE-100'),/已存在/);assert.throws(()=>f.p.start('AE-002'),/跳题/);assert.equal(f.p.next().next.id,'AE-031');});
test('private candidate preview and review reject stale media, wrong title and insufficient evidence',t=>{const f=fixture(t);const {dir}=complete(f);assert.ok(fs.existsSync(f.p.preview('AE-100')));const rev=JSON.parse(fs.readFileSync(path.join(dir,'review.json')));rev.checks.facts=false;write(path.join(dir,'review.json'),rev);assert.throws(()=>f.p.review('AE-100'),/不足/);rev.checks.facts=true;write(path.join(dir,'review.json'),rev);f.p.review('AE-100');fs.appendFileSync(path.join(dir,'article.md'),'\n追加未审核正文\n');assert.throws(()=>f.p.saved('AE-100','working/saved.json'),/失效/);assert.equal(f.p.status('AE-100').validation_stale,true);f.p.revise('AE-100');assert.equal(f.p.status('AE-100').phase,'writing');});
test('saved evidence requires two structured observations; self-declared saved alone cannot pass',t=>{const f=fixture(t);complete(f);f.p.review('AE-100');const file=path.join(f.privateRoot,'wechat/AE-100/working/saved.json');write(file,{saved:true});assert.throws(()=>f.p.saved('AE-100','working/saved.json'),/版本/);const e=evidence(f,'AE-100');e.observations[0].content={title:'错误版本'};write(file,e);assert.throws(()=>f.p.saved('AE-100','working/saved.json'),/不一致/);const good=evidence(f,'AE-100');write(file,good);assert.equal(f.p.saved('AE-100','working/saved.json').phase,'wechat_saved_verified');assert.ok(!fs.existsSync(path.join(f.root,f.data.questions.find(q=>q.id==='AE-100').article)));});
test('full local fixture archives verified version, binds website and rejects stale public version',t=>{const f=fixture(t);complete(f);f.p.review('AE-100');write(path.join(f.privateRoot,'wechat/AE-100/working/saved.json'),evidence(f,'AE-100'));f.p.saved('AE-100','working/saved.json');assert.equal(f.p.archive('AE-100').phase,'archived');const website=path.join(f.root,'content/interview/website.json');assert.equal(JSON.parse(fs.readFileSync(website)).schema_version,1);assert.throws(()=>f.p.enableSite('AE-100'),/公开/);f.p.enableSite('AE-100',{publicConfirmed:true});const data=loadInterview(f.root);assert.deepEqual(data.visible.map(q=>q.id),['AE-001','AE-100']);const q=data.visible[1];assert.ok(renderInterview(q,data,f.root).includes('/images/interview/ae-100/header.jpg'));fs.appendFileSync(path.join(f.root,q.article),'\n未绑定的修改\n');assert.throws(()=>loadInterview(f.root),/绑定/);assert.throws(()=>f.p.check('AE-100'),/漂移/);});
test('archive CAS rejects concurrent article changes and evidence path escape without altering target',t=>{const f=fixture(t);complete(f);f.p.review('AE-100');assert.throws(()=>f.p.saved('AE-100','../../outside.json'),/工作文件/);write(path.join(f.privateRoot,'wechat/AE-100/working/saved.json'),evidence(f,'AE-100'));f.p.saved('AE-100','working/saved.json');const file=path.join(f.root,f.data.questions.find(q=>q.id==='AE-100').article);write(file,'# 用户并发作品\n不能覆盖\n');assert.throws(()=>f.p.archive('AE-100'),/并发/);assert.ok(fs.readFileSync(file,'utf8').includes('不能覆盖'));});
test('topic changes invalidate locked issue, old website schema cannot expose new articles',t=>{const f=fixture(t);f.p.start('AE-100');f.data.questions.find(q=>q.id==='AE-100').title+='新版';write(path.join(f.root,'content/interview/questions.json'),f.data);assert.throws(()=>f.p.status('AE-100'),/主题/);write(path.join(f.root,'content/interview/website.json'),{schema_version:1,visible_questions:['AE-104']});assert.throws(()=>loadInterview(f.root),/旧网站/);});

test('frozen review and research changes are rejected; identical content can be reviewed in a new revision',t=>{
 const f=fixture(t),{dir}=complete(f);f.p.review('AE-100');const old=JSON.parse(fs.readFileSync(path.join(f.privateRoot,'wechat/AE-100/workflow.json'))).version;
 fs.appendFileSync(path.join(dir,'research.md'),'\n新增未审核资料');assert.throws(()=>f.p.check('AE-100'),/冻结/);
 f.p.revise('AE-100');f.p.review('AE-100');const current=JSON.parse(fs.readFileSync(path.join(f.privateRoot,'wechat/AE-100/workflow.json'))).version;assert.notEqual(old,current);assert.ok(fs.existsSync(path.join(f.privateRoot,'wechat/AE-100/versions',old,'research.md')));
 const r=JSON.parse(fs.readFileSync(path.join(dir,'review.json')));r.notes+='变更';write(path.join(dir,'review.json'),r);assert.throws(()=>f.p.saved('AE-100','working/saved.json'),/冻结/);
});

test('private root, cross-question symlink and media destination or missing header cannot bypass isolation',t=>{
 const f=fixture(t);assert.throws(()=>issueProject(f.root,{INTERVIEW_WORKSPACE:path.join(f.root,'private')}),/私有/);const {dir}=complete(f);
 const original=fs.readFileSync(path.join(dir,'media-manifest.json'),'utf8'),m=JSON.parse(original);m.items[0].destination='public/images/interview/ae-104/header.jpg';write(path.join(dir,'media-manifest.json'),m);assert.throws(()=>f.p.candidate('AE-100'),/本题/);
 write(path.join(dir,'media-manifest.json'),original);const out=path.join(f.privateRoot,'wechat/AE-104');fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'image.jpg'),Buffer.from([255,216,255,217]));fs.symlinkSync(out,path.join(dir,'other'));m.items[0]={...JSON.parse(original).items[0],source:'working/other/image.jpg'};write(path.join(dir,'media-manifest.json'),m);assert.throws(()=>f.p.candidate('AE-100'),/符号链接/);
 write(path.join(dir,'media-manifest.json'),original);const text=fs.readFileSync(path.join(dir,'article.md'),'utf8');write(path.join(dir,'article.md'),text.replace(/!\[[^\]]*\]\([^\n]+\)/,''));assert.throws(()=>f.p.candidate('AE-100'),/头图/);
});

function savedFixture(f){complete(f);f.p.review('AE-100');write(path.join(f.privateRoot,'wechat/AE-100/working/saved.json'),evidence(f,'AE-100'));f.p.saved('AE-100','working/saved.json');}
function interruptArchive(f){const rename=fs.renameSync;fs.renameSync=function(from,to){if(path.resolve(to)===path.join(fs.realpathSync(f.root),'content/interview/state.json'))throw new Error('fixture simulated interruption');return rename(from,to);};try{assert.throws(()=>f.p.archive('AE-100'),/simulated interruption/);}finally{fs.renameSync=rename;}}

test('interrupted multi-file archive stays unconfirmed and resumes without rewriting confirmed files',t=>{
 const f=fixture(t);savedFixture(f);interruptArchive(f);assert.equal(f.p.status('AE-100').phase,'wechat_saved_verified');assert.equal(f.p.inspect('AE-100').status,'prepared');assert.throws(()=>f.p.enableSite('AE-100',{publicConfirmed:true}),/阶段/);
 assert.throws(()=>f.p.revise('AE-100'),/未完成/);const article=path.join(f.root,f.data.questions.find(q=>q.id==='AE-100').article),mtime=fs.statSync(article).mtimeMs;
 assert.equal(f.p.resume('AE-100').phase,'archived');assert.equal(fs.statSync(article).mtimeMs,mtime);assert.equal(f.p.inspect('AE-100').status,'completed');assert.ok(f.p.check('AE-100').ok);
});

test('archive recovery rejects changed shared registry and edited journal before any further write',t=>{
 const f=fixture(t);savedFixture(f);interruptArchive(f);const registry=path.join(f.privateRoot,'wechat/registry.json'),prior=fs.readFileSync(registry);write(registry,'{"schema_version":2,"items":{},"author_change":true}\n');
 const state=fs.readFileSync(path.join(f.root,'content/interview/state.json'));assert.throws(()=>f.p.resume('AE-100'),/并发/);assert.deepEqual(fs.readFileSync(path.join(f.root,'content/interview/state.json')),state);fs.writeFileSync(registry,prior);
 const file=path.join(f.privateRoot,'wechat/AE-100/archive-transaction.json'),journal=JSON.parse(fs.readFileSync(file));journal.targets.pop();write(file,journal);assert.throws(()=>f.p.resume('AE-100'),/冻结归档计划/);assert.deepEqual(fs.readFileSync(path.join(f.root,'content/interview/state.json')),state);
});

test('new frozen technical media archive with source registration and existing media cannot be overwritten',t=>{
 const f=fixture(t),{dir}=complete(f),file=path.join(dir,'media-manifest.json'),mf=JSON.parse(fs.readFileSync(file)),header=mf.items[0];mf.items.push({...header,role:'diagram',destination:'public/diagrams/interview/ae-100/example.jpg',diagram_source:{schema_version:1,kind:'frozen-static',title:'合成示意',provenance:'测试固定字节，不是真实图验收',original_sha256:header.sha256,rebuildable:false}});write(file,mf);
 const c=f.p.candidate('AE-100'),r=JSON.parse(fs.readFileSync(path.join(dir,'review.json')));r.media_sha256=c.media_sha256;write(path.join(dir,'review.json'),r);f.p.review('AE-100');write(path.join(dir,'saved.json'),evidence(f,'AE-100'));f.p.saved('AE-100','working/saved.json');const dest=path.join(f.root,mf.items[2].destination);fs.mkdirSync(path.dirname(dest),{recursive:true});fs.writeFileSync(dest,Buffer.from([255,216,255,0]));assert.throws(()=>f.p.archive('AE-100'),/正式媒体不同/);fs.writeFileSync(dest,fs.readFileSync(path.join(dir,'header.jpg')));assert.equal(f.p.archive('AE-100').phase,'archived');const ds=JSON.parse(fs.readFileSync(path.join(f.root,'public/diagrams/interview/sources.json')));assert.ok(ds.diagrams.some(d=>d.source===mf.items[2].destination+'.source.json'));
});

test('frozen archive media and website binding changes invalidate subsequent checks',t=>{
 const f=fixture(t);savedFixture(f);f.p.archive('AE-100');f.p.enableSite('AE-100',{publicConfirmed:true});const mf=f.p.candidate('AE-100').media,cover=path.join(f.root,mf.find(m=>m.role==='cover').destination),original=fs.readFileSync(cover);fs.writeFileSync(cover,Buffer.from([255,216,255,0]));assert.throws(()=>f.p.check('AE-100'),/媒体漂移/);fs.writeFileSync(cover,original);
 const website=path.join(f.root,'content/interview/website.json'),site=JSON.parse(fs.readFileSync(website));site.articles=site.articles.filter(e=>e.id!=='AE-100');write(website,site);assert.throws(()=>f.p.check('AE-100'),/准入记录漂移/);
 const w=JSON.parse(fs.readFileSync(path.join(f.privateRoot,'wechat/AE-100/workflow.json')));fs.appendFileSync(path.join(f.privateRoot,'wechat/AE-100/versions',w.version,'article.md'),'changed');assert.throws(()=>f.p.check('AE-100'),/冻结版本/);
});

test('saved observations reject code, table, order, cover and link changes while object key order is irrelevant',t=>{
 const f=fixture(t);complete(f);f.p.review('AE-100');const file=path.join(f.privateRoot,'wechat/AE-100/working/saved.json');
 for(const key of ['codes','tables','sequence','covers','links']){const e=evidence(f,'AE-100');e.observations[0].content=JSON.parse(JSON.stringify(e.observations[0].content));e.observations[0].content[key]=['mismatch'];write(file,e);assert.throws(()=>f.p.saved('AE-100','working/saved.json'),/不一致/);}
 const e=evidence(f,'AE-100');e.observations.forEach(o=>o.content=Object.fromEntries(Object.entries(o.content).reverse()));write(file,e);assert.equal(f.p.saved('AE-100','working/saved.json').phase,'wechat_saved_verified');
});

test('channel mapping concurrent changes and late prerequisites do not change locked numbering or order',t=>{
 const f=fixture(t);const q=f.data.questions.find(q=>q.id==='AE-100');q.prerequisites=['AE-051'];write(path.join(f.root,'content/interview/questions.json'),f.data);assert.equal(f.p.next().next.id,'AE-100');savedFixture(f);const regFile=path.join(f.privateRoot,'wechat/registry.json'),reg=JSON.parse(fs.readFileSync(regFile));reg.items['AE-100']={title:'作者新映射'};write(regFile,reg);assert.throws(()=>f.p.archive('AE-100'),/映射已变化/);
 const other=fixture(t);const result=other.p.start('AE-081');assert.equal(result.title,'历史Planning');assert.equal(other.p.status('AE-081').title,'历史Planning');
});

