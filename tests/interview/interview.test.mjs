import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { project as actualProject, dateValid } from '../../scripts/interview.mjs';
import {workspaceRoot, workspaceFile, interviewRoot} from '../../scripts/interview/workspace.mjs';
import {checkLinks} from '../../scripts/check-links.mjs';
function privateRoot(root){return root+'-private';}
function project(root){return actualProject(root,{INTERVIEW_WORKSPACE:privateRoot(root)});}
function relative(root,file){
 if(['questions.json','state.json'].includes(file))return path.join(root,'content/interview',file);
 if(file.startsWith('reviews/')||file.startsWith('research/'))return path.join(privateRoot(root),file);
 if(file.startsWith('articles/')){const id=path.basename(file,'.md').toUpperCase();const data=JSON.parse(fs.readFileSync(path.join(root,'content/interview/questions.json')));return path.join(root,data.questions.find(q=>q.id===id).article);}
 return path.join(root,file);
}
import {spawnSync} from 'node:child_process';
import crypto from 'node:crypto';
const source=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'../..');
function fixture(t) {
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'agent-series-'));
  for(const dir of ['content','docs','public/diagrams/interview']) {fs.mkdirSync(path.dirname(path.join(root,dir)),{recursive:true});fs.cpSync(path.join(source,dir),path.join(root,dir),{recursive:true});}
  const data=JSON.parse(fs.readFileSync(relative(root,'questions.json')));
  for(const lab of new Set(data.questions.flatMap(q=>q.lab_refs))){fs.mkdirSync(path.join(root,lab),{recursive:true});for(const f of ['README.md','meta.yml'])fs.copyFileSync(path.join(source,lab,f),path.join(root,lab,f));}
  const file=relative(root,'state.json');
  const state=JSON.parse(fs.readFileSync(file));
  for(const id of Object.keys(state.items)) state.items[id]={status:'planned',history:[],published_at:null,published_url:null};
  fs.writeFileSync(file,JSON.stringify(state));
  // Keep placeholder tests independent from the real article's editorial progress.
  for (const file of ['articles/ae-001.md', 'research/AE-001.md', 'reviews/AE-001.json']) {
    fs.rmSync(relative(root,file), {force:true});
  }
  project(root).scaffold('AE-001');
  t.after(()=>{fs.rmSync(root,{recursive:true,force:true});fs.rmSync(privateRoot(root),{recursive:true,force:true});}); return root;
}
function mutate(root,name,fn) {const file=relative(root,name);const d=JSON.parse(fs.readFileSync(file));fn(d);fs.writeFileSync(file,JSON.stringify(d));}
function approved(root,p) {
  fs.writeFileSync(relative(root,'articles/ae-001.md'),'# Test article\nVerified fixture content\n');
  fs.writeFileSync(relative(root,'research/AE-001.md'),'Fixture evidence, no external claims.\n');
  mutate(root,'reviews/AE-001.json',r=>{r.reviewer='test fixture';r.reviewed_at='2026-10-27';r.article_sha256=p.hash('AE-001');r.notes='Only fixture evidence; not a real article review.';r.evidence_level='design';r.evidence_notes='Fixture evidence only.';for(const k of Object.keys(r.checks))r.checks[k]=true;});
}
test('real catalog has 96 questions and repeat rendering is deterministic', t=>{
 const root=fixture(t),p=project(root);p.validate();p.render();const first=fs.readFileSync(path.join(root,'content/interview/README.md'),'utf8');p.render();assert.equal(first,fs.readFileSync(path.join(root,'content/interview/README.md'),'utf8'));
});
test('scaffold never overwrites existing article, research or review',t=>{
 const root=fixture(t),p=project(root);p.scaffold('AE-002');
 for(const file of ['articles/ae-002.md','research/AE-002.md','reviews/AE-002.json'])fs.writeFileSync(relative(root,file),'user work');
 assert.match(p.scaffold('AE-002'),/保留/);
 for(const file of ['articles/ae-002.md','research/AE-002.md','reviews/AE-002.json'])assert.equal(fs.readFileSync(relative(root,file),'utf8'),'user work');
 assert.throws(()=>p.scaffold('../../escape'),/未知题号/);
});
test('rejects duplicate IDs, dangling dependencies, cycles, unsafe paths',t=>{
 for(const [file,edit,pattern] of [
  ['questions.json',d=>d.questions[1].id=d.questions[0].id,/ID/],
  ['questions.json',d=>d.questions[0].prerequisites=['AE-999'],/前置题不存在/],
  ['questions.json',d=>d.questions[0].prerequisites=['AE-011'],/有环/],
  ['questions.json',d=>d.questions[0].article='../escape',/正文路径/],
 ]) {const root=fixture(t);mutate(root,file,edit);assert.throws(()=>project(root).validate(),pattern);}
});
test('review gates, stale hash, revision and publication metadata',t=>{
 const root=fixture(t),p=project(root);p.scaffold('AE-001');
 assert.throws(()=>p.transition('AE-001','ready'),/不允许/);
 for(const s of ['researching','drafting','reviewing'])p.transition('AE-001',s);
 assert.throws(()=>p.transition('AE-001','ready'),/待完成/);
 approved(root,p);p.transition('AE-001','ready');
 fs.appendFileSync(relative(root,'articles/ae-001.md'),'New change');
 assert.throws(()=>p.transition('AE-001','published',{date:'2026-10-27',url:'https://example.org/article'}),/正文已变化/);
 p.transition('AE-001','drafting');p.transition('AE-001','reviewing');approved(root,p);p.transition('AE-001','ready');
 assert.throws(()=>p.transition('AE-001','published'),/发布登记/);
 p.transition('AE-001','published',{date:'2026-10-27',url:'https://example.org/article'});p.validate();assert.match(p.next(),/^AE-081/);
 assert.throws(()=>p.transition('AE-001','drafting'),/不允许/);
});
test('rejects incomplete review, invalid history and stale concurrent writers',t=>{
 const root=fixture(t),p=project(root),stale=project(root);
 p.transition('AE-001','researching');assert.throws(()=>stale.transition('AE-002','researching'),/其他进程/);
 p.transition('AE-001','drafting');p.transition('AE-001','reviewing');approved(root,p);
 mutate(root,'reviews/AE-001.json',r=>r.checks.facts=false);assert.throws(()=>p.transition('AE-001','ready'),/审核未完成/);
 mutate(root,'state.json',d=>d.items['AE-002'].status='ready');assert.throws(()=>project(root).validate(),/状态与历史/);
});
test('publication dates reject impossible dates and accept leap days',()=>{
 assert.equal(dateValid('2026-02-30'),false);assert.equal(dateValid('2028-02-29'),true);
 assert.equal(dateValid('2026-12-31'),true);assert.equal(dateValid('not-a-date'),false);
});

