import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {publicFile} from './workspace.mjs';

// Frozen delivery checks; these do not rerender Archify or certify diagram semantics.
export function verifyDiagrams(root) {
  const base='public/diagrams/interview';
  const registry=JSON.parse(fs.readFileSync(publicFile(root,base+'/sources.json'),'utf8'));
  if(registry.schema_version!==1 || !Array.isArray(registry.diagrams) || !registry.diagrams.length) throw new Error('面试图登记无效');
  const registered=new Set([base+'/sources.json']);
  for(const diagram of registry.diagrams){
    if(diagram.format!=='archify-workflow-v2' || !Array.isArray(diagram.outputs) || !diagram.outputs.length)throw new Error('面试图源格式无效');
    const source=JSON.parse(fs.readFileSync(publicFile(root,diagram.source),'utf8'));
    if(source.schema_version!==2 || source.diagram_type!=='workflow')throw new Error('面试图源结构无效');
    for(const file of [diagram.source,...diagram.outputs]){
      if(registered.has(file)||!file.startsWith(base+'/')||!(file in registry.sha256))throw new Error('面试图重复或未登记');
      registered.add(file);
    }
  }
  for(const [file,hash] of Object.entries(registry.sha256)){
    if(!registered.has(file))throw new Error('面试图摘要未关联图源');
    const bytes=fs.readFileSync(publicFile(root,file));
    if(crypto.createHash('sha256').update(bytes).digest('hex')!==hash)throw new Error('面试图字节已变化，需重新审查并登记');
    if(file.endsWith('.png') && !bytes.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10])))throw new Error('面试PNG格式无效');
    if(file.endsWith('.svg')){
      const text=bytes.toString('utf8');
      if(!/<svg\b/.test(text)||!/<title\b/.test(text)||/<!DOCTYPE|<!ENTITY|<(?:[\w-]+:)?(?:script|foreignObject|iframe|image|animate|set)\b|\son[\w:-]+\s*=|(?:href|src)\s*=\s*["'](?!#)|url\(\s*["']?(?!#)|@import|javascript:/i.test(text))throw new Error('面试SVG静态边界检查失败');
    }
  }
  function walk(dir){for(const ent of fs.readdirSync(dir,{withFileTypes:true})){const file=path.join(dir,ent.name);if(ent.isDirectory())walk(file);else if(!registered.has(path.relative(root,file).split(path.sep).join('/'))||ent.isSymbolicLink())throw new Error('存在未登记面试图文件');}}
  walk(path.join(root,base));
  return '面试图源、正式产物摘要与受限静态检查通过；未重建或验证语义';
}
