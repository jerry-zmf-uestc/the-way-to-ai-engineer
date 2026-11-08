import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {ROOT,INTERVIEW_BASE} from './interview-site.mjs';
const course=path.join(ROOT,'dist'),interview=path.join(ROOT,'dist-interview'),output=path.join(ROOT,'.pages-dist');
for(const dir of [course,interview])if(!fs.existsSync(path.join(dir,'index.html')))throw new Error('Build both sites before assembling Pages');
if(fs.existsSync(path.join(course,'interview')))throw new Error('Course output collides with independent interview site');
const html=fs.readFileSync(path.join(interview,'index.html'),'utf8');
if(!html.includes(INTERVIEW_BASE+'/'))throw new Error('Interview build base does not match shared Pages mount');
const stage=path.join(ROOT,`.pages-dist-${crypto.randomUUID()}`);
try{
 fs.cpSync(course,stage,{recursive:true});fs.cpSync(interview,path.join(stage,'interview'),{recursive:true});
 if(!fs.readFileSync(path.join(stage,'index.html'),'utf8').includes('The Way to AI Engineer'))throw new Error('Course homepage missing from Pages bundle');
 fs.rmSync(output,{recursive:true,force:true});fs.renameSync(stage,output);
 console.log('Pages bundle: course at existing root, independent handbook at /interview/; no deployment performed.');
}finally{fs.rmSync(stage,{recursive:true,force:true});}