test('catalogue title consistency, module allocation, source titles and retired IDs stay consistent',t=>{
 const root=fixture(t),q=JSON.parse(fs.readFileSync(relative(root,'questions.json')));
 assert.deepEqual(q.modules.map(m=>m.count),[7,10,10,10,8,12,9,8,12,10]);
 assert.equal(q.questions[0].id,'AE-001');
 assert.equal(q.questions.at(-1).proposal_id,'P-099');
 assert.ok(q.questions.some(q=>Number(q.id.slice(3))>=100));
 assert.equal(new Set([...q.questions.map(q=>q.id),...q.retired_questions.map(r=>r.question.id)]).size,q.questions.length+q.retired_questions.length);
 for(const [edit,pattern] of [
  [d=>d.questions[2].proposal_title='wrong source title',/题序或标题/],
  [d=>d.modules[4].count=10,/模块题数/],
  [d=>d.retired_questions[0].question.id='AE-001',/封存ID/]
 ]){const other=fixture(t);mutate(other,'questions.json',edit);assert.throws(()=>project(other).validate(),pattern);}
});
test('derived order is deterministic and follows prerequisites without a calendar',t=>{
 const root=fixture(t),p=project(root),ordered=p.orderedQuestions();
 assert.equal(ordered.length,96);assert.equal(new Set(ordered.map(q=>q.id)).size,96);
 const positions=new Map(ordered.map((q,i)=>[q.id,i]));
 for(const q of ordered)for(const parent of q.prerequisites)assert.ok(positions.get(parent)<positions.get(q.id));
 assert.deepEqual(ordered.map(q=>q.id),project(root).orderedQuestions().map(q=>q.id));
 assert.ok(!fs.existsSync(path.join(root,'content/interview/calendar.json')));
});
test('next-work skips completed content without treating a saved draft as published',t=>{
 const root=fixture(t),p=project(root);for(const s of ['researching','drafting','reviewing'])p.transition('AE-001',s);
 approved(root,p);p.transition('AE-001','ready');
 assert.match(p.next(),/^AE-001 \[ready\]/);assert.match(p.nextWork(),/^AE-081 \[planned\]/);
 assert.equal(JSON.parse(fs.readFileSync(relative(root,'state.json'))).items['AE-001'].published_url,null);
});
test('engineering readiness requires the planned evidence level',t=>{
 const root=fixture(t),p=project(root);p.scaffold('AE-012');
 fs.writeFileSync(relative(root,'articles/ae-012.md'),'# Fixture\nNo real engineering claims.');
 fs.writeFileSync(relative(root,'research/AE-012.md'),'Fixture checks only.');
 for(const s of ['researching','drafting','reviewing'])p.transition('AE-012',s);
 mutate(root,'reviews/AE-012.json',r=>{r.reviewer='fixture';r.reviewed_at='2026-11-01';r.article_sha256=p.hash('AE-012');r.notes='Fixture review only';r.evidence_level='design';r.evidence_notes='Fixture only';for(const k of Object.keys(r.checks))r.checks[k]=true;});
 assert.throws(()=>p.transition('AE-012','ready'),/工程证据等级/);
 mutate(root,'reviews/AE-012.json',r=>{r.evidence_level='runnable';r.evidence_notes='Synthetic fixture record, not real article evidence.';});
 p.transition('AE-012','ready');assert.match(p.nextWork(),/^AE-001/);
});

