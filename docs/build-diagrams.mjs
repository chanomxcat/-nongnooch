// Render diagrams/*.mmd -> diagrams/*.svg (with Lucide icons),
// then sync the mermaid blocks in system-design.md from the .mmd sources.
// .mmd files are the single source of truth: edit them, then run `npm run diagrams`.
import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

const dir = 'diagrams';
const ids = readdirSync(dir).filter((f) => f.endsWith('.mmd')).map((f) => f.slice(0, -4));
const mmdc = 'node_modules/@mermaid-js/mermaid-cli/src/cli.js';

for (const id of ids) {
  execFileSync(process.execPath, [mmdc, '-i', `${dir}/${id}.mmd`, '-o', `${dir}/${id}.svg`, '-b', 'white',
    '--iconPacks', '@iconify-json/lucide'], { stdio: ['ignore', 'ignore', 'inherit'] });
  console.log(`rendered ${id}`);
}

const doc = 'system-design.md';
const seen = new Set();
const out = readFileSync(doc, 'utf8').replace(
  /<!-- diagram:(\w+) -->[\s\S]*?<!-- \/diagram -->/g,
  (_, id) => {
    if (!ids.includes(id)) throw new Error(`${doc}: no ${dir}/${id}.mmd`);
    seen.add(id);
    const src = readFileSync(`${dir}/${id}.mmd`, 'utf8').trim();
    return `<!-- diagram:${id} -->\n![${id}](${dir}/${id}.svg)\n\n<details><summary>Mermaid source</summary>\n\n\`\`\`mermaid\n${src}\n\`\`\`\n\n</details>\n<!-- /diagram -->`;
  },
);
writeFileSync(doc, out);
const missing = ids.filter((id) => !seen.has(id));
console.log(`synced ${seen.size} blocks into ${doc}${missing.length ? `; NOT referenced: ${missing.join(', ')}` : ''}`);
