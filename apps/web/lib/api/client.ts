/**
 * Every browser → backend call lives here (through the BFF at /bff/*), so a backend shape
 * change is a one-file fix. Responses are normalized defensively because member / commerce /
 * ticket were built in parallel with this site; see task-8-report.md for assumed vs verified shapes.
 */

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public retryAfter?: number,
  ) {
    super(code);
  }
}

type Json = Record<string, unknown>;

async function call<T>(path: string, init: RequestInit & { json?: unknown } = {}): Promise<{ data: T; date: string | null }> {
  const headers = new Headers(init.headers);
  headers.set('X-Requested-With', 'nn-web');
  let body = init.body;
  if (init.json !== undefined) {
    headers.set('Content-Type', 'application/json');
    body = JSON.stringify(init.json);
  }
  let r: Response;
  try {
    r = await fetch(path, { ...init, headers, body, credentials: 'same-origin', cache: 'no-store' });
  } catch {
    throw new ApiError(0, 'network');
  }
  const text = await r.text();
  let data: unknown = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }
  if (!r.ok) {
    const code = (data as Json | null)?.error;
    const ra = Number(r.headers.get('retry-after'));
    throw new ApiError(r.status, typeof code === 'string' ? code : r.status === 401 ? 'invalid_token' : 'unknown', ra > 0 ? ra : undefined);
  }
  return { data: data as T, date: r.headers.get('date') };
}

const str = (v: unknown, d = ''): string => (typeof v === 'string' ? v : typeof v === 'number' ? String(v) : d);
const num = (v: unknown): number | undefined => (typeof v === 'number' && Number.isFinite(v) ? v : undefined);
const items = (d: unknown): Json[] => {
  if (Array.isArray(d)) return d as Json[];
  const it = (d as Json | null)?.items;
  return Array.isArray(it) ? (it as Json[]) : [];
};

/* ---------------- identity (via /bff/auth) ---------------- */

export type Session = { loggedIn: boolean; memberId: string | null };

export const auth = {
  session: () => call<Session>('/bff/auth/session').then((r) => r.data),
  requestOtp: (b: { phone?: string; email?: string; lang: string; turnstileToken: string }) =>
    call<{ requestId: string }>('/bff/auth/otp', { method: 'POST', json: b }).then((r) => r.data),
  verifyOtp: (requestId: string, otp: string) =>
    call<{ memberId: string | null }>('/bff/auth/verify', { method: 'POST', json: { requestId, otp } }).then((r) => r.data),
  refresh: () => call<{ memberId: string | null }>('/bff/auth/refresh', { method: 'POST', json: {} }).then((r) => r.data),
  logout: () => call<null>('/bff/auth/logout', { method: 'POST', json: {} }),
};

/* ---------------- member ---------------- */

export type RegisterInput = {
  firstName: string;
  lastName: string;
  nationality: string; // ISO 3166-1 alpha-2
  idNumber: string;
  passportCountry?: string;
  inviteCode?: string;
  marketing: boolean;
  lang: string;
};

export type Me = { memberId: string; name: string; nationality: string; thaiVerified: boolean; idLast4: string };
export type PointEntry = { id: string; deltaPoints: number; reason: string; status: string; createdAt: string };
export type Points = { balance: number; pending: number; items: PointEntry[] };

export const member = {
  register: (i: RegisterInput) =>
    call<Json>('/bff/member/members', {
      method: 'POST',
      json: {
        firstName: i.firstName,
        lastName: i.lastName,
        name: `${i.firstName} ${i.lastName}`.trim(),
        nationality: i.nationality,
        idType: i.nationality === 'TH' ? 'thai_id' : 'passport',
        idNumber: i.idNumber,
        passportCountry: i.nationality === 'TH' ? undefined : i.passportCountry || i.nationality,
        inviteCode: i.inviteCode || undefined,
        consents: { terms: true, pdpa: true, marketing: i.marketing },
        lang: i.lang,
        channel: 'web',
      },
    }).then((r) => r.data),

  me: async (): Promise<Me> => {
    const { data } = await call<Json>('/bff/member/members/me');
    const m = ((data?.member as Json) ?? data) as Json;
    const name = str(m.name) || [str(m.firstName), str(m.lastName)].filter(Boolean).join(' ');
    return {
      memberId: str(m.memberId),
      name,
      nationality: str(m.nationality),
      thaiVerified: m.thaiVerified === true,
      idLast4: str(m.idLast4),
    };
  },

  points: async (): Promise<Points> => {
    const { data } = await call<Json>('/bff/member/members/me/points');
    return {
      balance: num(data?.available) ?? num(data?.balance) ?? num(data?.availablePoints) ?? 0,
      pending: num(data?.pending) ?? num(data?.pendingPoints) ?? 0,
      items: items(data).map((e, i) => ({
        id: str(e.id ?? e.entryId, String(i)),
        deltaPoints: num(e.deltaPoints) ?? 0,
        reason: str(e.reason),
        status: str(e.status, 'available'),
        createdAt: str(e.createdAt),
      })),
    };
  },
};

