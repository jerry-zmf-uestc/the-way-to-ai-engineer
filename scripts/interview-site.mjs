import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {project} from './interview.mjs';
import {publicFile} from './interview/workspace.mjs';
import {loadManifest,SITE_BASE as COURSE_BASE} from './book-content.mjs';

export const ROOT=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
export const INTERVIEW_BASE=COURSE_BASE+'/interview';
export const OUTPUT=path.join(ROOT,'.generated/interview');
export function loadInterview(root=ROOT){
  project(root).validate();
  const data=JSON.parse(fs.readFileSync(path.join(root,'content/interview/questions.json'),'utf8'));
  const website=JSON.parse(fs.readFileSync(path.join(root,'content/interview/website.json'),'utf8'));
  if(website.schema_version!==1||!Array.isArray(website.visible_questions)||new Set(website.visible_questions).size!==website.visible_questions.length)throw new Error('面试网站可见清单无效');
  const visible=website.visible_questions.map(id=>{
    const q=data.questions.find(q=>q.id===id);
    if(!q)throw new Error('网站题目不存在：'+id);
    const file=publicFile(root,q.article);
    if(!fs.existsSync(file))throw new Error('网站题目正文不存在：'+id);
    const text=fs.readFileSync(file,'utf8');
    if(!text.startsWith('# ')||/TODO|待填写|待核实/.test(text))throw new Error('网站正文不完整：'+id);
    return q;
  }).sort((a,b)=>a.order-b.order);
  return {...data,visible};
}
export function renderInterview(q,data,root=ROOT){
  root=fs.realpathSync(root);
  let text=fs.readFileSync(publicFile(root,q.article),'utf8');
  const heading=text.split('\n')[0].slice(2).trim();
  text=text.replace(/^# .+\r?\n+/,'');
  const course=loadManifest(path.join(root,'content/book/chapters.json'));
  function link(target){
    if(/^(?:https?:|mailto:|#)/.test(target))return target;
    const [relative,...hash]=target.split('#'),fragment=hash.length?'#'+hash.join('#'):'';
    const absolute=publicFile(root,path.relative(root,path.resolve(root,path.dirname(q.article),decodeURI(relative))));
    const rel=path.relative(root,absolute).split(path.sep).join('/');
    if(!fs.existsSync(absolute))throw new Error('网站链接缺失：'+target);
    const question=data.visible.find(q=>q.article===rel);
    if(question)return `${INTERVIEW_BASE}/questions/${question.id.toLowerCase()}/${fragment}`;
    if(rel==='content/interview/README.md')return `${INTERVIEW_BASE}/${fragment}`;
    const page=Object.entries(course).find(([,p])=>p.path===rel&&p.published);
    if(page)return `${COURSE_BASE}/chapters/${page[0]}/${fragment}`;
    if(rel.startsWith('public/'))return `${INTERVIEW_BASE}/${rel.slice('public/'.length)}${fragment}`;
    return `https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer/${fs.statSync(absolute).isDirectory()?'tree':'blob'}/main/${rel.split('/').map(encodeURIComponent).join('/')}${fragment}`;
  }
  text=text.split(/(^```[^\n]*\n[\s\S]*?^```\s*$)/m).map((part,i)=>i%2?part:part.replace(/(!?\[[^\]\n]*\]\()([^\)\n]+)(\))/g,(_,a,target,b)=>a+link(target)+b)).join('');
  const meta={questionId:q.id,title:heading,module:q.module,order:q.order,difficulty:q.difficulty,source:q.article};
  return '---\n'+Object.entries(meta).map(([key,value])=>`${key}: ${JSON.stringify(value)}`).join('\n')+'\n---\n\n'+text;
}
export function prepareInterview(root=ROOT,output=path.join(root,'.generated/interview')){
  const data=loadInterview(root);
  const rendered=data.visible.map(q=>[q.id.toLowerCase()+'.md',renderInterview(q,data,root)]);
  fs.mkdirSync(output,{recursive:true});
  const names=new Set(rendered.map(([name])=>name));
  for(const [name,text] of rendered){
    const file=path.join(output,name);
    if(!fs.existsSync(file)||fs.readFileSync(file,'utf8')!==text){const temp=file+'.tmp';try{fs.writeFileSync(temp,text);fs.renameSync(temp,file);}finally{fs.rmSync(temp,{force:true});}}
  }
  for(const file of fs.readdirSync(output))if(file.endsWith('.md')&&!names.has(file))fs.unlinkSync(path.join(output,file));
  return rendered.length;
}
if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
  try{console.log(`Interview site: ${prepareInterview()} article(s) prepared independently of course routes.`);}catch(error){console.error(error.message);process.exitCode=1;}
}
