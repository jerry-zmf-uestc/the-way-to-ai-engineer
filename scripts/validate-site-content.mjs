import { loadManifest, renderPage, ROOT, validateManifest } from './book-content.mjs';

const manifest = loadManifest();
const entries = validateManifest(manifest);
for (const entry of entries.filter((item) => item.published)) {
  const { id, ...meta } = entry;
  const rendered = renderPage(id, meta, manifest);
  if (/\/(?:Users|Volumes|home)\//.test(rendered)) {
    throw new Error(`${id}: generated page exposes a local absolute path`);
  }
  if (/\]\([^)]*src\/content\/chapters/.test(rendered)) {
    throw new Error(`${id}: generated page links to generated content`);
  }
}

console.log(`site content: ${entries.length} page(s) valid under ${ROOT}`);
