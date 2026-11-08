import { readdirSync, statSync } from 'node:fs';
import { resolve } from 'node:path';
import { prepareBook, ROOT } from './book-content.mjs';

function snapshot(rootDir) {
  const files=new Map();
  function visit(folder,track) {
    for(const entry of readdirSync(folder,{withFileTypes:true})) {
      const path=resolve(folder,entry.name);
      if(entry.isDirectory())visit(path,track);
      else if(entry.isFile()&&((track==='content/book'&&(entry.name.endsWith('.md')||entry.name==='chapters.json'||entry.name==='redirects.json'))||(track==='labs'&&entry.name==='meta.yml'))) {
        const stat=statSync(path,{bigint:true});
        files.set(path,`${stat.ino}:${stat.mtimeNs}:${stat.size}`);
      }
    }
  }
  visit(resolve(rootDir,'content/book'),'content/book');visit(resolve(rootDir,'labs'),'labs');
  return files;
}
function equal(a,b){return a.size===b.size&&[...a].every(([key,value])=>b.get(key)===value);}

export function watchBook({rootDir=ROOT,outputDir=resolve(rootDir,'.generated/chapters'),onReady=()=>{},onError=error=>process.stderr.write(`book watch: ${error.message}\n`),debounceMs=120,pollMs=250}={}) {
  prepareBook({rootDir,outputDir});
  let previous=snapshot(rootDir),timer,closed=false,lastError='';
  const interval=setInterval(()=>{
    if(closed)return;
    try {
      const current=snapshot(rootDir);
      if(equal(previous,current))return;
      previous=current;clearTimeout(timer);
      timer=setTimeout(()=>{
        try {const generated=prepareBook({rootDir,outputDir});lastError='';onReady(generated);} catch(error){onError(error);}
      },debounceMs);
    } catch(error) {
      if(lastError!==error.message){lastError=error.message;onError(error);}
    }
  },pollMs);
  return {close(){closed=true;clearInterval(interval);clearTimeout(timer);}};
}
