import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import {verifyDiagrams} from './interview/diagrams.mjs';
import {checkWechat} from './interview/wechat.mjs';
import {workspaceFile, publicFile} from './interview/workspace.mjs';

const defaultRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const statuses = ['planned', 'researching', 'drafting', 'reviewing', 'ready', 'published'];
const checks = ['facts', 'engineering', 'clarity', 'sources', 'safety'];
const transitions = { planned: ['researching'], researching: ['drafting'], drafting: ['reviewing'], reviewing: ['drafting', 'ready'], ready: ['drafting', 'published'], published: [] };
const fail = message => { throw new Error(message); };
export function dateValid(s) {
  return typeof s === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(s) && Number.isFinite(Date.parse(s)) && new Date(s).toISOString().slice(0, 10) === s;
}
function urlValid(s) { try { return new URL(s).protocol === 'https:'; } catch { return false; } }
function atomicWrite(file, text) {
  const tmp = `${file}.${crypto.randomUUID()}.tmp`;
  try { fs.writeFileSync(tmp, text, { flag: 'wx' }); fs.renameSync(tmp, file); }
  finally { if (fs.existsSync(tmp)) fs.unlinkSync(tmp); }
}
export function project(root = defaultRoot, env = process.env) {
  const base = path.join(root, 'content/interview');
  const read = name => JSON.parse(fs.readFileSync(path.join(base, name), 'utf8'));
  const data = read('questions.json'), state = read('state.json');
  const question = id => data.questions.find(q => q.id === id) ?? fail(`未知题号：${id}`);
  const articlePath = q => publicFile(root, q.article);
  const researchPath = id => workspaceFile(`research/${id}.md`,env);
  const reviewPath = id => workspaceFile(`reviews/${id}.json`,env);
  const hash = id => crypto.createHash('sha256').update(fs.readFileSync(articlePath(question(id)))).digest('hex');
  const requireText = (file, complete = false) => {
    if (!fs.existsSync(file) || !fs.readFileSync(file, 'utf8').trim()) fail(`缺少内容：${path.relative(root, file)}`);
    if (complete && /TODO|待填写|待核实/.test(fs.readFileSync(file, 'utf8'))) fail(`仍有待完成内容：${path.relative(root, file)}`);
  };
  function ready(id) {
    requireText(articlePath(question(id)), true); requireText(researchPath(id), true);
    requireText(reviewPath(id));
    const review = JSON.parse(fs.readFileSync(reviewPath(id), 'utf8'));
    const ranks = {design:1,runnable:2,measured:3};
    if ((!(review.evidence_notes?.trim()) || /TODO|待填写/.test(review.evidence_notes) || !(ranks[review.evidence_level] >= ranks[question(id).minimum_evidence]))) fail(`${id} 工程证据等级或记录不足`);
    if (review.id !== id || !review.reviewer?.trim() || !dateValid(review.reviewed_at) || !review.notes?.trim() || /TODO|待填写/.test(review.notes) || checks.some(k => review.checks?.[k] !== true) || review.article_sha256 !== hash(id)) fail(`${id} 审核未完成或正文已变化`);
  }
  function validate(skipReady = null) {
    if (data.schema_version !== 3 || state.schema_version !== 1) fail('不支持的 schema_version');
    if (['plans','exports','.archify','research','reviews','scripts','templates','articles','assets','docs'].some(name => fs.existsSync(path.join(root,'content/interview',name)))) fail('发行回执、导出与运行材料应位于仓库外');
    if(fs.existsSync(path.join(root,'editorial')) || fs.existsSync(path.join(base,'calendar.json')))fail('旧编辑目录与发布日历已移除，不应重建');
    if (data.questions.length !== 50) fail('题目数量必须为 50');
    const ids = new Set(data.questions.map(q => q.id));
    if (ids.size !== 50 || data.questions.some(q => !/^AE-\d{3}$/.test(q.id))) fail('ID 必须唯一且为 AE-三位数字');
    if (data.plan_source !== 'docs/designs/interview-handbook.md' || !fs.existsSync(path.join(root,data.plan_source))) fail('方案来源无效');
    const validProposal = id => /^P-0[0-9]{2}$/.test(id) && id !== 'P-000';
    data.questions.forEach((q,i) => {
      if (q.order !== i+1 || !validProposal(q.proposal_id) || (i > 0 && q.proposal_id <= data.questions[i-1].proposal_id) || !q.proposal_title?.trim()) fail(`题序或标题不一致：${q.id}`);
      if (!['design','runnable','measured'].includes(q.minimum_evidence) || !q.acceptance_case?.trim()) fail(`缺少证据等级或验收用例：${q.id}`);
    });
    if (data.questions[0].id !== 'AE-001') fail('首题必须保持 AE-001');
    const retired = data.retired_questions;
    if (!Array.isArray(retired) || new Set(retired.map(r => r.question?.id)).size !== retired.length || retired.some(r => !/^AE-\d{3}$/.test(r.question?.id) || ids.has(r.question.id) || !ids.has(r.covered_by) || r.last_state?.status !== 'planned' || r.last_state?.history?.length !== 0)) fail('封存ID重复、复用或记录无效');
    const proposals = [...data.questions, ...retired.map(r => r.question)].map(q => q.proposal_id).filter(Boolean);
    if (proposals.length !== 99 || new Set(proposals).size !== 99 || proposals.some(id => !validProposal(id))) fail('原提案 P-001 至 P-099 必须各保留一次');
    const modules = new Set(data.modules.map(m => m.id));
    if (modules.size !== 10 || data.modules.length !== 10) fail('必须有 10 个唯一模块');
    const quotas=[5,4,4,6,4,5,3,5,6,8];
    const directories=['reasoning-planning','tool-calling','context-memory','rag','evaluation-observability','runtime-harness','multi-agent','mcp-external-tools','production-engineering','system-design'];
    data.modules.forEach((m,i) => {
      if(m.count!==quotas[i] || m.directory!==directories[i] || data.questions.filter(q=>q.module===m.id).length!==m.count) fail(`模块题数或目录错误：${m.id}`);
    });
    const chapters=JSON.parse(fs.readFileSync(path.join(root,'content/book/chapters.json'),'utf8'));
    const articles=new Set();
    for(const q of data.questions){
      const m=data.modules.find(m=>m.id===q.module);
      if(!m || !new RegExp(`^content/interview/${m.directory}/${q.id.toLowerCase()}-[a-z0-9]+(?:-[a-z0-9]+)*\\.md$`).test(q.article) || articles.has(q.article)) fail(`正文路径无效：${q.id}`);
      articles.add(q.article);articlePath(q);
      if(!Array.isArray(q.course_refs) || !q.course_refs.length || new Set(q.course_refs).size!==q.course_refs.length || q.course_refs.some(c=>!chapters[c] || !fs.existsSync(publicFile(root,chapters[c].path)))) fail(`课程关联无效：${q.id}`);
      const labs=new Set(q.course_refs.flatMap(c=>chapters[c].labs));
      if(!Array.isArray(q.lab_refs) || new Set(q.lab_refs).size!==q.lab_refs.length || q.lab_refs.some(l=>!labs.has(l) || !fs.existsSync(publicFile(root,l+'/README.md')) || !fs.existsSync(publicFile(root,l+'/meta.yml')))) fail(`Lab关联无效：${q.id}`);
      if(!['complete','partial','gap'].includes(q.coverage?.status) || !q.coverage?.notes?.trim()) fail(`覆盖说明无效：${q.id}`);
    }
    for(const directory of directories){const file=path.join(base,directory);if(!fs.existsSync(file)||!fs.lstatSync(file).isDirectory())fail('缺少大章节目录：'+directory);}
    function visitContent(dir){
      for(const ent of fs.readdirSync(dir,{withFileTypes:true})){
        const file=path.join(dir,ent.name),rel=path.relative(root,file).split(path.sep).join('/');
        if(ent.isDirectory()) {if(path.dirname(rel)!=='content/interview'||!directories.includes(ent.name))fail('面试目录仅允许大章节');visitContent(file);}
        else if(ent.isSymbolicLink() || !((path.dirname(rel)==='content/interview' && ['README.md','questions.json','state.json','website.json','.state.lock'].includes(ent.name)) || (directories.includes(path.basename(path.dirname(file))) && ent.name==='.gitkeep' && fs.statSync(file).size===0) || articles.has(rel)))fail(`面试目录仅允许总导航与已登记正文：${rel}`);
      }
    }
    visitContent(path.join(root,'content/interview'));
    verifyDiagrams(root);
    for(const file of articles){if(!fs.existsSync(articlePath({article:file})))continue;const text=fs.readFileSync(articlePath({article:file}),'utf8');for(const match of text.matchAll(/!\[([^\]]*)\]\(([^)]+)\)/g)){const target=path.resolve(path.dirname(path.join(root,file)),match[2]),rel=path.relative(root,target).split(path.sep).join('/');if(!match[1].trim()||!(rel.startsWith('public/diagrams/interview/')||rel.startsWith('public/images/interview/'))||!fs.existsSync(publicFile(root,rel)))fail('正文配图缺少替代文字或正式文件');}}
    if (new Set(data.questions.map(q => q.title)).size !== 50) fail('题目标题重复');
    if (Object.keys(state.items).length !== 50 || Object.keys(state.items).some(id => !ids.has(id))) fail('状态记录与题库不一致');
    const visited = new Set(), visiting = new Set();
    function visit(id) {
      if (visiting.has(id)) fail(`前置依赖有环：${id}`);
      if (visited.has(id)) return;
      visiting.add(id);
      const q = question(id);
      if (!Array.isArray(q.prerequisites) || new Set(q.prerequisites).size !== q.prerequisites.length) fail('前置依赖格式错误');
      for (const p of q.prerequisites) { if (!ids.has(p)) fail(`前置题不存在：${p}`); visit(p); }
      visiting.delete(id); visited.add(id);
    }
    for (const q of data.questions) {
      if (!modules.has(q.module) || !['basic','intermediate','advanced','system-design'].includes(q.difficulty) || !q.title?.trim() || !q.focus?.trim()) fail(`元数据无效：${q.id}`);
      visit(q.id);
      const item = state.items[q.id];
      if (!item || !statuses.includes(item.status) || !Array.isArray(item.history)) fail(`状态无效：${q.id}`);
      let previous = 'planned';
      for (const h of item.history) {
        if (h.from !== previous || !transitions[previous].includes(h.to) || !Number.isFinite(Date.parse(h.at))) fail(`状态历史无效：${q.id}`);
        previous = h.to;
      }
      if (previous !== item.status) fail(`状态与历史不一致：${q.id}`);
      if (['drafting','reviewing','ready','published'].includes(item.status)) requireText(articlePath(q));
      if (['ready','published'].includes(item.status) && q.id !== skipReady) requireText(articlePath(q),true);
      if (item.status === 'published' && (!dateValid(item.published_at) || !urlValid(item.published_url))) fail(`缺少发布日期或链接：${q.id}`);
      if (item.status !== 'published' && (item.published_at !== null || item.published_url !== null)) fail(`未发布题不应带发布记录：${q.id}`);
    }
    return '50 题、模块、依赖、课程/Lab关联、十大章节及状态结构校验通过；不代表私有审核通过';
  }

  function scaffold(id) {
    validate(); const q = question(id);
    const apply = text => text.replace(/\{\{(id|title|focus|proposal_id|minimum_evidence|acceptance_case)\}\}/g, (_, key) => q[key]);
    const review = {id, reviewer:'', reviewed_at:null, article_sha256:null, notes:'', evidence_level:null, evidence_notes:'', checks:Object.fromEntries(checks.map(k => [k,false]))};
    const files = [
      [articlePath(q), apply(fs.readFileSync(path.join(root,'docs/templates/interview-article.md'),'utf8'))],
      [researchPath(id), apply(fs.readFileSync(path.join(root,'docs/templates/interview-research.md'),'utf8'))],
      [reviewPath(id), JSON.stringify(review,null,2)+'\n']
    ];
    return files.map(([file, content]) => {
      fs.mkdirSync(path.dirname(file), {recursive:true});
      try { fs.writeFileSync(file,content,{flag:'wx'}); if(file===articlePath(q)){const keep=path.join(path.dirname(file),'.gitkeep');if(fs.existsSync(keep) && fs.statSync(keep).size===0)fs.unlinkSync(keep);} return `创建 ${path.relative(root,file)}`; }
      catch(e) { if(e.code !== 'EEXIST') throw e; return `保留 ${path.relative(root,file)}`; }
    }).join('\n');
  }
  function transition(id, target, options={}) {
    validate(target === 'drafting' && state.items[id]?.status === 'ready' ? id : null); question(id); const item = state.items[id];
    if (!transitions[item.status].includes(target)) fail(`不允许状态转换：${item.status} → ${target}`);
    if (['drafting','reviewing'].includes(target)) { requireText(articlePath(question(id))); requireText(researchPath(id)); }
    if (target === 'reviewing') requireText(reviewPath(id));
    if (['ready','published'].includes(target)) ready(id);
    if (target === 'published' && (!dateValid(options.date) || !urlValid(options.url))) fail('发布登记需要有效 --date 和 https --url');
    const lock = path.join(base,'.state.lock'); let fd;
    try {
      fd = fs.openSync(lock,'wx');
      if (JSON.stringify(read('state.json')) !== JSON.stringify(state)) fail('状态被其他进程修改，请重试');
      item.history.push({from:item.status,to:target,at:new Date().toISOString()}); item.status=target;
      if (target === 'published') { item.published_at=options.date; item.published_url=options.url; }
      atomicWrite(path.join(base,'state.json'),JSON.stringify(state,null,2)+'\n');
    } finally { if(fd !== undefined) { fs.closeSync(fd); fs.unlinkSync(lock); } }
    return `${id} → ${target}`;
  }
  function orderedQuestions() {
    // Stable topological order: choose the lowest proposal order among currently available topics.
    const ordered=[], pending=[...data.questions];
    const seen=new Set();
    while(pending.length){
      const index=pending.findIndex(q=>q.prerequisites.every(id=>seen.has(id)));
      if(index<0)fail('前置依赖有环');
      const [q]=pending.splice(index,1);ordered.push(q);seen.add(q.id);
    }
    return ordered;
  }
  function render() {
    validate();
    atomicWrite(path.join(root,'content/interview/README.md'), navigation());
    return '已更新面试手册总导航';
  }
  function navigation() {
    const intro='# Agent 工程师面试手册 · '+data.questions.length+' 题\n\n题库title是本期主题，按order组织公众号科普文章，优先保存草稿并回读，最终正文归档后供网站同稿预览。课程见 [book](../book/README.md)，实现与实验见 [实验约定](../../docs/rules/topic-contract.md)。题库是主题定义真源，已创建正文才生成链接；十大章节目录预先建立，空目录仅用 .gitkeep 保留，不创建空白文章。\n\n运行、研究、审核及草稿边界见 [内容规范](../../docs/rules/interview-content-contract.md)，整体设计见 [面试方案](../../docs/designs/interview-handbook.md)，豆包工作创建和更新公众号稿件见 [公众号规则](../../docs/rules/interview-wechat.md)。不接入课程章号；独立面试站点通过 website.json 控制可见题目，与公众号发布状态分开。\n\n在仓库根目录运行 `node scripts/interview/issue.mjs next` 按列表选择下一篇新稿，`node scripts/interview/issue.mjs start AE-ID` 在私有区准备本期；保存核验后归档。旧next-work/scaffold保留历史编辑兼容，不作为公众号新稿入口。`node scripts/interview.mjs render` 更新导航。题库与状态直接保存在本目录的 `questions.json`、`state.json`；状态用 `status`，关联缺口用 `coverage` 查看；`make validate-interview` 检查公共结构，`node scripts/interview.mjs delivery-check AE-ID` 单独检查本地交付证据。\n\n<!-- generated: questions.json; edit catalogue then render -->\n\n';
    return intro+data.modules.map(m=>`## ${m.name}（${m.count} 题）\n\n| 来源编号 | 永久 ID | 题目 | 难度 |\n| --- | --- | --- | --- |\n`+data.questions.filter(q=>q.module===m.id).map(q=>`| ${q.proposal_id} | ${q.id} | ${fs.existsSync(articlePath(q))?`[${q.title}](${path.relative(path.join(root,'content/interview'),articlePath(q)).split(path.sep).join('/')})`:q.title+'（待编写）'} | ${q.difficulty} |`).join('\n')).join('\n\n')+'\n';
  }
  function checkNavigation(){if(fs.readFileSync(path.join(root,'content/interview/README.md'),'utf8')!==navigation())fail('总导航过期，请运行 render');return '总导航与题库一致';}
  function coverage(){
    const chapters=JSON.parse(fs.readFileSync(path.join(root,'content/book/chapters.json'),'utf8'));
    return '# '+data.questions.length+' 题课程与实验关联\n\n覆盖是内容关联，不是题目完成或效果验收。\n\n'+data.questions.map(q=>`## ${q.proposal_id} / ${q.id} ${q.title}\n\n- 状态：${q.coverage.status}\n- 课程：${q.course_refs.map(c=>chapters[c].title+' ['+c+']').join('；')}\n- 实验：${q.lab_refs.join('；')||'无'}\n- 分工与缺口：${q.coverage.notes}\n- 最低证据：${q.minimum_evidence}；验收：${q.acceptance_case}\n`).join('\n');
  }
  function status(){return statuses.map(s=>`${s}: ${Object.values(state.items).filter(i=>i.status===s).length}`).join(' · ');}
  function deliveryCheck(id){validate();question(id);ready(id);return `${id} 本地审核绑定当前正文且证据字段达标；未授权远端发布`;}

  function next() {
    validate();
    const id=orderedQuestions().find(q=>state.items[q.id].status!=='published')?.id;
    return id ? `${id} [${state.items[id].status}] ${question(id).title}` : '全部已发布';
  }
  function nextWork() {
    validate();
    const id=orderedQuestions().find(q=>!['ready','published'].includes(state.items[q.id].status))?.id;
    return id ? `${id} [${state.items[id].status}] ${question(id).proposal_id} ${question(id).title}` : '全部已完成内容制作';
  }
  function wechatCheck(){validate();const result=checkWechat(root,env);if(!result.ok)fail(result.errors.join('\n'));return JSON.stringify(result,null,2);}
  return {validate,scaffold,transition,hash,render,next,nextWork,orderedQuestions,checkNavigation,coverage,status,deliveryCheck,wechatCheck};
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const [command,...args]=process.argv.slice(2), p=project();
    const flag = name => { const i=args.indexOf(name); return i>=0 ? args[i+1] : undefined; };
    const positional={validate:0,next:0,'next-work':0,render:0,scaffold:1,hash:1,transition:2,coverage:0,status:0,'delivery-check':1,'check-navigation':0,'wechat-check':0};
    if(!(command in positional)) fail('用法：validate | next | next-work | render | scaffold ID | hash ID | transition ID STATUS [--date DATE --url URL] | status | coverage | delivery-check ID | check-navigation | wechat-check');
    const n=positional[command];
    if(args.length<n || args.slice(0,n).some(a=>a.startsWith('--'))) fail('缺少必需参数');
    const allowed=command==='transition' ? ['--date','--url'] : [];
    const seen=new Set();
    for(let i=n;i<args.length;i+=2) {
      if(!allowed.includes(args[i]) || seen.has(args[i]) || !args[i+1] || args[i+1].startsWith('--')) fail('未知、重复或缺失值的参数');
      seen.add(args[i]);
    }
    if(command !== 'transition') p.validate();
    if(command==='transition') { console.log(p.transition(args[0],args[1],{date:flag('--date'),url:flag('--url')})); console.log(p.render()); }
    else if(command==='scaffold') { console.log(p.scaffold(args[0])); console.log(p.render()); }
    else if(command==='next-work') console.log(p.nextWork());
    else if(command==='delivery-check') console.log(p.deliveryCheck(args[0]));
    else if(command==='hash') console.log(p.hash(args[0]));
    else if(command==='wechat-check') console.log(p.wechatCheck());
    else if(command==='check-navigation') console.log(p.checkNavigation());
    else console.log(p[command]());
  } catch(e) { console.error(e.message); process.exitCode=1; }
}
