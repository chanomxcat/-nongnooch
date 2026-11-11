import { describe, expect, it } from 'vitest';
import { formatTHB, todayInBangkok, addDays } from './format';
import { getMessages, mergeMessages, translate } from './i18n';
import { negotiate, htmlLang } from './locale';
import { safeNext } from './safeNext';
import { formatThaiId, isValidThaiId, normalizePassport } from './thaiId';
import { normalizeInvite } from './invite';

describe('thai id', () => {
  it.each([
    ['1-1037-02071-81-1', true],
    ['3100600000005', true],
    ['1101700230708', true], // contracts §7 seed member
    ['1101700230705', false],
    ['1103702071812', false],
    ['110370207181', false],
    ['11037020718a1', false],
  ])('%s → %s', (id, ok) => expect(isValidThaiId(id)).toBe(ok));
  it('formats while typing', () => expect(formatThaiId('1101700230708')).toBe('1-1017-00230-70-8'));
  it('passport normalize', () => {
    expect(normalizePassport(' ab 123456 ')).toBe('AB123456');
    expect(normalizePassport('AB-12')).toBeNull();
  });
});

describe('format', () => {
  it('formatTHB uses satang', () => {
    expect(formatTHB(1999, 'en')).toContain('19.99');
    expect(formatTHB(30000, 'th')).toContain('300');
    expect(formatTHB(30000, 'en')).not.toContain('.00');
  });
  it('todayInBangkok crosses midnight', () => expect(todayInBangkok(new Date('2026-11-11T17:30:00Z'))).toBe('2026-11-12'));
  it('addDays', () => expect(addDays('2026-12-31', 1)).toBe('2027-01-01'));
});

describe('i18n', () => {
  it('messages fall back to en', () => {
    const ja = getMessages('ja');
    expect(translate(ja, 'nav.home')).toBe('ホーム');
    expect(translate(ja, 'auth.loginTitle')).toBe(translate(getMessages('en'), 'auth.loginTitle'));
  });
  it('deep merge keeps siblings', () => {
    expect(mergeMessages({ a: { x: '1', y: '2' } }, { a: { y: '3' } })).toEqual({ a: { x: '1', y: '3' } });
  });
  it('interpolates and exposes missing keys', () => {
    expect(translate({ a: 'Hi {name}' }, 'a', { name: 'Ann' })).toBe('Hi Ann');
    expect(translate({}, 'nope.key')).toBe('nope.key');
  });
  it('th and zh carry the welcome copy', () => {
    expect(translate(getMessages('th'), 'done.points', { points: 150 })).toBe('ได้รับ 150 พ้อย');
    expect(translate(getMessages('zh'), 'nav.buyTickets')).toBe('购买门票');
  });
  it('locale negotiation + zh-Hans', () => {
    expect(negotiate('fr-CA,fr;q=0.9')).toBe('fr');
    expect(negotiate('ru')).toBe('en');
    expect(htmlLang('zh')).toBe('zh-Hans');
  });
});

describe('safety', () => {
  it('safeNext rejects open redirect', () => {
    for (const bad of ['//evil.com', 'https://evil.com', '/\evil', '/en/x', '/thx']) expect(safeNext(bad, 'th')).toBe('/th');
    expect(safeNext('/th/checkout?step=holders', 'th')).toBe('/th/checkout?step=holders');
  });
  it('invite code normalised per contracts §10', () => {
    expect(normalizeInvite(' demo-guide1 ')).toBe('DEMOGUIDE1');
    expect(normalizeInvite('<script>')).toBeNull();
  });
});
