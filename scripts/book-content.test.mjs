import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { test } from 'node:test';
import { mkdtempSync } from 'node:fs';
import {
  loadManifest,
  loadRedirects,
  validateRedirects,
  renderPage,
  ROOT,
  SITE_BASE,
  validateManifest,
} from './book-content.mjs';

test('current manifest and published pages are valid', () => {
  const manifest = loadManifest();
  const entries = validateManifest(manifest);
  assert.equal(entries.length, 38);
  assert.equal(entries.filter((entry) => entry.published).length, 38);
  const units = entries.filter((entry) => entry.unitNumber !== undefined);
  assert.deepEqual(units.map((entry) => entry.unitNumber).sort((a,b) => a-b), Array.from({ length: 37 }, (_,i) => i+1));
  assert.deepEqual([1,2,3].map((n) => units.filter((entry) => entry.part.startsWith(`Part ${n} · `)).length), [11,10,16]);
  assert.ok(units.every((entry) => entry.published));
  assert.ok(entries.some((entry) => entry.path === "content/book/part3-agent/runtime/chapter33-serving.md" && entry.published));
});

test('Agent navigation covers the approved three macro sections', () => {
  const manifest = loadManifest();
  const counts = new Map();
  for (const meta of Object.values(manifest).filter((meta) => meta.part === 'Part 3 · Golang Agent' && meta.published)) {
    const key = meta.navigationPath.join('/');
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  assert.deepEqual(Object.fromEntries(counts), {
    '构建篇': 7,
    '运行时': 5,
    '治理篇': 4,
  });
  const expectedGroups = {
    '构建篇': ['harness-intro', 'agent-tool', 'ch23-tool-calling', 'agent-planning', 'agent-context', 'ch28-rag-app', 'agent-skills'],
    '运行时': ['agent-runtime', 'agent-memory', 'agent-delegation', 'agent-async-tasks', 'agent-serving'],
    '治理篇': ['agent-trace-replay', 'agent-eval', 'agent-safety', 'agent-cost'],
  };
  for (const [group, ids] of Object.entries(expectedGroups)) {
    const actual = Object.entries(manifest).filter(([, meta]) => meta.published && meta.navigationPath?.[0] === group).map(([id]) => id);
    assert.deepEqual(actual.sort(), ids.sort(), group);
  }
  assert.match(renderPage('agent-tool', manifest['agent-tool'], manifest), /navigationPath: \["构建篇"\]/);
  for (const navigationPath of [undefined, ['Agent Harness', 'typo'], ['Agent Harness']]) {
    const bad = structuredClone(manifest);
    bad['agent-tool'].navigationPath = navigationPath;
    assert.throws(() => validateManifest(bad), /navigationPath|knowledge category/);
  }
});

test('Agent prerequisites express concepts rather than the full recommended page chain',()=>{
  const manifest=loadManifest();
  const ancestors=id=>{
    const found=new Set();
    const visit=key=>{for(const parent of manifest[key].prerequisites){if(!found.has(parent)){found.add(parent);visit(parent);}}};
    visit(id);return found;
  };
  for(const [id,notNeeded] of [
    ['harness-intro','ch21-service-runtime'],
    ['agent-eval','ch28-rag-app'],
    ['agent-runtime','agent-planning'],
    ['agent-tool','agent-safety'],
    ['agent-context','agent-memory'],
  ])assert.equal(ancestors(id).has(notNeeded),false,`${id} unnecessarily requires ${notNeeded}`);
  assert.ok(ancestors('ch28-rag-app').has('agent-context'));
  assert.ok(ancestors('agent-planning').has('ch23-tool-calling'));
  assert.ok(ancestors('agent-serving').has('agent-async-tasks'));
  validateManifest(manifest);
});

test('chapter transform removes duplicate H1 and rewrites repository links', () => {
  const manifest = loadManifest();
  const meta = manifest['ch28-rag-app'];
  const output = renderPage('ch28-rag-app', meta, manifest);
  assert.doesNotMatch(output, /^# Agent Harness · Retrieval/m);
  assert.match(output, new RegExp(`${SITE_BASE}/chapters/introduction/|github.com`));
  assert.match(output, /github\.com\/jerry-zmf-uestc\/the-way-to-ai-engineer\/blob\/main\/labs\/part3-agent\/chapter28\/hybrid-retrieval\/README\.md/);
  assert.doesNotMatch(output, /\[返回全书目录\]/);
  assert.equal(output, renderPage('ch28-rag-app', meta, manifest));
});

function fixture() {
  const root = mkdtempSync(resolve(tmpdir(), 'book-content-test-'));
  mkdirSync(resolve(root, 'content/book'), { recursive: true });
  writeFileSync(resolve(root, 'content/book/a.md'), '# A\n', 'utf8');
  writeFileSync(resolve(root, 'content/book/b.md'), '# B\n', 'utf8');
  return root;
}

function page(path, title, order, prerequisites = []) {
  return {
    path,
    title,
    part: 'Test',
    order,
    description: 'Test page',
    published: true,
    prerequisites,
    labs: [],
    capabilities: [],
  };
}

test('duplicate order fails closed', () => {
  const root = fixture();
  assert.throws(
    () => validateManifest({ a: page('content/book/a.md', 'A', 1), b: page('content/book/b.md', 'B', 1) }, root),
    /order 1 is already used/,
  );
});

test('unknown prerequisite and cycles fail closed', () => {
  const root = fixture();
  assert.throws(
    () => validateManifest({ a: page('content/book/a.md', 'A', 1, ['missing']) }, root),
    /unknown prerequisite/,
  );
  assert.throws(
    () => validateManifest({
      a: page('content/book/a.md', 'A', 1, ['b']),
      b: page('content/book/b.md', 'B', 2, ['a']),
    }, root),
    /cycle detected/,
  );
});

test('missing source and mismatched title fail closed', () => {
  const root = fixture();
  assert.throws(
    () => validateManifest({ missing: page('content/book/missing.md', 'Missing', 1) }, root),
    /source not found/,
  );
  assert.throws(
    () => validateManifest({ a: page('content/book/a.md', 'Wrong', 1) }, root),
    /does not match source H1/,
  );
});

test('planned sources may be absent but cannot create a published route', () => {
  const root = fixture();
  const planned = { ...page('content/book/planned.md', 'Planned', 1), published: false };
  assert.equal(validateManifest({ planned }, root)[0].published, false);
  assert.throws(() => validateManifest({ planned: { ...planned, published: true } }, root), /source not found/);
  assert.throws(() => validateManifest({ planned: { ...planned, path: 'content/book/../../outside.md' } }, root), /source escapes/);
});

test('unit numbers reject duplicates, invalid bounds, wrong Parts and numbered entrances', () => {
  for (const change of [
    (m) => {m['agent-tool'].unitNumber = 22;},
    (m) => {m['agent-tool'].unitNumber = 48;},
    (m) => {m['agent-tool'].unitNumber = 0;},
    (m) => {m['agent-tool'].part = 'Part 2 · 后端进阶';},
    (m) => {delete m['agent-tool'].unitNumber;},
    (m) => {m['introduction'].unitNumber = 14;},
    (m) => {delete m['harness-intro'].unitNumber;},
  ]) {
    const manifest = structuredClone(loadManifest());
    change(manifest);
    assert.throws(() => validateManifest(manifest), /unitNumber/);
  }
});


test('Agent Loop sample preserves Lab and diagram links under the Pages base', () => {
  const manifest = loadManifest();
  const output = renderPage('ch23-tool-calling', manifest['ch23-tool-calling'], manifest);
  assert.match(output, /配套实验：.*minimal-agent-loop\/README\.md/);
  assert.ok(output.includes(`${SITE_BASE}/diagrams/agent-loop.svg`));
  assert.doesNotMatch(output, /## 动手练|## 踩坑清单|## 链接/);
});


test('repository directory and file links retain distinct GitHub targets', () => {
  const root = fixture();
  mkdirSync(resolve(root, 'labs/topic'), { recursive: true });
  writeFileSync(resolve(root, 'labs/topic/README.md'), '# Lab\n', 'utf8');
  writeFileSync(resolve(root, 'content/book/a.md'), '# A\n\n[Directory](../../labs/topic) [File](../../labs/topic/README.md)\n', 'utf8');
  const manifest = { a: page('content/book/a.md', 'A', 1) };
  const output = renderPage('a', manifest.a, manifest, root);
  assert.match(output, /\[Directory\]\(https:\/\/github\.com\/jerry-zmf-uestc\/the-way-to-ai-engineer\/tree\/main\/labs\/topic\)/);
  assert.match(output, /\[File\]\(https:\/\/github\.com\/jerry-zmf-uestc\/the-way-to-ai-engineer\/blob\/main\/labs\/topic\/README\.md\)/);
});

test('book index links preserve the Pages base without introducing a double slash', () => {
  const root = fixture();
  writeFileSync(resolve(root, 'content/book/a.md'), '# A\n\n[目录](README.md) [路线](README.md#start)\n');
  const manifest = { a: page('content/book/a.md', 'A', 1) };
  const output = renderPage('a', manifest.a, manifest, root);
  assert.ok(output.includes(`[目录](${SITE_BASE}/)`));
  assert.ok(output.includes(`[路线](${SITE_BASE}/#start)`));
  assert.ok(!output.includes(`${SITE_BASE}//`));
});

test('merged pages redirect directly to published topics and reject broken or shadowing targets', () => {
  const manifest = loadManifest();
  const redirects = loadRedirects();
  assert.equal(validateRedirects(redirects, manifest).length, 10);
  assert.deepEqual(redirects['agent-hitl'], { pageId: 'agent-tool', anchor: 'topic-agent-hitl' });
  assert.deepEqual(manifest['agent-tool'].navigationPath, ['构建篇']);
  for (const mutate of [
    (r) => { r['agent-hitl'].pageId = 'absent'; },
    (r, pages) => { pages['ch14-redis'].published = false; r['agent-hitl'].pageId = 'ch14-redis'; },
    (r) => { r['agent-hitl'].pageId = 'agent-mcp'; },
    (r) => { r['agent-hitl'].anchor = 'absent-anchor'; },
    (r) => { r['agent-hitl'].anchor = '../outside'; },
    (r) => { r['agent-tool'] = r['agent-hitl']; },
  ]) {
    const broken = structuredClone(redirects);
    const pages = structuredClone(manifest);
    mutate(broken, pages);
    assert.throws(() => validateRedirects(broken, pages), /redirect/);
  }
});

test('frozen source references have unique declared moves without rewriting datasets', () => {
  const manifest = loadManifest();
  const sources = Object.values(manifest).flatMap((page) => page.legacySources ?? []);
  assert.equal(sources.length, 26);
  assert.equal(new Set(sources).size, 26);
  for (const paths of [
    ['content/book/../../outside.md'],
    ['content/book/part3-agent/chapter24-tool.md', 'content/book/part3-agent/chapter24-tool.md'],
    manifest['agent-context'].legacySources,
  ]) {
    const broken = structuredClone(manifest);
    broken['agent-tool'].legacySources = paths;
    assert.throws(() => validateManifest(broken), /legacySources/);
  }
});
