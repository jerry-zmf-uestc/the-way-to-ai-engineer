import path from 'node:path';
import os from 'node:os';
import fs from 'node:fs';
import {fileURLToPath} from 'node:url';

export const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
export const interviewRoot = path.join(repositoryRoot,'content/interview');
export function workspaceRoot(env = process.env) {
  const root = path.resolve(env.INTERVIEW_WORKSPACE || path.join(os.homedir(), '.local', 'share', 'the-way-to-ai-engineer', 'interview'));
  // Resolve existing ancestors so a symlink cannot redirect private output into the checkout.
  let ancestor = root;
  while (!fs.existsSync(ancestor)) ancestor = path.dirname(ancestor);
  const actual = path.join(fs.realpathSync(ancestor), path.relative(ancestor, root));
  const relative = path.relative(fs.realpathSync(repositoryRoot), actual);
  if (!relative || (!relative.startsWith('..' + path.sep) && relative !== '..' && !path.isAbsolute(relative))) {
    throw new Error('INTERVIEW_WORKSPACE 必须位于仓库外');
  }
  return root;
}
export function workspaceFile(relative, env = process.env) {
  const root = workspaceRoot(env), file = path.resolve(root, relative), rel = path.relative(root, file);
  if (!rel || rel === '..' || rel.startsWith('..' + path.sep) || path.isAbsolute(rel)) throw new Error('工作文件必须位于私有工作目录内');
  // Also reject existing child symlinks that escape the selected workspace.
  let ancestor = file;
  while (!fs.existsSync(ancestor)) ancestor = path.dirname(ancestor);
  if (fs.existsSync(root)) {
    const actual = path.join(fs.realpathSync(ancestor), path.relative(ancestor, file));
    const resolved = path.relative(fs.realpathSync(root), actual);
    if (resolved === '..' || resolved.startsWith('..' + path.sep) || path.isAbsolute(resolved)) throw new Error('工作文件不能通过符号链接离开私有工作目录');
  }
  return file;
}

export function publicFile(root, relative) {
  root=fs.realpathSync(root);
  const file=path.resolve(root,relative), rel=path.relative(root,file);
  if(!rel || rel==='..' || rel.startsWith('..'+path.sep) || path.isAbsolute(rel) || rel.split(path.sep).some(p=>['.git','.specs','.tools'].includes(p))) throw new Error('正文路径或公开关联越界');
  let ancestor=file;
  while(!fs.existsSync(ancestor)) ancestor=path.dirname(ancestor);
  const actual=path.join(fs.realpathSync(ancestor),path.relative(ancestor,file)), realrel=path.relative(root,actual);
  if(realrel==='..'||realrel.startsWith('..'+path.sep)||path.isAbsolute(realrel)||realrel.split(path.sep).some(p=>['.git','.specs','.tools'].includes(p)))throw new Error('公开路径通过符号链接越界');
  return file;
}
