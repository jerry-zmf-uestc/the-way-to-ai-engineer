import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const defaultRoot=process.cwd();
export function checkLinks(root=defaultRoot, repositoryRoot=root) {
  root=path.resolve(root);repositoryRoot=fs.realpathSync(repositoryRoot);
  const broken=[];let count=0;
  function visit(dir){
    for(const entry of fs.readdirSync(dir,{withFileTypes:true})){
      if(['.git','.specs','.tools','.astro','.generated','.astro-interview','.pages-dist','node_modules','dist','dist-interview'].includes(entry.name))continue;
      const file=path.join(dir,entry.name);
      if(entry.isDirectory()){if(path.relative(repositoryRoot,file).split(path.sep).join('/')!=='.generated/chapters')visit(file);continue;}
      if(!file.endsWith('.md'))continue;count++;
      const prose=fs.readFileSync(file,'utf8').replace(/```[\s\S]*?```/g,'').replace(/`[^`\n]+`/g,'');
      for(const match of prose.matchAll(/\[[^\]]*\]\(([^)]+)\)/g)){
        const link=match[1];if(/^(https?:|mailto:|#)/.test(link))continue;
        let target;
        try { target=path.resolve(path.dirname(file),decodeURIComponent(link.split('#')[0])); }
        catch {broken.push(`${path.relative(root,file)}: invalid link`);continue;}
        if(!fs.existsSync(target)){broken.push(`${path.relative(root,file)}: ${link}`);continue;}
        const resolved=fs.realpathSync(target),relative=path.relative(repositoryRoot,resolved);
        if(relative==='..'||relative.startsWith('..'+path.sep)||path.isAbsolute(relative)||relative.split(path.sep).some(p=>['.git','.specs','.tools'].includes(p)))broken.push(`${path.relative(root,file)}: private target or outside repository: ${link}`);
      }
    }
  }
  visit(root);if(!count)broken.push('no public Markdown files found');return broken;
}
if (process.argv[1] && path.resolve(process.argv[1])===fileURLToPath(import.meta.url)) {
  const scope=process.argv[2] ? path.resolve(defaultRoot,process.argv[2]) : defaultRoot;
  const relative=path.relative(defaultRoot,scope);
  if(relative==='..'||relative.startsWith('..'+path.sep)||path.isAbsolute(relative))throw new Error('link check scope outside repository');
  const broken=checkLinks(scope,defaultRoot);
  if(broken.length){console.error(broken.join('\n'));process.exitCode=1;}else console.log('Markdown links: public relative targets resolve without private files');
}
