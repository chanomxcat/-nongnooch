// TH / EN / ZH must have every key in en.json (with matching {placeholders}); ja/ko/de/fr may fall back to EN.
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const dir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'messages');
const load = (l) => JSON.parse(readFileSync(path.join(dir, `${l}.json`), 'utf8'));
const flat = (o, p = '') =>
  Object.entries(o).flatMap(([k, v]) => (v && typeof v === 'object' ? flat(v, `${p}${k}.`) : [[`${p}${k}`, String(v)]]));
const vars = (s) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',');

const en = new Map(flat(load('en')));
let bad = 0;
for (const l of ['th', 'zh', 'ja', 'ko', 'de', 'fr']) {
  const m = new Map(flat(load(l)));
  for (const [k, v] of m) {
    if (!en.has(k)) { console.log(`${l}: extra ${k}`); bad++; }
    else if (vars(v) !== vars(en.get(k))) { console.log(`${l}: placeholder mismatch ${k}`); bad++; }
  }
  if (['th', 'zh'].includes(l)) for (const k of en.keys()) if (!m.has(k)) { console.log(`${l}: missing ${k}`); bad++; }
}
if (bad) process.exit(1);
console.log('i18n ok');