/* ---------------- commerce ---------------- */

export type Product = {
  productId: string;
  kind: 'day' | 'annual' | string;
  ageGroup: 'adult' | 'child' | string;
  name: string;
  thaiSatang?: number;
  foreignSatang?: number;
};
export type Availability = Record<string, number>;
export type Holder = { companionId: 'me' } | { name: string; nationality: string; idLast4: string };
export type OrderItemInput = { productId: string; audience: 'thai' | 'foreign'; holder: Holder };
export type Hold = { orderId: string; orderNo: string; totalSatang: number; holdExpiresAt: string; serverDate: string | null };
export type Order = {
  orderId: string;
  orderNo: string;
  status: string; // pending | confirmed | paid | expired | cancelled
  paymentStatus: string; // none | pending | paid | failed | cancelled
  totalSatang?: number;
  visitDate?: string;
  holdExpiresAt?: string;
  ticketCount?: number;
};

export const commerce = {
  products: async (visitDate: string, lang: string): Promise<Product[]> => {
    const { data } = await call<Json>(`/bff/commerce/products?visitDate=${visitDate}&lang=${lang}`);
    return items(data).map((p) => ({
      productId: str(p.productId ?? p.id),
      kind: str(p.kind, 'day'),
      ageGroup: str(p.ageGroup ?? p.audience ?? p.age, 'adult'),
      name: str(p.name),
      thaiSatang: num(p.thaiSatang) ?? num(p.thaiPriceSatang) ?? num(p.priceThaiSatang),
      foreignSatang: num(p.foreignSatang) ?? num(p.foreignPriceSatang) ?? num(p.priceForeignSatang) ?? num(p.priceSatang),
    }));
  },

  availability: async (from: string, to: string): Promise<Availability> => {
    const { data } = await call<Json>(`/bff/commerce/products/availability?from=${from}&to=${to}`);
    const out: Availability = {};
    for (const a of items(data)) out[str(a.visitDate)] = num(a.remaining) ?? 0;
    return out;
  },

  createOrder: async (idempotencyKey: string, visitDate: string, orderItems: OrderItemInput[], lang: string): Promise<Hold> => {
    const { data, date } = await call<Json>('/bff/commerce/orders', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      json: { visitDate, items: orderItems, lang },
    });
    const o = ((data?.order as Json) ?? data) as Json;
    return {
      orderId: str(o.orderId ?? o.id),
      orderNo: str(o.orderNo),
      totalSatang: num(o.totalSatang) ?? 0,
      holdExpiresAt: str(o.holdExpiresAt),
      serverDate: date,
    };
  },

  pay: async (orderId: string, method: 'card' | 'promptpay', returnUrl: string, idempotencyKey: string): Promise<string> => {
    const { data } = await call<Json>(`/bff/commerce/orders/${encodeURIComponent(orderId)}/payments`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      json: { method, returnUrl },
    });
    return str(data?.paymentUrl ?? data?.redirectUrl ?? data?.url);
  },

  order: async (orderId: string): Promise<Order> => {
    const { data } = await call<Json>(`/bff/commerce/orders/${encodeURIComponent(orderId)}`);
    const o = ((data?.order as Json) ?? data) as Json;
    const its = Array.isArray(o.items) ? o.items.length : undefined;
    return {
      orderId: str(o.orderId ?? o.id, orderId),
      orderNo: str(o.orderNo),
      status: str(o.status, 'pending'),
      paymentStatus: str(o.paymentStatus, 'none'),
      totalSatang: num(o.totalSatang),
      visitDate: str(o.visitDate) || undefined,
      holdExpiresAt: str(o.holdExpiresAt) || undefined,
      ticketCount: its,
    };
  },
};

/* ---------------- ticket ---------------- */

export type Ticket = {
  ticketId: string;
  kind: string;
  visitDate: string | null;
  holderName: string;
  idLast4: string;
  status: string; // active | used | void | transferred
  qr: string;
  orderId?: string;
  orderNo?: string;
};

export const ticket = {
  mine: async (): Promise<Ticket[]> => {
    const { data } = await call<Json>('/bff/ticket/tickets/me');
    return items(data).map((t) => ({
      ticketId: str(t.ticketId ?? t.id),
      kind: str(t.kind, 'day'),
      visitDate: str(t.visitDate) || null,
      holderName: str(t.holderName ?? (t.holder as Json | undefined)?.name),
      idLast4: str(t.idLast4 ?? (t.holder as Json | undefined)?.idLast4),
      status: str(t.status, 'active'),
      qr: str(t.qr),
      orderId: str(t.orderId) || undefined,
      orderNo: str(t.orderNo) || undefined,
    }));
  },
};

/** Fresh idempotency key (POST /orders, POST /payments). */
export const newKey = () =>
  typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
