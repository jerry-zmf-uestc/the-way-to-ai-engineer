import test from 'node:test';
import assert from 'node:assert/strict';
import { enhanceReading } from '../src/scripts/reading-tools.mjs';

function setup() {
  class Element {
    constructor(tag) { this.tag=tag;this.children=[];this.dataset={};this.attributes={};this.listeners={};this.ownerDocument=document;this.parent=null;this._text=''; }
    get textContent(){return this._text+this.children.map(child=>child.textContent).join('');}
    set textContent(value){this._text=value;this.children=[];}
    setAttribute(key,value){this.attributes[key]=value;}
    getAttribute(key){return this.attributes[key]??null;}
    addEventListener(key,fn){this.listeners[key]=fn;}
    append(...children){for(const child of children){if(child.parent)child.parent.children=child.parent.children.filter(value=>value!==child);this.children.push(child);child.parent=this;}}
    before(node){const parent=this.parent;const index=parent.children.indexOf(this);parent.children.splice(index,0,node);node.parent=parent;}
    querySelectorAll(tag){return this.children.flatMap(child=>[...(child.tag===tag?[child]:[]),...child.querySelectorAll(tag)]);}
    querySelector(tag){return this.querySelectorAll(tag)[0]??null;}
    closest(tag){for(let node=this;node;node=node.parent)if(node.tag===tag)return node;return null;}
  }
  const document={createElement:tag=>new Element(tag)};
  const root=document.createElement('div');
  const pre=document.createElement('pre'),code=document.createElement('code');
  code.textContent='\tvalue := "中 文"\n\n  // preserve spaces\n';pre.append(code);root.append(pre);
  return {root,pre,code,document};
}
const page='https://example.test/the-way-to-ai-engineer/chapters/agent-loop/';
const base='/the-way-to-ai-engineer';

test('copy preserves code bytes, avoids duplicate enhancements and reports only settled success',async()=>{
  const ui=setup();let resolve;const copied=[];
  const clipboard={writeText:text=>{copied.push(text);return new Promise(done=>{resolve=done;});}};
  enhanceReading(ui.root,clipboard,page,base);enhanceReading(ui.root,clipboard,page,base);
  const buttons=ui.root.querySelectorAll('button');assert.equal(buttons.length,1);
  const button=buttons[0],status=ui.root.querySelector('span');
  const pending=button.listeners.click();assert.equal(button.disabled,true);assert.equal(status.textContent,'正在复制');
  await button.listeners.click();assert.equal(copied.length,1);
  assert.equal(copied[0],ui.code.textContent);resolve();await pending;
  assert.equal(button.disabled,false);assert.equal(status.textContent,'已复制');assert.equal(status.getAttribute('role'),'status');
});

test('unavailable or rejected clipboard never reports success and enables manual fallback',async()=>{
  for(const clipboard of [undefined,{writeText:async()=>{throw new Error('denied');}}]){
    const ui=setup();enhanceReading(ui.root,clipboard,page,base);await ui.root.querySelector('button').listeners.click();
    assert.match(ui.root.querySelector('span').textContent,/未复制.*手动复制/);assert.equal(ui.root.querySelector('button').disabled,false);
  }
});

test('only local diagram SVGs get safe original-size links, existing links remain',()=>{
  const ui=setup();
  for(const src of [`${base}/diagrams/loop.svg`,'https://other.test/diagrams/a.svg',`${base}/photo.png`,'http://[invalid']){
    const img=ui.document.createElement('img');img.setAttribute('src',src);img.setAttribute('alt','执行流程');ui.root.append(img);
  }
  const existing=ui.document.createElement('a'),linked=ui.document.createElement('img');linked.setAttribute('src',`${base}/diagrams/existing.svg`);existing.append(linked);ui.root.append(existing);
  enhanceReading(ui.root,{writeText:async()=>{}},page,base);enhanceReading(ui.root,{writeText:async()=>{}},page,base);
  const links=ui.root.querySelectorAll('a');assert.equal(links.length,2);
  const original=links.find(link=>link!==existing);assert.equal(original.href,`https://example.test${base}/diagrams/loop.svg`);assert.equal(original.target,'_blank');assert.equal(original.rel,'noopener');assert.match(original.getAttribute('aria-label'),/执行流程.*新窗口/);
  assert.equal(ui.root.querySelectorAll('img').length,5);
});
