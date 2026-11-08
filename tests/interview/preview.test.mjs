import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import crypto from 'node:crypto';
import {renderArticle,safeNativeHTML,imageData} from '../../scripts/interview/preview.mjs';
import {checkWechat} from '../../scripts/interview/wechat.mjs';
import {verifyDiagrams} from '../../scripts/interview/diagrams.mjs';
const hash=p=>crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
function fixture(t){
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'wechat-preview-'));
 const privateRoot=root+'-private';
 for(const dir of ['content/interview','public/diagrams/interview','public/images/interview'])fs.mkdirSync(path.join(root,dir),{recursive:true});
 fs.mkdirSync(privateRoot,{recursive:true});
 t.after(()=>{fs.rmSync(root,{recursive:true,force:true});fs.rmSync(privateRoot,{recursive:true,force:true});});
 return {root,privateRoot,article:path.join(root,'content/interview/test.md')};
}
test('wechat preview preserves code blank lines, correct heading text and typography, native cards and tables',t=>{
 const {root,article}=fixture(t);
 const source='# 标题\n\n# 主标题\n\n## 次标题\n\n### 小标题\n\n<mark>先给判断</mark>再解释。\n\n```python\nif True:\n    value = "<unsafe>"\n\n    print(value)\n```\n\n| 概念 | 说明 |\n| --- | --- |\n| State | 任务数据 |\n\n<section class="request-diagram"><p class="request-code">{ role: &quot;user&quot; }</p></section>\n\n图2：位置示意\n\n# 参考资料\n\n[1] 官方资料<br>https://example.org/docs\n';
 const html=renderArticle(source,{root,article,wechat:true});
 assert.ok(html.includes('<h1>主标题</h1>'));assert.ok(html.includes('<h2>次标题</h2>'));assert.ok(html.includes('<h3>小标题</h3>'));
 assert.ok(html.includes('    value = &quot;&lt;unsafe&gt;&quot;\n\n    print(value)'));
 assert.match(html,/font-size:15px;line-height:28\.5px/);assert.match(html,/h1\{font-size:20px/);assert.match(html,/h2\{font-size:18px/);assert.match(html,/h3\{font-size:16px/);
 assert.ok(html.includes('font:14px/26px'));assert.ok(html.includes('<table>'));assert.ok(html.includes('<mark>先给判断</mark>'));assert.ok(html.includes('request-diagram'));
 assert.ok(html.includes('class="caption"'));assert.ok(html.includes('class="references"'));
 assert.ok(html.includes('mark{background:transparent;color:#2d6a5b;font-weight:600}'));
 assert.ok(!html.includes('mark{background:#e5f5ef'));
 const reference=renderArticle('# 标题\n\n[[1]](https://example.org/docs)\n\n| 项目 | 机制 | 应用 | 边界 |\n| --- | --- | --- | --- |\n| MCP | 协议 | 连接 | 权限 |',{root,article,wechat:true});
 assert.ok(reference.includes('<a href="https://example.org/docs" rel="noreferrer">[1]</a>'));
 assert.ok(reference.includes('class="table-scroll columns-4"'));assert.ok(reference.includes('<col style="width:16%">'));
 assert.match(reference,/header\{font-size:20px;line-height:30px/);assert.ok(reference.includes('.columns-4 table{min-width:620px}'));
 assert.ok(!reference.includes("<colgroup>'+cols"));
});
test('unsafe native HTML, URLs, unsupported whiteboards and malformed fences fail closed',t=>{
 const {root,article}=fixture(t),render=s=>renderArticle('# Test\n\n'+s,{root,article,wechat:true});
 for(const s of ['<script>alert(1)</script>','<whiteboard token="x"></whiteboard>','<table><tr></tr></table>','```python\nx = 1','[bad](javascript:alert)','[bad](http://example.org)'])assert.throws(()=>render(s));
 for(const s of ['<section onclick="bad()"></section>','<section class="request-diagram"><img src="x"></section>','<section><p>wrong</section></p>','<section style="background:url(x)"></section>'])assert.throws(()=>safeNativeHTML(s));
 assert.doesNotThrow(()=>safeNativeHTML('<section class="request-diagram"><p class="request-code">&lt;agent_status&gt;</p></section>'));
});
test('local images use actual MIME and reject external, missing and symlink escapes',t=>{
 const {root,article}=fixture(t);
 fs.writeFileSync(path.join(root,'public/images/interview/header.jpg'),Buffer.from([255,216,255,217]));
 const out=renderArticle('# Test\n\n![header](../../public/images/interview/header.jpg)',{root,article});assert.ok(out.includes('data:image/jpeg;base64,'));
 for(const s of ['https://example.org/x.png','../../outside.png','../../public/images/interview/missing.png'])assert.throws(()=>imageData(root,article,s));
 const outside=path.join(path.dirname(root),path.basename(root)+'-outside.png');fs.writeFileSync(outside,'private');t.after(()=>fs.rmSync(outside,{force:true}));
 fs.symlinkSync(outside,path.join(root,'public/images/interview/escape.png'));assert.throws(()=>imageData(root,article,'../../public/images/interview/escape.png'));
});
test('channel adapter omits selected site supplements without rewriting shared body',t=>{
 const {root,article}=fixture(t);const source='# Common\n\n## 机制\n\n共同正文\n\n## 30 秒面试回答\n\n可选补充\n\n## 参考资料\n\n原始出处\n';
 const html=renderArticle(source,{root,article,title:'渠道标题',omitHeadings:['30 秒面试回答'],wechat:true});
 assert.ok(html.includes('渠道标题'));assert.ok(html.includes('共同正文'));assert.ok(!html.includes('可选补充'));assert.ok(html.includes('原始出处'));
});
test('registry checker rejects duplicate section sequence, stale hashes, evidence escapes and false ready',t=>{
 const {root,privateRoot,article}=fixture(t),env={INTERVIEW_WORKSPACE:privateRoot};fs.writeFileSync(article,'# Article\n\n正文');
 const q={id:'AE-001',article:'content/interview/test.md',minimum_evidence:'measured'};
 fs.writeFileSync(path.join(root,'content/interview/questions.json'),JSON.stringify({questions:[q]}));
 const state={items:{'AE-001':{status:'drafting'}}};fs.writeFileSync(path.join(root,'content/interview/state.json'),JSON.stringify(state));
 for(const dir of ['wechat/AE-001','research','reviews'])fs.mkdirSync(path.join(privateRoot,dir),{recursive:true});
 fs.writeFileSync(path.join(privateRoot,'research/AE-001.md'),'已知输入，尚缺测量');
 const review={id:'AE-001',article_sha256:hash(article),evidence_level:'design',checks:{facts:false}};fs.writeFileSync(path.join(privateRoot,'reviews/AE-001.json'),JSON.stringify(review));
 const item={article:q.article,section:'Reasoning&Planning',section_number:'01',title:'Agent 每日一题｜Reasoning&Planning 01：问题？',body_sha256:hash(article),platform_status:'unverified',evidence:[],adaptation:'AE-001/adaptation.json'};
 fs.writeFileSync(path.join(privateRoot,'wechat/AE-001/adaptation.json'),JSON.stringify({id:q.id,article:q.article,article_sha256:hash(article),title:item.title}));
 const registry={schema_version:2,items:{'AE-001':item}},file=path.join(privateRoot,'wechat/registry.json'),write=()=>fs.writeFileSync(file,JSON.stringify(registry));write();
 assert.equal(checkWechat(root,env).ok,true);assert.equal(checkWechat(root,env).pending.length,2);
 item.body_sha256='0'.repeat(64);write();assert.equal(checkWechat(root,env).ok,false);item.body_sha256=hash(article);
 item.evidence=['../../outside'];write();assert.equal(checkWechat(root,env).ok,false);item.evidence=[];
 state.items['AE-001'].status='ready';fs.writeFileSync(path.join(root,'content/interview/state.json'),JSON.stringify(state));write();assert.equal(checkWechat(root,env).ok,false);
 const second={...q,id:'AE-002'};fs.writeFileSync(path.join(root,'content/interview/questions.json'),JSON.stringify({questions:[q,second]}));registry.items['AE-002']={...item};write();assert.ok(checkWechat(root,env).errors.some(e=>e.includes('编号重复')));
});
test('frozen static diagram registration requires provenance and authentic digest, rejects JPEG disguise',t=>{
 const {root}=fixture(t),base='public/diagrams/interview',img=base+'/diagram.jpg',src=base+'/diagram.jpg.source.json';
 fs.writeFileSync(path.join(root,img),Buffer.from([255,216,255,217]));
 const source={schema_version:1,kind:'frozen-static',title:'Diagram',provenance:'Existing author image',original_sha256:hash(path.join(root,img)),rebuildable:false};
 fs.writeFileSync(path.join(root,src),JSON.stringify(source));const registry={schema_version:1,diagrams:[{format:'frozen-static-v1',source:src,outputs:[img]}],sha256:{[src]:hash(path.join(root,src)),[img]:hash(path.join(root,img))}};
 const write=()=>fs.writeFileSync(path.join(root,base+'/sources.json'),JSON.stringify(registry));write();assert.doesNotThrow(()=>verifyDiagrams(root));
 source.provenance='';fs.writeFileSync(path.join(root,src),JSON.stringify(source));registry.sha256[src]=hash(path.join(root,src));write();assert.throws(()=>verifyDiagrams(root),/来源/);
 source.provenance='Existing';fs.writeFileSync(path.join(root,img),'not jpeg');source.original_sha256=hash(path.join(root,img));fs.writeFileSync(path.join(root,src),JSON.stringify(source));registry.sha256[src]=hash(path.join(root,src));registry.sha256[img]=hash(path.join(root,img));write();assert.throws(()=>verifyDiagrams(root),/JPEG/);
});