test('private output paths reject checkout locations and symlink escapes',t=>{
 const temp=fs.mkdtempSync(path.join(os.tmpdir(),'interview-workspace-'));t.after(()=>fs.rmSync(temp,{recursive:true,force:true}));
 assert.throws(()=>workspaceRoot({INTERVIEW_WORKSPACE:interviewRoot}),/仓库外/);
 assert.throws(()=>workspaceRoot({INTERVIEW_WORKSPACE:path.dirname(interviewRoot)}),/仓库外/);
 fs.symlinkSync(interviewRoot,path.join(temp,'checkout'),'dir');
 assert.throws(()=>workspaceRoot({INTERVIEW_WORKSPACE:path.join(temp,'checkout','generated')}),/仓库外/);
 const work=path.join(temp,'work');fs.mkdirSync(work);fs.symlinkSync(temp,path.join(work,'escape'),'dir');
 assert.throws(()=>workspaceFile('escape/out.html',{INTERVIEW_WORKSPACE:work}),/符号链接/);
 assert.throws(()=>workspaceFile('../outside.html',{INTERVIEW_WORKSPACE:work}),/私有工作目录/);
 assert.equal(workspaceFile('exports/preview.html',{INTERVIEW_WORKSPACE:work}),path.join(work,'exports/preview.html'));
});
test('article export runs from another cwd and preserves source bytes outside checkout',t=>{
 const temp=fs.mkdtempSync(path.join(os.tmpdir(),'interview-export-'));t.after(()=>fs.rmSync(temp,{recursive:true,force:true}));
 const article=path.join(source,JSON.parse(fs.readFileSync(path.join(source,'content/interview/questions.json'))).questions[0].article);const hash=()=>crypto.createHash('sha256').update(fs.readFileSync(article)).digest('hex');const before=hash();
 for(const script of ['export.mjs']){
  const result=spawnSync(process.execPath,[path.join(source,'scripts/interview',script),'AE-001'],{cwd:temp,env:{...process.env,INTERVIEW_WORKSPACE:temp},encoding:'utf8'});
  assert.equal(result.status,0,result.stderr);
 }
 const html=fs.readFileSync(path.join(temp,'exports/ae-001.html'),'utf8');
 assert.equal((html.match(/data:image\/png;base64,/g)||[]).length,2);
 assert.ok(html.includes('ReAct'));assert.equal(hash(),before);assert.ok(!fs.existsSync(path.join(source,'content/interview/exports')));
});
test('public interview files cannot depend on private workspace or outside repository links',t=>{
 const temp=fs.mkdtempSync(path.join(os.tmpdir(),'interview-links-'));t.after(()=>fs.rmSync(temp,{recursive:true,force:true}));
 const repo=path.join(temp,'repo'),root=path.join(repo,'content/interview');fs.mkdirSync(root,{recursive:true});fs.mkdirSync(path.join(repo,'content/book'),{recursive:true});fs.writeFileSync(path.join(repo,'content/book/chapter.md'),'public');
 fs.writeFileSync(path.join(root,'README.md'),'[course](../book/chapter.md)');assert.deepEqual(checkLinks(root,repo),[]);
 fs.mkdirSync(path.join(repo,'.specs'));fs.writeFileSync(path.join(repo,'.specs/private.md'),'private');fs.writeFileSync(path.join(root,'README.md'),'[private](../../.specs/private.md)');assert.equal(checkLinks(root,repo).length,1);
 fs.writeFileSync(path.join(temp,'outside.md'),'outside');fs.symlinkSync(path.join(temp,'outside.md'),path.join(root,'escaped.md'));
 fs.writeFileSync(path.join(root,'README.md'),'[outside](escaped.md)');assert.equal(checkLinks(root,repo).length,1);
});
test('private production folders fail the public catalogue gate even when gitignored',t=>{
 for(const dir of ['plans','exports','.archify']){const root=fixture(t);fs.mkdirSync(path.join(root,'content/interview',dir));assert.throws(()=>project(root).validate(),/仓库外/);}
});

