import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {ROOT,loadInterview,INTERVIEW_BASE} from '../scripts/interview-site.mjs';
const output=path.join(ROOT,'dist-interview'),data=loadInterview();
const read=f=>fs.readFileSync(path.join(output,f),'utf8');
test('independent homepage lists all active questions and links only explicitly visible articles',()=>{
 const html=read('index.html');assert.equal((html.match(/<li\b[^>]*\bdata-question(?:\s|>)/g)||[]).length,data.questions.length);assert.equal((html.match(/<section\b[^>]*\bdata-topic(?:\s|>)/g)||[]).length,10);
 for(const q of data.questions){const route=`${INTERVIEW_BASE}/questions/${q.id.toLowerCase()}/`;assert.equal(html.includes(`href="${route}"`),data.visible.some(v=>v.id===q.id),q.id);}
 assert.ok(html.includes(`当前可阅读 ${data.visible.length} 篇`));assert.ok(!html.includes('CourseNavigation')&&!html.includes('课程目录'));
 assert.ok(html.includes('question-search'));assert.ok(!html.includes('readonly')&&!html.includes('TODO'));
});
test('article pages have independent canonical URL, active navigation, working anchors and images',()=>{
 for(const q of data.visible){const html=read(`questions/${q.id.toLowerCase()}/index.html`);
  assert.ok(html.includes(`href="https://jerry-zmf-uestc.github.io${INTERVIEW_BASE}/questions/${q.id.toLowerCase()}/"`));
  assert.ok(html.includes('aria-current="page"'));assert.ok(html.includes('aria-label="本篇目录"'));
  assert.ok(!html.includes('课程目录')&&!html.includes('TODO'));
  for(const [,src] of html.matchAll(/<img\b[^>]*src="([^"]+)"/g)){assert.ok(src.startsWith(INTERVIEW_BASE+'/diagrams/interview/')||src.startsWith(INTERVIEW_BASE+'/images/interview/'));assert.ok(fs.existsSync(path.join(output,src.slice(INTERVIEW_BASE.length+1))));}
  for(const [,id] of html.matchAll(/href="#([^"]+)"/g))assert.ok(html.includes(`id="${id}"`),id);
 }
});
test('interview output has no course routes, hidden questions or private production data',()=>{
 assert.ok(!fs.existsSync(path.join(output,'chapters')));
 assert.deepEqual(fs.readdirSync(path.join(output,'questions')).sort(),data.visible.map(q=>q.id.toLowerCase()).sort());
 for(const dir of ['research','reviews','plans','.archify','content'])assert.ok(!fs.existsSync(path.join(output,dir)));
});
