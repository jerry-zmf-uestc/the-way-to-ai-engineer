import {
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  renameSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { dirname, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
export const ROOT = resolve(SCRIPT_DIR, '..');
export const MANIFEST_PATH = resolve(ROOT, 'content/book/chapters.json');
export const REDIRECTS_PATH = resolve(ROOT, 'content/book/redirects.json');
export const OUTPUT_DIR = resolve(ROOT, '.generated/chapters');
export const SITE_BASE = '/the-way-to-ai-engineer';
export const REPOSITORY_URL = 'https://github.com/jerry-zmf-uestc/the-way-to-ai-engineer';

const requiredStringFields = ['path', 'title', 'part', 'description'];
export const AGENT_NAVIGATION_PATHS = [
  ['构建篇'],
  ['运行时'],
  ['治理篇'],
];

export function loadManifest(path = MANIFEST_PATH) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

export function loadRedirects(path = REDIRECTS_PATH) {
  return existsSync(path) ? JSON.parse(readFileSync(path, 'utf8')) : {};
}

export function validateRedirects(redirects, manifest, rootDir = ROOT) {
  if (!redirects || Array.isArray(redirects) || typeof redirects !== 'object') {
    throw new Error('content/book/redirects.json must contain an object keyed by retired page ID');
  }
  for (const [id, target] of Object.entries(redirects)) {
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(id) || manifest[id]) {
      throw new Error(`${id}: invalid redirect ID or collision with a teaching page`);
    }
    const page = manifest[target?.pageId];
    if (!page?.published || typeof target.anchor !== 'string' || !/^[a-z0-9-]+$/.test(target.anchor)) {
      throw new Error(`${id}: redirect must target a published page and a stable topic anchor`);
    }
    const body = readFileSync(resolve(rootDir, page.path), 'utf8');
    if (!body.includes(`<a id="${target.anchor}"></a>`)) {
      throw new Error(`${id}: redirect target anchor not found`);
    }
  }
  return Object.entries(redirects).map(([id, target]) => ({ id, ...target }));
}

function repoPath(rootDir, absolutePath) {
  return relative(rootDir, absolutePath).split(sep).join('/');
}

function encodeRepoPath(path) {
  return path.split('/').map(encodeURIComponent).join('/');
}

export function validateManifest(manifest, rootDir = ROOT) {
  if (!manifest || Array.isArray(manifest) || typeof manifest !== 'object') {
    throw new Error('content/book/chapters.json must contain an object keyed by stable page ID');
  }

  const entries = Object.entries(manifest);
  if (entries.length === 0) throw new Error('content/book/chapters.json must not be empty');

  const orders = new Map();
  const sourcePaths = new Map();
  const legacyPaths = new Set();
  const unitNumbers = new Map();
  for (const [id, meta] of entries) {
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(id)) {
      throw new Error(`${id}: ID must use lowercase kebab-case`);
    }
    if (!meta || Array.isArray(meta) || typeof meta !== 'object') {
      throw new Error(`${id}: metadata must be an object`);
    }
    for (const field of requiredStringFields) {
      if (typeof meta[field] !== 'string' || meta[field].trim() === '') {
        throw new Error(`${id}: ${field} must be a non-empty string`);
      }
    }
    if (!meta.path.startsWith('content/book/') || !meta.path.endsWith('.md')) {
      throw new Error(`${id}: path must point to a Markdown source under content/book/`);
    }
    if (!Number.isInteger(meta.order) || meta.order < 0) {
      throw new Error(`${id}: order must be a non-negative integer`);
    }
    if (orders.has(meta.order)) {
      throw new Error(`${id}: order ${meta.order} is already used by ${orders.get(meta.order)}`);
    }
    orders.set(meta.order, id);
    if (sourcePaths.has(meta.path)) {
      throw new Error(`${id}: source path is already used by ${sourcePaths.get(meta.path)}`);
    }
    sourcePaths.set(meta.path, id);
    if (meta.legacySources !== undefined) {
      if (!Array.isArray(meta.legacySources) || new Set(meta.legacySources).size !== meta.legacySources.length || meta.legacySources.some((path) =>
        typeof path !== 'string' || !/^book\/part3-agent\/chapter\d+-[a-z0-9-]+\.md$/.test(path) || legacyPaths.has(path))) {
        throw new Error(`${id}: invalid or duplicate legacySources`);
      }
      for (const path of meta.legacySources) legacyPaths.add(path);
    }
    if (typeof meta.published !== 'boolean') {
      throw new Error(`${id}: published must be a boolean`);
    }
    if (meta.unitNumber !== undefined) {
      if (!Number.isInteger(meta.unitNumber) || meta.unitNumber < 1 || meta.unitNumber > 37 || unitNumbers.has(meta.unitNumber)) {
        throw new Error(`${id}: invalid or duplicate unitNumber`);
      }
      if (id === 'introduction') {
        throw new Error(`${id}: unnumbered intro pages do not have a unitNumber`);
      }
      const partNumber = meta.unitNumber <= 11 ? 1 : meta.unitNumber <= 21 ? 2 : 3;
      if (!meta.part.startsWith(`Part ${partNumber} · `)) {
        throw new Error(`${id}: unitNumber does not match Part`);
      }
      unitNumbers.set(meta.unitNumber, id);
    } else if (/^Part [123] · /.test(meta.part)) {
      throw new Error(`${id}: teaching page requires unitNumber`);
    }
    if (meta.part === 'Part 3 · Golang Agent') {
      if (!AGENT_NAVIGATION_PATHS.some((path) => JSON.stringify(path) === JSON.stringify(meta.navigationPath))) {
        throw new Error(`${id}: missing or unknown Agent navigationPath`);
      }
      if (meta.navigationPath.length !== 1) {
        throw new Error(`${id}: Agent navigationPath must use one approved knowledge category`);
      }
    } else if (meta.navigationPath !== undefined) {
      throw new Error(`${id}: navigationPath is reserved for Part 3`);
    }

    const source = resolve(rootDir, meta.path);
    if (!source.startsWith(`${resolve(rootDir, 'content/book')}${sep}`)) {
      throw new Error(`${id}: source escapes book directory`);
    }
    if (!existsSync(source)) {
      if (meta.published) throw new Error(`${id}: source not found: ${meta.path}`);
    } else {
      const firstHeading = readFileSync(source, 'utf8').match(/^#\s+(.+)$/m)?.[1]?.trim();
      if (firstHeading !== meta.title) {
        throw new Error(`${id}: manifest title does not match source H1`);
      }
    }

    for (const field of ['prerequisites', 'labs', 'capabilities']) {
      if (!Array.isArray(meta[field]) || meta[field].some((item) => typeof item !== 'string')) {
        throw new Error(`${id}: ${field} must be an array of strings`);
      }
    }
    for (const lab of meta.labs) {
      if (!lab.startsWith('labs/') || !existsSync(resolve(rootDir, lab))) {
        throw new Error(`${id}: Lab path not found: ${lab}`);
      }
    }
  }

  for (const [id, meta] of entries) {
    for (const prerequisite of meta.prerequisites) {
      if (!manifest[prerequisite]) {
        throw new Error(`${id}: unknown prerequisite ${prerequisite}`);
      }
    }
  }

  const visiting = new Set();
  const visited = new Set();
  function visit(id) {
    if (visiting.has(id)) throw new Error(`prerequisite cycle detected at ${id}`);
    if (visited.has(id)) return;
    visiting.add(id);
    for (const dependency of manifest[id].prerequisites) visit(dependency);
    visiting.delete(id);
    visited.add(id);
  }
  for (const [id] of entries) visit(id);

  return entries
    .map(([id, meta]) => ({ id, ...meta }))
    .sort((left, right) => left.order - right.order);
}

function splitTarget(target) {
  const hashIndex = target.indexOf('#');
  if (hashIndex === -1) return [target, ''];
  return [target.slice(0, hashIndex), target.slice(hashIndex)];
}

function isExternal(target) {
  return /^(?:[a-z][a-z0-9+.-]*:|#|\/\/)/i.test(target);
}

function withBase(path) {
  return `${SITE_BASE}/${path.replace(/^\/+/, '')}`.replace(/\/{2,}/g, '/');
}

function rewriteTarget(target, sourcePath, manifest, rootDir, image = false) {
  if (isExternal(target)) return target;
  const [pathPart, fragment] = splitTarget(target);
  if (pathPart === '') return target;

  if (pathPart.startsWith('/')) return `${withBase(pathPart)}${fragment}`;

  const sourceAbsolute = resolve(rootDir, sourcePath);
  const targetAbsolute = resolve(dirname(sourceAbsolute), decodeURI(pathPart));
  const targetRepoPath = repoPath(rootDir, targetAbsolute);
  const page = Object.entries(manifest).find(([, meta]) => meta.path === targetRepoPath || meta.legacySources?.includes(targetRepoPath));
  if (page && page[1].published) {
    return `${withBase(`chapters/${page[0]}/`)}${fragment}`;
  }
  if (targetRepoPath === 'content/book/README.md') return `${withBase('')}${fragment}`;

  const publicIndex = targetRepoPath.indexOf('public/');
  if (publicIndex >= 0) {
    return `${withBase(targetRepoPath.slice(publicIndex + 'public/'.length))}${fragment}`;
  }

  if (!existsSync(targetAbsolute)) {
    throw new Error(`${sourcePath}: unresolved ${image ? 'image' : 'link'} target ${target}`);
  }
  const view = statSync(targetAbsolute).isDirectory() ? 'tree' : 'blob';
  return `${REPOSITORY_URL}/${view}/main/${encodeRepoPath(targetRepoPath)}${fragment}`;
}

function rewriteMarkdownLinks(body, sourcePath, manifest, rootDir) {
  body = body.replace(/!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g, (_match, alt, target) => {
    return `![${alt}](${rewriteTarget(target, sourcePath, manifest, rootDir, true)})`;
  });
  return body.replace(/(?<!!)\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g, (_match, label, target) => {
    return `[${label}](${rewriteTarget(target, sourcePath, manifest, rootDir, false)})`;
  });
}

function stripSourceOnlyNavigation(body) {
  const lines = body.trimEnd().split('\n');
  const last = lines.at(-1)?.trim() ?? '';
  if (/^\[(?:返回|开始)[^\]]*\]\([^)]+\)(?:\s*·\s*\[[^\]]+\]\([^)]+\))*$/.test(last)) {
    lines.pop();
  }
  return `${lines.join('\n').trimEnd()}\n`;
}

