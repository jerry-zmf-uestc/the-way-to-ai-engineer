import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import {ROOT,loadInterview,prepareInterview,renderInterview,INTERVIEW_BASE} from '../scripts/interview-site.mjs';
import {watchInterviewSite} from '../scripts/watch-interview-site.mjs';
import {filterQuestions} from '../src/interview-site/scripts/search.mjs';
import {formatArticle} from '../src/interview-site/scripts/article.mjs';
test('article semantic classes mark captions and references without modifying text',()=>{
 const n=(tagName,textContent)=>({tagName,textContent,classList:{values:[],add(value){this.values.push(value);}}}),nodes=[n('P','普通正文'),n('P','图2：位置示意'),n('H1','参考资料'),n('UL','引用'),n('P','完整来源URL'),n('H2','回到开头'),n('P','结论')];
 formatArticle({children:nodes});assert.deepEqual(nodes[1].classList.values,['caption']);assert.deepEqual(nodes[3].classList.values,['references']);assert.deepEqual(nodes[4].classList.values,['references']);assert.deepEqual(nodes[6].classList.values,[]);assert.equal(nodes[1].textContent,'图2：位置示意');
});
function fixture(t){
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'interview-site-'));t.after(()=>fs.rmSync(root,{recursive:true,force:true}));
 for(const dir of ['content','docs','public/diagrams/interview','public/images/interview']){fs.mkdirSync(path.dirname(path.join(root,dir)),{recursive:true});fs.cpSync(path.join(ROOT,dir),path.join(root,dir),{recursive:true});}
 const data=JSON.parse(fs.readFileSync(path.join(root,'content/interview/questions.json')));
 for(const lab of new Set(data.questions.flatMap(q=>q.lab_refs))){fs.mkdirSync(path.join(root,lab),{recursive:true});for(const name of ['README.md','meta.yml'])fs.copyFileSync(path.join(ROOT,lab,name),path.join(root,lab,name));}
 return root;
}
function website(root,ids){fs.writeFileSync(path.join(root,'content/interview/website.json'),JSON.stringify({schema_version:1,visible_questions:ids}));}
test('website visibility is explicit, independent from editorial publication and rejects missing/incomplete content',t=>{
 const root=fixture(t),data=loadInterview(root);assert.deepEqual(data.visible.map(q=>q.id),['AE-001']);
 const statefile=path.join(root,'content/interview/state.json'),state=JSON.parse(fs.readFileSync(statefile));state.items['AE-001']={status:'planned',history:[],published_at:null,published_url:null};fs.writeFileSync(statefile,JSON.stringify(state));assert.equal(loadInterview(root).visible.length,1);
 website(root,[]);assert.equal(loadInterview(root).visible.length,0);
 for(const ids of [['AE-999'],['AE-001','AE-001'],['AE-002']]){website(root,ids);assert.throws(()=>loadInterview(root),/网站/);}
 const q=data.questions.find(q=>q.id==='AE-002');fs.mkdirSync(path.dirname(path.join(root,q.article)),{recursive:true});fs.writeFileSync(path.join(root,q.article),'# Draft\nTODO');fs.writeFileSync(path.join(root,'content/interview/website.json'),JSON.stringify({schema_version:2,articles:[{id:q.id,title:'Draft',article_sha256:'0'.repeat(64),media_sha256:{},origin:'wechat-archive'}]}));assert.throws(()=>loadInterview(root),/不完整/);
});
test('generation is stable, rewrites formal images and removes withdrawn routes without touching source',t=>{
 const root=fixture(t),data=loadInterview(root),q=data.visible[0],file=path.join(root,q.article),source=fs.readFileSync(file);
 const text=renderInterview(q,data,root);assert.ok(text.includes(INTERVIEW_BASE+'/diagrams/interview/ae-001/react.png'));assert.ok(!text.includes('../../../public/'));
 prepareInterview(root);const out=path.join(root,'.generated/interview/ae-001.md'),mtime=fs.statSync(out).mtimeMs;prepareInterview(root);assert.equal(fs.statSync(out).mtimeMs,mtime);assert.deepEqual(fs.readFileSync(file),source);
 website(root,[]);prepareInterview(root);assert.ok(!fs.existsSync(out));
});
test('real interview watcher keeps last valid content on error, recovers, withdraws and closes',async t=>{
 const root=fixture(t);let errors=0;
 const watcher=watchInterviewSite({rootDir:root,pollMs:20,debounceMs:10,onError:()=>errors++});t.after(()=>watcher.close());
 const out=path.join(root,'.generated/interview/ae-001.md'),previous=fs.readFileSync(out,'utf8');
 async function until(check){const end=Date.now()+4000;while(!check()){if(Date.now()>end)throw new Error('watcher timed out');await new Promise(r=>setTimeout(r,25));}}
 website(root,['AE-999']);await until(()=>errors>0);assert.equal(fs.readFileSync(out,'utf8'),previous);
 website(root,[]);await until(()=>!fs.existsSync(out));website(root,['AE-001']);await until(()=>fs.existsSync(out));
 watcher.close();website(root,[]);await new Promise(r=>setTimeout(r,100));assert.ok(fs.existsSync(out));
});
test('question search is case insensitive, supports stable IDs and updates empty state',()=>{
 const items=[{dataset:{search:'ae-001 react'},hidden:false},{dataset:{search:'ae-002 json'},hidden:false}],status={},empty={};
 const section={querySelectorAll:()=>items};const root={querySelectorAll:s=>s==='[data-question]'?items:[section],querySelector:s=>s==='#search-result'?status:empty};
 assert.equal(filterQuestions(root,' REACT '),1);assert.equal(items[1].hidden,true);assert.equal(empty.hidden,true);
 assert.equal(filterQuestions(root,'AE-002'),1);assert.equal(items[0].hidden,true);
 assert.equal(filterQuestions(root,'missing'),0);assert.equal(section.hidden,true);assert.equal(empty.hidden,false);
 assert.equal(filterQuestions(root,''),2);assert.equal(section.hidden,false);
});
