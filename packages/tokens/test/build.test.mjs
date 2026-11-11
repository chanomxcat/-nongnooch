import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildCss, buildSprite } from '../build.mjs';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const tokens = JSON.parse(readFileSync(join(root, 'tokens.json'), 'utf8'));

function lum(hex) {
  const n = hex.replace('#', '');
  const c = [0, 2, 4].map((i) => parseInt(n.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}
const contrast = (a, b) => {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
  return (x + 0.05) / (y + 0.05);
};

describe('tokens', () => {
  it('css maps figma names', () => {
    const css = buildCss(tokens);
    expect(css).toContain('--color-brand-primary: #074536;');
    expect(css).toContain('--color-brand-secondary: #D9A52E;');
    expect(css).toContain('--spacing-40: 40px;');
    expect(css).toContain('--radius-xlarge: 24px;');
    expect(css).toContain('--elevation-4: 0 6px 18.6px #071C1652;');
  });
  it('gold button text contrast', () => {
    expect(contrast('#074536', '#D9A52E')).toBeGreaterThanOrEqual(4.5);
    expect(contrast('#FFFFFF', '#D9A52E')).toBeLessThan(4.5);
  });
  it('body text contrast', () => {
    for (const fg of ['#2A2E37', '#5A606D']) for (const bg of ['#FFFFFF', '#F6F7F8']) expect(contrast(fg, bg)).toBeGreaterThanOrEqual(4.5);
    expect(contrast('#E9CA8E', '#00271C')).toBeGreaterThanOrEqual(4.5);
  });
  it('icons are themeable', () => {
    const { sprite, names } = buildSprite(join(root, 'icons'));
    expect(names).toEqual(expect.arrayContaining(['ticket', 'clock', 'globe', 'plus', 'minus', 'check', 'qr', 'close', 'menu']));
    expect(sprite).not.toMatch(/(fill|stroke)="#/);
    expect(sprite.match(/<symbol /g)?.length).toBe(names.length);
  });
});
