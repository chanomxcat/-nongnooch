import { describe, expect, it } from 'vitest';
import { formatBaht, timeHHmm } from './format';
import { createDebouncer, mapScan } from './scan';
import { canScan, decodeStaff } from './session/jwt';
import { safeNext } from './safeNext';
import { upstreamPath } from './session/bffPath';
import { forwardedFor } from './session/upstream';

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
    const d = createDebouncer(3000, () => t);
    expect(d.accept('a')).toBe(true);
    t += 1000;
    expect(d.accept('a')).toBe(false);
    expect(d.accept('b')).toBe(true);
    t += 3500;
    expect(d.accept('a')).toBe(true);
  });
  it('a qr held in view is one presentation, however long', () => {
    let t = 0;
    const d = createDebouncer(3000, () => t);
    expect(d.accept('a')).toBe(true);
    for (let i = 0; i < 20; i++) {
      t += 500; // 10 s of continuous sightings
      expect(d.accept('a')).toBe(false);
    }
    t += 3000; // left the frame for 3 s
    expect(d.accept('a')).toBe(true);
  });
  it('forget (failed request) and reset (scan next) allow an immediate retry', () => {
    let t = 0;
    const d = createDebouncer(3000, () => t);
    expect(d.accept('a')).toBe(true);
    d.forget('a');
    t += 100;
    expect(d.accept('a')).toBe(true);
    d.reset();
    t += 100;
    expect(d.accept('a')).toBe(true);
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

const hx = (xff?: string) => ({ get: (n: string) => (n.toLowerCase() === 'x-forwarded-for' ? (xff ?? null) : null) });

describe('bff upstream path', () => {
  it('builds /api/<svc>/… and limits services', () => {
    expect(upstreamPath('ticket', ['gate', 'scans'])).toEqual({ ok: true, path: '/api/ticket/gate/scans' });
    expect(upstreamPath('commerce', ['admin', 'stats', 'today'])).toEqual({ ok: true, path: '/api/commerce/admin/stats/today' });
    expect(upstreamPath('identity', ['auth', 'staff', 'login'])).toEqual({ ok: false, status: 404 });
    expect(upstreamPath('notify', ['x'])).toEqual({ ok: false, status: 404 });
    expect(upstreamPath('member', ['InTeRnAl', 'seed'])).toEqual({ ok: false, status: 404 });
  });
  it.each([
    [['..', 'identity']],
    [['.', 'x']],
    [['%2e%2e', 'identity']],
    [['%2E%2e', 'identity']],
    [['%252e%252e', 'identity']],
    [['a%2Fb']],
    [['a%5Cb']],
    [['']],
    [['%E0%A4%A']],
    [[]],
  ])('rejects traversal %j', (segs) => {
    expect(upstreamPath('ticket', segs as string[])).toEqual({ ok: false, status: 400 });
  });
});

describe('X-Forwarded-For', () => {
  it('never forwards the client header without a trusted proxy', () => {
    expect(forwardedFor(hx('6.6.6.6'), false)).toBeNull();
  });
  it('behind a trusted proxy keeps only the right-most valid entry', () => {
    expect(forwardedFor(hx('6.6.6.6, 203.0.113.9'), true)).toBe('203.0.113.9');
    expect(forwardedFor(hx('1.2.3.4, evil<x>'), true)).toBeNull();
    expect(forwardedFor(hx(), true)).toBeNull();
  });
});
