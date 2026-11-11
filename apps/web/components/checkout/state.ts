import type { OrderItemInput, Product } from '@/lib/api/client';

export type Pass = 'day' | 'annual';
export type Segment = 'thai' | 'foreign';
export type AgeGroup = 'adult' | 'child';
export type HolderInput = { kind: 'me' } | { kind: 'other'; name: string; nationality: string; idLast4: string };

export type CheckoutState = {
  pass: Pass;
  segment: Segment;
  visitDate: string | null;
  qty: Record<AgeGroup, number>;
  holders: HolderInput[];
  idempotencyKey: string;
};

export type Ctx = { thaiVerified: boolean };

export const MAX_TICKETS = 20;

export const emptyHolder = (): HolderInput => ({ kind: 'other', name: '', nationality: '', idLast4: '' });

export function findProduct(catalog: Product[], pass: Pass, age: AgeGroup): Product | undefined {
  return catalog.find((p) => p.kind === pass && p.ageGroup === age);
}

/** Adult tickets first, then child: one slot per ticket. */
export function slots(state: CheckoutState): AgeGroup[] {
  return [...Array(state.qty.adult).fill('adult'), ...Array(state.qty.child).fill('child')];
}

export const ticketCount = (s: CheckoutState) => s.qty.adult + s.qty.child;

/** Thai price only for a holder whose Thai ID is verified: in the demo that is the signed-in member ("me"). */
export function audienceFor(state: CheckoutState, holder: HolderInput | undefined, ctx: Ctx): 'thai' | 'foreign' {
  if (state.segment !== 'thai' || !ctx.thaiVerified) return 'foreign';
  return holder?.kind === 'me' ? 'thai' : 'foreign';
}

export function unitPrice(p: Product | undefined, audience: 'thai' | 'foreign'): number {
  if (!p) return 0;
  return (audience === 'thai' ? p.thaiSatang : p.foreignSatang) ?? p.foreignSatang ?? p.thaiSatang ?? 0;
}

export type Line = { age: AgeGroup; audience: 'thai' | 'foreign'; qty: number; unitSatang: number };

/**
 * Price lines. Before holders are named (`perHolder` false) the chosen segment is shown for every ticket,
 * as in the Figma; once holders are known each ticket is priced by its holder.
 */
export function summarize(state: CheckoutState, catalog: Product[], ctx: Ctx, perHolder: boolean): { lines: Line[]; totalSatang: number } {
  const map = new Map<string, Line>();
  slots(state).forEach((age, i) => {
    const audience = perHolder ? audienceFor(state, state.holders[i], ctx) : state.segment;
    const key = `${age}:${audience}`;
    const line = map.get(key) ?? { age, audience, qty: 0, unitSatang: unitPrice(findProduct(catalog, state.pass, age), audience) };
    line.qty += 1;
    map.set(key, line);
  });
  const lines = [...map.values()];
  return { lines, totalSatang: lines.reduce((n, l) => n + l.qty * l.unitSatang, 0) };
}

/** Error key per holder slot (null = ok). */
export function holderErrors(state: CheckoutState): (string | null)[] {
  let meSeen = false;
  return slots(state).map((_, i) => {
    const h = state.holders[i];
    if (!h) return 'profile.required';
    if (h.kind === 'me') {
      if (meSeen) return 'checkout.meDup';
      meSeen = true;
      return null;
    }
    if (!h.name.trim() || !h.nationality) return 'profile.required';
    if (!/^[A-Z0-9]{4}$/.test(h.idLast4)) return 'checkout.holderId';
    return null;
  });
}

export function buildOrderItems(state: CheckoutState, catalog: Product[], ctx: Ctx): OrderItemInput[] {
  return slots(state).map((age, i) => {
    const h = state.holders[i];
    const product = findProduct(catalog, state.pass, age);
    return {
      productId: product?.productId ?? '',
      audience: audienceFor(state, h, ctx),
      holder: h?.kind === 'me' ? { companionId: 'me' as const } : { name: h.name.trim(), nationality: h.nationality, idLast4: h.idLast4 },
    };
  });
}

/** Keep one holder entry per ticket slot; the first slot defaults to "me" for members. */
export function syncHolders(state: CheckoutState, isMember: boolean): HolderInput[] {
  const n = ticketCount(state);
  const out = state.holders.slice(0, n);
  while (out.length < n) out.push(out.length === 0 && isMember ? { kind: 'me' } : emptyHolder());
  return out;
}

const KEY = 'nn_checkout';

/** sessionStorage only holds names, nationality and last-4 digits (never a full ID). */
export function persist(state: CheckoutState) {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(state));
  } catch {
    /* storage unavailable */
  }
}

export function restore(): CheckoutState | null {
  try {
    const raw = sessionStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as CheckoutState) : null;
  } catch {
    return null;
  }
}

export function clearPersisted() {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    /* ignore */
  }
}
