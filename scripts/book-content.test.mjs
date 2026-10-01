import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { test } from 'node:test';
import { mkdtempSync } from 'node:fs';
import {
  loadManifest,
  renderPage,
  ROOT,
  SITE_BASE,
  validateManifest,
} from './book-content.mjs';

test('current manifest and published pages are valid', () => {
  const manifest = loadManifest();
  const entries = validateManifest(manifest);
  assert.equal(entries.filter((entry) => entry.published).length, 22);
});

test('chapter transform removes duplicate H1 and rewrites repository links', () => {
  const manifest = loadManifest();
  const meta = manifest['ch28-vector-retrieval'];
  const output = renderPage('ch28-vector-retrieval', meta, manifest);
  assert.doesNotMatch(output, /^# 第 28 章/m);
  assert.match(output, new RegExp(`${SITE_BASE}/chapters/introduction/|github.com`));
  assert.match(output, /github\.com\/jerry-zmf-uestc\/the-way-to-ai-engineer\/tree\/main\/labs\//);
  assert.doesNotMatch(output, /\[返回全书目录\]/);
  assert.equal(output, renderPage('ch28-vector-retrieval', meta, manifest));
});

function fixture() {
  const root = mkdtempSync(resolve(tmpdir(), 'book-content-test-'));
  mkdirSync(resolve(root, 'book'), { recursive: true });
  writeFileSync(resolve(root, 'book/a.md'), '# A\n', 'utf8');
  writeFileSync(resolve(root, 'book/b.md'), '# B\n', 'utf8');
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
    () => validateManifest({ a: page('book/a.md', 'A', 1), b: page('book/b.md', 'B', 1) }, root),
    /order 1 is already used/,
  );
});

test('unknown prerequisite and cycles fail closed', () => {
  const root = fixture();
  assert.throws(
    () => validateManifest({ a: page('book/a.md', 'A', 1, ['missing']) }, root),
    /unknown prerequisite/,
  );
  assert.throws(
    () => validateManifest({
      a: page('book/a.md', 'A', 1, ['b']),
      b: page('book/b.md', 'B', 2, ['a']),
    }, root),
    /cycle detected/,
  );
});

test('missing source and mismatched title fail closed', () => {
  const root = fixture();
  assert.throws(
    () => validateManifest({ missing: page('book/missing.md', 'Missing', 1) }, root),
    /source not found/,
  );
  assert.throws(
    () => validateManifest({ a: page('book/a.md', 'Wrong', 1) }, root),
    /does not match source H1/,
  );
});