test('public checkout validates ready text without private evidence, delivery still fails closed',t=>{
 const root=fixture(t),p=project(root);
 for(const s of ['researching','drafting','reviewing'])p.transition('AE-001',s);
 approved(root,p);p.transition('AE-001','ready');
 fs.rmSync(privateRoot(root),{recursive:true,force:true});
 assert.doesNotThrow(()=>p.validate());
 assert.throws(()=>p.deliveryCheck('AE-001'),/缺少内容/);
});
test('stable course IDs, Lab ownership, coverage and generated navigation are enforced',t=>{
 for(const edit of [d=>d.questions[0].course_refs=['missing-page'],d=>d.questions[0].lab_refs=['labs/internal'],d=>d.questions[0].coverage={status:'complete',notes:''}]){
  const root=fixture(t);mutate(root,'questions.json',edit);assert.throws(()=>project(root).validate(),/关联|覆盖/);
 }
 const root=fixture(t),p=project(root);p.render();assert.doesNotThrow(()=>p.checkNavigation());
 fs.appendFileSync(path.join(root,'content/interview/README.md'),'outdated');assert.throws(()=>p.checkNavigation(),/总导航过期/);
 const chapter=path.dirname(relative(root,'articles/ae-001.md'));fs.writeFileSync(path.join(chapter,'README.md'),'extra');assert.throws(()=>p.validate(),/总导航与已登记正文/);
});
test('public article paths reject symlink redirection and private process files',t=>{
 const root=fixture(t),file=relative(root,'articles/ae-001.md'),outside=path.join(privateRoot(root),'outside.md');
 fs.writeFileSync(outside,'private');fs.rmSync(file);fs.symlinkSync(outside,file);
 assert.throws(()=>project(root).validate(),/符号链接/);
 fs.rmSync(file);fs.writeFileSync(file,'# Public');fs.mkdirSync(path.join(root,'content/interview/reviews'));
 assert.throws(()=>project(root).validate(),/仓库外/);
});
test('registered diagram bytes and source set cannot silently drift',t=>{
 const root=fixture(t),file=path.join(root,'public/diagrams/interview/ae-001/react.svg');
 fs.appendFileSync(file,'<!-- changed -->');assert.throws(()=>project(root).validate(),/面试图字节/);
 fs.copyFileSync(path.join(source,'public/diagrams/interview/ae-001/react.svg'),file);
 fs.writeFileSync(path.join(path.dirname(file),'unused.svg'),'<svg/>');assert.throws(()=>project(root).validate(),/未登记面试图/);
});

