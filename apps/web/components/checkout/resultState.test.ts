import { describe, expect, it } from 'vitest';
import type { Order, Ticket } from '@/lib/api/client';
import { advanceAttempt, classify, needsTickets, CONFIRMING_AFTER_MS, currentAttempt, pollDelay, retryKey, ticketsForOrder } from './resultState';

const order = (p: Partial<Order>): Order => ({ orderId: 'o1', orderNo: 'NN1', status: 'pending', paymentStatus: 'none', ticketCount: 2, ...p });
const tk = (id: string, orderId?: string, visitDate = '2026-11-20'): Ticket => ({
  ticketId: id,
  kind: 'day',
  visitDate,
  holderName: 'A',
  idLast4: '1234',
  status: 'active',
  qr: 'NN1.x.y',
  orderId,
});

function memStore() {
  const m = new Map<string, string>();
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) };
}

describe('order result', () => {
  it('a slow webhook is never a failure: keeps polling and switches to "confirming"', () => {
    expect(classify(order({}), [], 3_000)).toEqual({ view: 'checking', done: false });
    expect(classify(order({}), [], CONFIRMING_AFTER_MS + 1)).toEqual({ view: 'confirming', done: false });
    expect(classify(order({}), [], 5 * 60_000)).toEqual({ view: 'confirming', done: false });
    expect(pollDelay(1_000)).toBe(2000);
    expect(pollDelay(30_000)).toBeGreaterThanOrEqual(3000);
    expect(pollDelay(30_000)).toBeLessThanOrEqual(5000);
  });
  it('retry only on an explicit failure; expired stops', () => {
    expect(classify(order({ paymentStatus: 'failed' }), [], 1_000)).toEqual({ view: 'failed', done: true });
    expect(classify(order({ status: 'failed' }), [], 1_000).view).toBe('failed');
    expect(classify(order({ status: 'expired' }), [], 1_000)).toEqual({ view: 'expired', done: true });
  });
  it('paid: success only when all of this order\'s tickets are issued', () => {
    expect(classify(order({ status: 'confirmed' }), [tk('a', 'o1')], 1_000).view).toBe('issuing');
    expect(classify(order({ status: 'confirmed' }), [tk('a', 'o1'), tk('b', 'o1')], 1_000)).toEqual({ view: 'success', done: true });
  });
  it("a 'completed' order loads its tickets and reaches success", () => {
    for (const st of ['confirmed', 'paid', 'completed']) expect(needsTickets(order({ status: st }))).toBe(true);
    expect(needsTickets(order({ paymentStatus: 'paid' }))).toBe(true);
    expect(needsTickets(order({}))).toBe(false);
    expect(classify(order({ status: 'completed' }), [tk('a', 'o1'), tk('b', 'o1')], 1_000)).toEqual({ view: 'success', done: true });
  });
  it('tickets match strictly by orderId (no visitDate fallback)', () => {
    const all = [tk('a', 'o1'), tk('b', 'o2'), tk('c', undefined), tk('d', '')];
    expect(ticketsForOrder(all, 'o1').map((t) => t.ticketId)).toEqual(['a']);
  });
  it('retry reuses a per-order key until a retry payment was actually created', () => {
    const store = memStore();
    const k1 = retryKey('o1', currentAttempt('o1', store));
    expect(k1).toBe('pay-o1-0');
    expect(retryKey('o1', currentAttempt('o1', store))).toBe(k1); // double click / reload: same key
    advanceAttempt('o1', store); // retry payment created, then confirmed failure again
    expect(retryKey('o1', currentAttempt('o1', store))).toBe('pay-o1-1');
    expect(currentAttempt('o2', store)).toBe(0);
  });
});
