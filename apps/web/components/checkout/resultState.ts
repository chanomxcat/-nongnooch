import type { Order, Ticket } from '@/lib/api/client';

export type View = 'checking' | 'confirming' | 'issuing' | 'success' | 'slow' | 'failed' | 'expired' | 'error';

/** After this the page says "still confirming your payment" but keeps polling; it never infers failure from time. */
export const CONFIRMING_AFTER_MS = 12_000;
/** Paid but tickets not all issued after this: tell the user they'll appear in My Tickets (still polling). */
export const ISSUING_SLOW_AFTER_MS = 60_000;
/** Stop polling a still-pending order after this (the 15 min hold will have expired by then anyway). */
export const POLL_LIMIT_MS = 20 * 60_000;

export const isPaid = (o: Order) => ['confirmed', 'paid', 'completed'].includes(o.status) || o.paymentStatus === 'paid';
/** Tickets are fetched for exactly the orders classify() treats as paid (single source of truth). */
export const needsTickets = (o: Order) => isPaid(o);

/** Only an explicit failure from commerce counts; a slow webhook is not a failure. */
export const isFailed = (o: Order) => o.status === 'failed' || o.status === 'payment_failed' || ['failed', 'cancelled'].includes(o.paymentStatus);
export const isExpired = (o: Order) => o.status === 'expired' || o.status === 'cancelled';

/** 2 s for the first 12 s, then back off to 4 s. */
export const pollDelay = (elapsedMs: number) => (elapsedMs < CONFIRMING_AFTER_MS ? 2000 : 4000);

/** Tickets belong to this order only when the ticket says so (ticket service returns orderId). */
export function ticketsForOrder(all: Ticket[], orderId: string): Ticket[] {
  return all.filter((t) => !!t.orderId && t.orderId === orderId);
}

/** View for one poll result; `done` means stop polling. */
export function classify(o: Order, mine: Ticket[], elapsedMs: number): { view: View; done: boolean } {
  if (isPaid(o)) {
    const complete = mine.length > 0 && (!o.ticketCount || mine.length >= o.ticketCount);
    if (complete) return { view: 'success', done: true };
    return { view: elapsedMs > ISSUING_SLOW_AFTER_MS ? 'slow' : 'issuing', done: elapsedMs > POLL_LIMIT_MS };
  }
  if (isFailed(o)) return { view: 'failed', done: true };
  if (isExpired(o)) return { view: 'expired', done: true };
  return { view: elapsedMs < CONFIRMING_AFTER_MS ? 'checking' : 'confirming', done: elapsedMs > POLL_LIMIT_MS };
}

/**
 * Idempotency key for a payment retry on this order: `pay-<orderId>-<attempt>`. The attempt number is
 * only advanced once a retry payment has actually been created (just before redirecting to the bank), so
 * double clicks and reloads of the failed page reuse the same key and cannot create a second charge.
 */
export function retryKey(orderId: string, attempt: number): string {
  return `pay-${orderId}-${attempt}`;
}

type Store = Pick<Storage, 'getItem' | 'setItem'> | null;
const attemptKey = (orderId: string) => `nn_pay_attempt_${orderId}`;

export function currentAttempt(orderId: string, store: Store): number {
  const n = Number(store?.getItem(attemptKey(orderId)) ?? '0');
  return Number.isFinite(n) && n >= 0 ? n : 0;
}

/** Called after a retry payment was created: the next confirmed failure gets a fresh key. */
export function advanceAttempt(orderId: string, store: Store): void {
  try {
    store?.setItem(attemptKey(orderId), String(currentAttempt(orderId, store) + 1));
  } catch {
    /* storage unavailable */
  }
}
