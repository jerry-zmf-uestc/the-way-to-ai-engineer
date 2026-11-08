import { spawn } from 'node:child_process';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { ROOT } from './book-content.mjs';
import { watchBook } from './watch-book.mjs';

export function startBookDev({rootDir=ROOT,args=process.argv.slice(2),spawnChild=spawn,shutdownMs=5000,command='dev',watcherFactory=watchBook}={}){
  if(!['dev','preview'].includes(command))throw new Error('unsupported book server command');
  if(args.includes('--background')||args.includes('--ignore-lock'))throw new Error('book server keeps one owned foreground server; background/ignore-lock are unsupported');
  const watcher=command==='dev'?watcherFactory({rootDir,onReady:pages=>process.stdout.write(`book watch: ${pages.length} pages refreshed\n`)}):{close(){}};
  let child,stopping=false,timer;
  const cleanup=()=>{watcher.close();clearTimeout(timer);process.off('SIGINT',interrupt);process.off('SIGTERM',terminate);};
  const stop=signal=>{
    if(stopping)return;stopping=true;watcher.close();
    child.kill(signal);
    timer=setTimeout(()=>child.kill('SIGKILL'),shutdownMs);timer.unref();
  };
  const interrupt=()=>stop('SIGINT'),terminate=()=>stop('SIGTERM');
  // The pinned CLI uses this child marker to avoid agent-environment auto-detach.
  const marker=command==='dev'?'ASTRO_DEV_BACKGROUND':'ASTRO_PREVIEW_BACKGROUND';
  try {child=spawnChild(process.execPath,[resolve(rootDir,'node_modules/astro/bin/astro.mjs'),command,...args],{cwd:rootDir,stdio:'inherit',env:{...process.env,[marker]:'1'}});} catch(error){cleanup();throw error;}
  process.on('SIGINT',interrupt);process.on('SIGTERM',terminate);
  const done=new Promise(resolveDone=>{
    child.once('error',error=>{cleanup();resolveDone({code:1,error});});
    child.once('exit',(code,signal)=>{cleanup();resolveDone({code:stopping?0:(code??1),signal});});
  });
  return {child,done,stop};
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  try {const session=startBookDev();const result=await session.done;process.exitCode=result.code;if(result.error)process.stderr.write(`book dev: ${result.error.message}\n`);}catch(error){process.stderr.write(`book dev: ${error.message}\n`);process.exitCode=1;}
}