function frontmatter(id, meta) {
  const fields = {
    title: meta.title,
    chapter: meta.chapter,
    part: meta.part,
    unitNumber: meta.unitNumber,
    navigationPath: meta.navigationPath,
    order: meta.order,
    description: meta.description,
    published: meta.published,
    source: meta.path,
    labs: meta.labs,
    capabilities: meta.capabilities,
  };
  const lines = ['---', `pageId: ${JSON.stringify(id)}`];
  for (const [key, value] of Object.entries(fields)) {
    if (value !== undefined) lines.push(`${key}: ${JSON.stringify(value)}`);
  }
  lines.push('---', '');
  return lines.join('\n');
}

export function renderPage(id, meta, manifest, rootDir = ROOT) {
  let body = readFileSync(resolve(rootDir, meta.path), 'utf8');
  body = body.replace(/^#\s+.+\r?\n+/, '');
  body = stripSourceOnlyNavigation(body);
  // 代码围栏内的内容不是 Markdown 链接，必须跳过（如 Go 泛型调用 F[T](1)）。
  const segments = body.split(/(^```[^\n]*\n[\s\S]*?^```\s*$)/m);
  body = segments
    .map((segment, index) =>
      index % 2 === 1 ? segment : rewriteMarkdownLinks(segment, meta.path, manifest, rootDir),
    )
    .join('');
  return `${frontmatter(id, meta)}${body.trimStart()}`;
}

export function prepareBook({ rootDir = ROOT, outputDir = OUTPUT_DIR } = {}) {
  const manifest = loadManifest(resolve(rootDir, 'content/book/chapters.json'));
  const entries = validateManifest(manifest, rootDir);
  validateRedirects(loadRedirects(resolve(rootDir, 'content/book/redirects.json')), manifest, rootDir);
  // Render before touching the last valid generation; partial editor saves can fail.
  const desired = new Map();
  for (const entry of entries) {
    if (!entry.published) continue;
    const { id, ...meta } = entry;
    desired.set(`${id}.md`, renderPage(id, meta, manifest, rootDir));
  }
  mkdirSync(outputDir, { recursive: true });

  const generated = [];
  for (const [name, body] of desired) {
    const outputPath = resolve(outputDir, name);
    if (!existsSync(outputPath) || readFileSync(outputPath, 'utf8') !== body) {
      const temporary = `${outputPath}.${process.pid}.tmp`;
      try { writeFileSync(temporary, body, 'utf8'); renameSync(temporary, outputPath); }
      finally { rmSync(temporary, { force: true }); }
    }
    generated.push(repoPath(rootDir, outputPath));
  }
  for (const name of readdirSync(outputDir)) {
    if (name.endsWith('.md') && !desired.has(name)) rmSync(resolve(outputDir, name));
  }
  return generated;
}