test('all module directories exist, only empty placeholders are allowed, and old calendar is rejected',t=>{
 const root=fixture(t),p=project(root);const data=JSON.parse(fs.readFileSync(relative(root,'questions.json')));
 for(const m of data.modules)assert.ok(fs.statSync(path.join(root,'content/interview',m.directory)).isDirectory());
 const empty=path.join(root,'content/interview/system-design');
 fs.writeFileSync(path.join(empty,'.gitkeep'),'not empty');assert.throws(()=>p.validate(),/总导航与已登记正文/);
 fs.writeFileSync(path.join(empty,'.gitkeep'),'');assert.doesNotThrow(()=>p.validate());
 fs.rmSync(empty,{recursive:true});assert.throws(()=>p.validate(),/缺少大章节/);
 fs.mkdirSync(empty);fs.writeFileSync(path.join(root,'content/interview/calendar.json'),'{}');assert.throws(()=>p.validate(),/发布日历已移除/);
});

test('Planning merge preserves old IDs and advanced coverage without retired dependencies or lost evidence',t=>{
 const root=fixture(t),data=JSON.parse(fs.readFileSync(relative(root,'questions.json'))),state=JSON.parse(fs.readFileSync(relative(root,'state.json')));
 const merged=data.questions.find(q=>q.id==='AE-081');
 assert.equal(data.questions.length,96);assert.equal(merged.title,'Agent 为什么需要 Planning？如何设计一个可靠的 Planner？');assert.equal(merged.minimum_evidence,'measured');
 assert.ok(merged.supplemental_topics.some(t=>t.includes('树搜索')));assert.ok(merged.supplemental_topics.some(t=>t.includes('矛盾')));
 for(const id of ['AE-021','AE-091','AE-041']){
  assert.ok(!data.questions.some(q=>q.id===id));assert.ok(!(id in state.items));assert.ok(data.questions.every(q=>!q.prerequisites.includes(id)));
  const archived=data.retired_questions.find(r=>r.question.id===id);assert.equal(archived.covered_by,'AE-081');assert.equal(archived.last_state.status,'planned');assert.ok(archived.question.acceptance_case);
 }
 assert.deepEqual(data.questions.map(q=>q.order),Array.from({length:96},(_,i)=>i+1));
 assert.deepEqual(data.questions.map(q=>q.proposal_id),Array.from({length:99},(_,i)=>i+1).filter(n=>![5,6,7].includes(n)).map(n=>`P-${String(n).padStart(3,'0')}`));
 assert.equal(data.retired_questions.length,28);assert.ok(data.retired_questions.every(r=>data.questions.some(q=>q.id===r.covered_by)));
});
