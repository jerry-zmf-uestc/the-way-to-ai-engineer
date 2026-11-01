import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { test } from 'node:test';
import { loadManifest, loadRedirects, ROOT, SITE_BASE, validateManifest } from '../scripts/book-content.mjs';

function escapeHtml(value) {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function decodeHtml(value) {
  return value.replace(/&quot;/g, '"').replace(/&gt;/g, '>').replace(/&lt;/g, '<').replace(/&amp;/g, '&');
}

function regexEscape(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

test('built course navigation preserves every published URL and identifies each active page', () => {
  const entries = validateManifest(loadManifest()).filter((page) => page.published);
  const expected = entries.map((page) => `${SITE_BASE}/chapters/${page.id}/`).sort();
  for (const page of entries) {
    const html = readFileSync(resolve(ROOT, 'dist/chapters', page.id, 'index.html'), 'utf8');
    const nav = html.match(/<nav aria-label="课程目录">([\s\S]*?)<\/nav>/)?.[1];
    assert.ok(nav, `${page.id}: missing navigation`);
    const links = [...nav.matchAll(/<a\b[^>]*href="([^"]+)"[^>]*>/g)].map((match) => match[1]);
    assert.deepEqual(links.filter((link) => link !== `${SITE_BASE}/`).sort(), expected, page.id);
    const current = [...nav.matchAll(/<a\b[^>]*aria-current="page"[^>]*>/g)];
    assert.equal(current.length, 1, page.id);
    assert.ok(current[0][0].includes(`href="${SITE_BASE}/chapters/${page.id}/"`), page.id);
    for (const label of [page.part, ...(page.navigationPath ?? [])]) {
      assert.match(nav, new RegExp(`<details open>\\s*<summary>${regexEscape(escapeHtml(label))}</summary>`), `${page.id}: closed ancestor ${label}`);
    }
    if (page.navigationPath?.length === 1) {
      assert.ok(nav.includes(`第 ${page.unitNumber} 单元 · `), page.id);
      assert.ok(html.includes(`<span class="unit-number">第 ${page.unitNumber} 单元</span>`), page.id);
    }
  }
  for (const page of validateManifest(loadManifest()).filter((page) => !page.published)) {
    assert.equal(existsSync(resolve(ROOT, 'dist/chapters', page.id)), false, `${page.id}: planned route must not exist`);
  }
});

test('homepage groups contain each published chapter once with matching Agent macro membership', () => {
  const entries = validateManifest(loadManifest()).filter((page) => page.published);
  const html = readFileSync(resolve(ROOT, 'dist/index.html'), 'utf8');
  const cardLinks = (source) => [...source.matchAll(/<a class="card-link" href="([^"]+)"/g)].map((match) => match[1]).sort();
  assert.deepEqual(cardLinks(html), entries.map((page) => `${SITE_BASE}/chapters/${page.id}/`).sort());
  const groups = [...html.matchAll(/<section class="course-family"[^>]*data-course-group="([^"]+)"[^>]*>([\s\S]*?)<\/section>/g)];
  assert.equal(groups.length, 3);
  for (const [, encodedLabel, body] of groups) {
    const label = decodeHtml(encodedLabel);
    const expected = entries.filter((page) => page.navigationPath?.[0] === label);
    assert.deepEqual(cardLinks(body), expected.map((page) => `${SITE_BASE}/chapters/${page.id}/`).sort(), label);
    assert.equal([...body.matchAll(/<h4 class="card-title">/g)].length, expected.length, label);
  }
  for (const label of ['构建篇', '运行时', '治理篇']) {
    assert.match(html, new RegExp(`<h3>${regexEscape(escapeHtml(label))}<\\/h3>`));
  }
});

test('all legacy Agent URLs and section fragments remain reachable after consolidation', () => {
  const legacy = JSON.parse(readFileSync(resolve(ROOT, 'tests/fixtures/agent-legacy-links.json'), 'utf8'));
  const redirects = loadRedirects();
  assert.equal(Object.keys(legacy).length, 26);
  for (const [oldId, { pageId, fragments }] of Object.entries(legacy)) {
    const target = readFileSync(resolve(ROOT, 'dist/chapters', pageId, 'index.html'), 'utf8');
    const ids = [...target.matchAll(/\bid="([^"]+)"/g)].map((m) => m[1]);
    assert.equal(ids.length, new Set(ids).size, `${pageId}: duplicate DOM IDs`);
    for (const fragment of fragments) assert.ok(ids.includes(fragment), `${oldId}: missing legacy fragment #${fragment}`);
    if (oldId === pageId) continue;
    const html = readFileSync(resolve(ROOT, 'dist/chapters', oldId, 'index.html'), 'utf8');
    const expected = `${SITE_BASE}/chapters/${pageId}/#${redirects[oldId].anchor}`;
    assert.ok(html.includes(`href="${expected}"`), `${oldId}: missing direct fallback`);
    assert.ok(html.includes('window.location.hash'), `${oldId}: incoming fragment must be preserved`);
    assert.ok(html.includes('window.location.replace'), `${oldId}: redirect must not add a history loop`);
    assert.ok(html.includes('name="robots" content="noindex"'), `${oldId}: alias is not a teaching page`);
    assert.ok(ids.includes(redirects[oldId].anchor), `${oldId}: missing default topic anchor`);
  }
});
