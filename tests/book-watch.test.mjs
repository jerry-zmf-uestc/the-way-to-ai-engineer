import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync,statSync,existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { EventEmitter } from 'node:events';
import { prepareBook } from '../scripts/book-content.mjs';
import { watchBook } from '../scripts/watch-book.mjs';
import { startBookDev } from '../scripts/dev-book.mjs';

function fixture(){
  const root=mkdtempSync(resolve(tmpdir(),'course-watch-'));
  mkdirSync(resolve(root,'content/book'),{recursive:true});mkdirSync(resolve(root,'labs'));
  writeFileSync(resolve(root,'content/book/a.md'),'# A\n\ninitial\n');
  const manifest={a:{path:'content/book/a.md',title:'A',part:'Test',order:1,description:'Test',published:true,prerequisites:[],labs:[],capabilities:[]}};
  writeFileSync(resolve(root,'content/book/chapters.json'),JSON.stringify(manifest));
  return {root,manifest,output:resolve(root,'.generated/chapters/a.md')};
}
async function until(check){
  const end=Date.now()+4000;
  while(!check()){if(Date.now()>end)throw new Error('watch condition timed out');await new Promise(done=>setTimeout(done,25));}
}

test('generation writes only changed pages and preserves last valid data on render error',()=>{
  const f=fixture();try{
    prepareBook({rootDir:f.root,outputDir:resolve(f.root,'.generated/chapters')});
    const before=readFileSync(f.output,'utf8'),mtime=statSync(f.output).mtimeMs;
    prepareBook({rootDir:f.root,outputDir:resolve(f.root,'.generated/chapters')});assert.equal(statSync(f.output).mtimeMs,mtime);
    writeFileSync(resolve(f.root,'content/book/a.md'),'# A\n\n[missing](missing.md)\n');
    assert.throws(()=>prepareBook({rootDir:f.root,outputDir:resolve(f.root,'.generated/chapters')}),/unresolved/);
    assert.equal(readFileSync(f.output,'utf8'),before);
  }finally{rmSync(f.root,{recursive:true,force:true});}
});

test('real watcher updates, deletes unpublished page, surfaces errors, recovers and closes',async()=>{
  const f=fixture();let refreshed=0;const errors=[];
  const watcher=watchBook({rootDir:f.root,debounceMs:30,onReady:()=>refreshed++,onError:error=>errors.push(error)});
  try{
    writeFileSync(resolve(f.root,'content/book/a.md'),'# A\n\nchanged\n');
    await until(()=>refreshed>0&&readFileSync(f.output,'utf8').includes('changed'));
    writeFileSync(resolve(f.root,'content/book/chapters.json'),'{');await until(()=>errors.length>0);
    assert.ok(readFileSync(f.output,'utf8').includes('changed'));
    f.manifest.a.published=false;writeFileSync(resolve(f.root,'content/book/chapters.json'),JSON.stringify(f.manifest));
    await until(()=>!existsSync(f.output));
    f.manifest.a.published=true;writeFileSync(resolve(f.root,'content/book/chapters.json'),JSON.stringify(f.manifest));await until(()=>existsSync(f.output));
    const previous=refreshed;writeFileSync(resolve(f.root,'labs/meta.yml'),'source: updated\n');await until(()=>refreshed>previous);
    watcher.close();const stopped=refreshed;writeFileSync(resolve(f.root,'content/book/a.md'),'# A\n\nclosed\n');
    await new Promise(done=>setTimeout(done,100));assert.equal(refreshed,stopped);
  }finally{watcher.close();rmSync(f.root,{recursive:true,force:true});}
});

test('dev supervisor forwards arguments and closes its watcher after owned child exit/error',async()=>{
  for(const failure of [false,true]){
    const f=fixture();const child=new EventEmitter();child.kill=signal=>{queueMicrotask(()=>child.emit('exit',null,signal));return true;};
    let args,options;
    const before=process.listenerCount('SIGINT');
    const session=startBookDev({rootDir:f.root,args:['--host','127.0.0.1'],spawnChild:(_cmd,a,o)=>{args=a;options=o;return child;},shutdownMs:20});
    try{
      assert.deepEqual(args.slice(-2),['--host','127.0.0.1']);
      assert.equal(options.env.ASTRO_DEV_BACKGROUND,'1');
      if(failure)child.emit('error',new Error('startup failed'));else session.stop('SIGTERM');
      const result=await session.done;assert.equal(result.code,failure?1:0);assert.equal(process.listenerCount('SIGINT'),before);
      const old=readFileSync(f.output,'utf8');writeFileSync(resolve(f.root,'content/book/a.md'),'# A\n\nno watcher\n');await new Promise(done=>setTimeout(done,100));assert.equal(readFileSync(f.output,'utf8'),old);
    }finally{rmSync(f.root,{recursive:true,force:true});}
  }
});

test('shutdown forces only its stubborn child after grace and rejects detached modes',async()=>{
  const f=fixture();const child=new EventEmitter();const signals=[];
  const heartbeat=setInterval(()=>{},100);
  child.kill=signal=>{signals.push(signal);if(signal==='SIGKILL'){clearInterval(heartbeat);queueMicrotask(()=>child.emit('exit',null,signal));}return true;};
  const session=startBookDev({rootDir:f.root,spawnChild:()=>child,shutdownMs:20});
  try{
    session.stop('SIGTERM');await session.done;assert.deepEqual(signals,['SIGTERM','SIGKILL']);
    assert.throws(()=>startBookDev({rootDir:f.root,args:['--background']}),/unsupported/);
    assert.throws(()=>startBookDev({rootDir:f.root,args:['--ignore-lock']}),/unsupported/);
  }finally{clearInterval(heartbeat);rmSync(f.root,{recursive:true,force:true});}
});

test('production preview supervises its owned child without preparing or watching sources',async()=>{
  const f=fixture();const child=new EventEmitter();let args,options;
  child.kill=signal=>{queueMicrotask(()=>child.emit('exit',null,signal));return true;};
  const session=startBookDev({rootDir:f.root,command:'preview',args:['--host','127.0.0.1','--port','4321'],spawnChild:(_cmd,a,o)=>{args=a;options=o;return child;},shutdownMs:20});
  try{
    assert.equal(args[1],'preview');
    assert.equal(options.env.ASTRO_PREVIEW_BACKGROUND,'1');
    assert.equal(existsSync(f.output),false,'preview must serve the last build, not silently regenerate it');
    session.stop('SIGTERM');
    assert.equal((await session.done).code,0);
    assert.throws(()=>startBookDev({rootDir:f.root,command:'preview',args:['--background']}),/unsupported/);
  }finally{rmSync(f.root,{recursive:true,force:true});}
});
