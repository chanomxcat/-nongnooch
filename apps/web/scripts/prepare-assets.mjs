// Copies the shared icon sprite and the CJK font subsets (self-hosted, no Google Fonts) into public/.
import { cpSync, mkdirSync, readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const pub = path.join(here, '..', 'public');
const require = createRequire(import.meta.url);

cpSync(path.join(here, '../../../packages/tokens/generated/icons.svg'), path.join(pub, 'icons.svg'));

const fonts = { sc: '@fontsource/noto-sans-sc', jp: '@fontsource/noto-sans-jp', kr: '@fontsource/noto-sans-kr' };
for (const [short, pkg] of Object.entries(fonts)) {
  const dir = path.dirname(require.resolve(`${pkg}/package.json`));
  const dest = path.join(pub, 'fonts', short);
  mkdirSync(path.join(dest, 'files'), { recursive: true });
  let css = '';
  for (const w of ['400', '700']) {
    // keep only woff2 sources; browsers we target all support it
    css += readFileSync(path.join(dir, `${w}.css`), 'utf8').replace(/,\s*url\([^)]*\.woff\) format\('woff'\)/g, '');
    for (const f of readdirSync(path.join(dir, 'files'))) {
      if (f.endsWith(`-${w}-normal.woff2`)) cpSync(path.join(dir, 'files', f), path.join(dest, 'files', f));
    }
  }
  writeFileSync(path.join(dest, 'font.css'), css);
}
console.log('assets ready');
