import { describe, expect, it } from 'vitest';
import { formatBaht, timeHHmm } from './format';
import { createDebouncer, mapScan } from './scan';
import { canScan, decodeStaff } from './session/jwt';
import { safeNext } from './safeNext';

const hh = (i?: string) => timeHHmm(i);

describe('mapScan', () => {
  it('ok', () => {
    const v = mapScan({ result: 'ok', holderName: 'Somchai J.', idLast4: '0708', kind: 'day', visitDate: '2026-11-11' }, hh);
    expect(v.tone).toBe('ok');
    expect(v.title).toBe('ผ่าน');
    expect(v.lines).toContain('•••• 0708');
    expect(v.lines).toContain('Somchai J.');
    expect(v.lines.join('|')).toContain('2026-11-11');
  });
  it('used shows Bangkok time', () => {
    const v = mapScan({ result: 'used', usedAt: '2026-11-11T03:05:00Z' }, hh);
    expect(v.tone).toBe('used');
    expect(v.en).toBe('ALREADY USED');
    expect(v.lines[0]).toContain('10:05');
  });
  it('wrong_date', () => {
    const v = mapScan({ result: 'wrong_date', visitDate: '2026-12-01' }, hh);
    expect(v.tone).toBe('warn');
    expect(v.title).toBe('ไม่ใช่วันเข้าชม');
    expect(v.lines[0]).toContain('2026-12-01');
  });
  it.each(['void', 'invalid', 'whatever', undefined])('%s is bad', (r) => {
    const v = mapScan({ result: r }, hh);
    expect(v.tone).toBe('bad');
    expect(v.title).toBe('บัตรไม่ถูกต้อง');
  });
});

describe('formatBaht', () => {
  it('formats satang', () => {
    expect(formatBaht(0)).toBe('฿0');
    expect(formatBaht(90000)).toBe('฿900');
    expect(formatBaht(123456)).toBe('฿1,234.56');
    expect(formatBaht(5)).toBe('฿0.05');
  });
});

describe('debounce', () => {
  it('ignores same qr within 3s only', () => {
    let t = 1000;
    const ok = createDebouncer(3000, () => t);
    expect(ok('a')).toBe(true);
    t += 1000;
    expect(ok('a')).toBe(false);
    expect(ok('b')).toBe(true);
    t += 2500;
    expect(ok('a')).toBe(true);
  });
});

describe('session', () => {
  it('decodes claims and roles', () => {
    const p = Buffer.from(JSON.stringify({ sub: 's1', exp: 9999999999, name: 'Gate', roles: ['gate'] })).toString('base64url');
    const c = decodeStaff(`x.${p}.y`);
    expect(c?.name).toBe('Gate');
    expect(canScan(c?.roles)).toBe(true);
    expect(canScan(['finance'])).toBe(false);
    expect(decodeStaff('garbage')).toBeNull();
  });
  it('safeNext', () => {
    expect(safeNext('/gate')).toBe('/gate');
    expect(safeNext('//evil.com')).toBe('/dashboard');
    expect(safeNext('https://evil.com')).toBe('/dashboard');
    expect(safeNext(null)).toBe('/dashboard');
  });
});
