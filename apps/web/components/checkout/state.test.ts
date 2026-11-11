import { describe, expect, it } from 'vitest';
import type { Product } from '@/lib/api/client';
import { mmss, remainingMs, serverOffset } from './clock';
import { buildOrderItems, holderErrors, summarize, syncHolders, type CheckoutState } from './state';

const catalog: Product[] = [
  { productId: 'day-a', kind: 'day', ageGroup: 'adult', name: 'Adult', thaiSatang: 30000, foreignSatang: 60000 },
  { productId: 'day-c', kind: 'day', ageGroup: 'child', name: 'Child', thaiSatang: 15000, foreignSatang: 30000 },
];
const base: CheckoutState = { pass: 'day', segment: 'thai', visitDate: '2026-11-20', qty: { adult: 2, child: 0 }, holders: [], idempotencyKey: 'k' };

describe('checkout state', () => {
  it('thai price per holder: only the verified member gets it', () => {
    const st = { ...base, holders: [{ kind: 'me' as const }, { kind: 'other' as const, name: 'Li Wei', nationality: 'CN', idLast4: '5678' }] };
    expect(summarize(st, catalog, { thaiVerified: true }, true).totalSatang).toBe(30000 + 60000);
    expect(summarize(st, catalog, { thaiVerified: false }, true).totalSatang).toBe(120000);
  });
  it('order request shape', () => {
    const st = { ...base, holders: [{ kind: 'me' as const }, { kind: 'other' as const, name: ' Li Wei ', nationality: 'CN', idLast4: '5678' }] };
    expect(buildOrderItems(st, catalog, { thaiVerified: true })).toEqual([
      { productId: 'day-a', audience: 'thai', holder: { companionId: 'me' } },
      { productId: 'day-a', audience: 'foreign', holder: { name: 'Li Wei', nationality: 'CN', idLast4: '5678' } },
    ]);
  });
  it('duplicate me and missing fields rejected', () => {
    const st = { ...base, holders: [{ kind: 'me' as const }, { kind: 'me' as const }] };
    expect(holderErrors(st)).toEqual([null, 'checkout.meDup']);
    expect(holderErrors({ ...base, holders: syncHolders(base, false) })).toEqual(['profile.required', 'profile.required']);
  });
  it('first holder defaults to me for members', () => {
    expect(syncHolders(base, true)[0]).toEqual({ kind: 'me' });
  });
  it('timer uses server clock', () => {
    const off = serverOffset('Wed, 11 Nov 2026 10:00:00 GMT', Date.parse('2026-11-11T10:10:00Z'));
    expect(mmss(remainingMs('2026-11-11T10:15:00Z', off, Date.parse('2026-11-11T10:10:00Z')))).toBe('15:00');
  });
});
