import fs from 'node:fs';
import path from 'node:path';
import {prepareInterview,ROOT} from './interview-site.mjs';
export function watchInterviewSite({rootDir=ROOT,pollMs=250,debounceMs=120,onError=error=>process.stderr.write(`interview watch: ${error.message}\n`)}={}){
  const snapshot=()=>{
    const rows=[];
    function visit(dir){for(const entry of fs.readdirSync(dir,{withFileTypes:true})){const file=path.join(dir,entry.name);if(entry.isDirectory())visit(file);else if(/\.(md|json)$/.test(entry.name)){const stat=fs.statSync(file,{bigint:true});rows.push(`${file}:${stat.ino}:${stat.mtimeNs}:${stat.size}`);}}}
    visit(path.join(rootDir,'content/interview'));
    const manifest=path.join(rootDir,'content/book/chapters.json');rows.push(fs.readFileSync(manifest,'utf8'));
    return rows.sort().join('\n');
  };
  prepareInterview(rootDir);let previous=snapshot(),timer,lastError='';
  const interval=setInterval(()=>{try{const current=snapshot();if(current===previous)return;previous=current;clearTimeout(timer);timer=setTimeout(()=>{try{prepareInterview(rootDir);lastError='';}catch(error){onError(error);}},debounceMs);}catch(error){if(lastError!==error.message){onError(error);lastError=error.message;}}},pollMs);
  return {close(){clearInterval(interval);clearTimeout(timer);}};
}
