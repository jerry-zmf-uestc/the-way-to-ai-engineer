import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {ROOT} from '../scripts/interview-site.mjs';
import {SITE_BASE} from '../scripts/book-content.mjs';
const bundle=path.join(ROOT,'.pages-dist');
function files(dir){return fs.readdirSync(dir,{withFileTypes:true}).flatMap(e=>e.isDirectory()?files(path.join(dir,e.name)):[path.join(dir,e.name)]);}
test('shared Pages bundle preserves course HTML byte for byte and adds independent handbook',()=>{
 for(const file of files(path.join(ROOT,'dist')).filter(f=>f.endsWith('.html')))assert.deepEqual(fs.readFileSync(path.join(bundle,path.relative(path.join(ROOT,'dist'),file))),fs.readFileSync(file));
 assert.deepEqual(fs.readFileSync(path.join(bundle,'interview/index.html')),fs.readFileSync(path.join(ROOT,'dist-interview/index.html')));
 assert.ok(fs.existsSync(path.join(bundle,'interview/questions/ae-001/index.html')));
});
test('all built local page, script, stylesheet and image references resolve in the shared bundle',()=>{
 for(const file of files(bundle).filter(f=>f.endsWith('.html'))){const html=fs.readFileSync(file,'utf8');for(const [,url] of html.matchAll(/(?:href|src)="([^"]+)"/g)){
  if(!url.startsWith(SITE_BASE+'/'))continue;
  const relative=decodeURI(url.slice(SITE_BASE.length+1).split('#')[0].split('?')[0]);
  let target=path.join(bundle,relative);if(fs.existsSync(target)&&fs.statSync(target).isDirectory())target=path.join(target,'index.html');
  assert.ok(fs.existsSync(target),`${path.relative(bundle,file)} → ${url}`);
 }}
});
