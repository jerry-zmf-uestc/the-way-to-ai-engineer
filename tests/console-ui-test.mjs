import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function setup() {
  const nodes = new Map();
  const node = () => ({textContent:'', value:'', disabled:false, children:[], listeners:{}, appendChild(n){this.children.push(n);}, addEventListener(k,f){this.listeners[k]=f;}});
  for (const id of ['log','ask','q','stop']) nodes.set(id,node());
  const connections = [];
  class FakeSource {
    constructor(url){this.url=url;this.listeners={};this.closed=false;this.seq=0;this.runID="run-"+connections.length;connections.push(this);}
    addEventListener(k,f){this.listeners[k]=f;}
    close(){this.closed=true;}
    emit(k,data){this.listeners[k]?.({data:JSON.stringify({run_id:this.runID,seq:++this.seq,payload:data})});}
  }
  const document = {getElementById:id=>nodes.get(id),createElement:node,createTextNode:text=>({textContent:text})};
  const html = readFileSync(new URL('../labs/part3-agent/chapter32/mini-agent-console/web/index.html',import.meta.url),'utf8');
  vm.runInNewContext(html.match(/<script>([\s\S]*?)<\/script>/)[1], {document,EventSource:FakeSource,encodeURIComponent});
  return {nodes,connections,submit(q){nodes.get('q').value=q;nodes.get('ask').listeners.submit({preventDefault(){}});},text(){return nodes.get('log').children.map(n=>n.textContent).join('');}};
}

test('late token and done cannot mutate or stop a new request',()=>{
  const ui=setup();ui.submit('old');const old=ui.connections[0];ui.submit('new');const active=ui.connections[1];
  old.emit('token',{token:'STALE'});old.emit('done',{finish_reason:'stop'});
  assert.equal(old.closed,true);assert.equal(active.closed,false);assert.equal(ui.text().includes('STALE'),false);
  active.emit('token',{token:'CURRENT'});assert.equal(ui.text().includes('CURRENT'),true);
  active.emit('done',{finish_reason:'stop'});assert.equal(active.closed,true);
});
test('transport failure closes even a reconnecting EventSource and does not claim completion',()=>{
  const ui=setup();ui.submit('research context');const source=ui.connections[0];source.readyState=0;source.emit('error');
  assert.equal(source.closed,true);assert.match(ui.text(),/completion unknown/);assert.equal(ui.text().includes('finish_reason=stop'),false);
  source.emit('token',{token:'AFTER_ERROR'});assert.equal(ui.text().includes('AFTER_ERROR'),false);
});
test('stop is local cancellation, not a server acknowledgement',()=>{
  const ui=setup();ui.submit('question');ui.nodes.get('stop').listeners.click();assert.equal(ui.connections[0].closed,true);assert.match(ui.text(),/not acknowledged/);
});


test('invalid JSON, sequence, identity and payload close the stream',()=>{
  for (const data of ['{',JSON.stringify({run_id:'x',seq:2,payload:{token:'bad'}}),JSON.stringify({run_id:'x',seq:1,payload:{token:7}})]) {
    const ui=setup();ui.submit('question');const source=ui.connections[0];
    source.listeners.token({data});assert.equal(source.closed,true);assert.match(ui.text(),/invalid stream; completion unknown/);
  }
  const ui=setup();ui.submit('question');const source=ui.connections[0];source.emit('token',{token:'first'});
  source.listeners.token({data:JSON.stringify({run_id:'another',seq:2,payload:{token:'wrong'}})});
  assert.equal(source.closed,true);assert.equal(ui.text().includes('wrong'),false);
});
test('runtime error cannot be followed by a successful terminal',()=>{
  const ui=setup();ui.submit('question');const source=ui.connections[0];source.emit('agent_error',{message:'failed'});source.emit('done',{finish_reason:'stop'});
  assert.equal(source.closed,true);assert.equal(ui.text().includes('finish_reason=stop'),false);assert.match(ui.text(),/completion unknown/);
});
