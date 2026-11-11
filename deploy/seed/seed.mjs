// Demo seed: POST /internal/seed per service, then today's demo history through the real APIs.
// Runs inside the compose network (make seed): /internal/* is blocked at the gateway.
// Idempotent: service seeds are idempotent; history is skipped when today's orders already exist.
import { readFileSync } from 'node:fs';

const GW = process.env.GATEWAY_URL ?? 'http://gateway:8080';
const MOCK = process.env.MOCKEXT_URL ?? 'http://mockext:8090';
const SERVICES = ['identity', 'member', 'commerce', 'ticket', 'notify', 'content'];
const fx = JSON.parse(readFileSync(new URL('./fixtures.json', import.meta.url), 'utf8'));
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const log = (...a) => console.log('[seed]', ...a);

export function thaiId(prefix12) {
  const d = [...prefix12].map(Number);
  const sum = d.reduce((n, v, i) => n + v * (13 - i), 0);
  return prefix12 + ((11 - (sum % 11)) % 10);
}

async function call(method, url, { token, body, headers = {}, ok = [200, 201, 202, 204] } = {}) {
  const h = { ...headers };
  if (token) h.Authorization = `Bearer ${token}`;
  if (body !== undefined) h['Content-Type'] = 'application/json';
  const r = await fetch(url, { method, headers: h, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await r.text();
  const json = text ? (() => { try { return JSON.parse(text); } catch { return text; } })() : null;
  if (!ok.includes(r.status)) throw new Error(`${method} ${url} -> ${r.status} ${text.slice(0, 200)}`);
  return { status: r.status, json };
}

async function waitFor(name, fn, timeoutMs = 120_000) {
  const end = Date.now() + timeoutMs;
  let last;
  while (Date.now() < end) {
    try {
      const v = await fn();
      if (v) return v;
    } catch (e) { last = e; }
    await sleep(1000);
  }
  throw new Error(`timeout waiting for ${name}: ${last?.message ?? 'not ready'}`);
}

async function seedServices() {
  for (const svc of SERVICES) {
    await waitFor(`${svc} /healthz`, async () => (await fetch(`http://${svc}:8080/healthz`)).ok);
    // Retry: a service can answer /healthz before its migrations / DB pool are ready.
    await waitFor(`${svc} seed`, async () => (await call('POST', `http://${svc}:8080/internal/seed`), true), 60_000);
    log(`${svc}: seeded`);
  }
}

async function otpLogin(phone, lang = 'th') {
  const t0 = Date.now() - 2000;
  const { json } = await waitFor(`otp ${phone}`, () =>
    call('POST', `${GW}/api/identity/auth/otp`, { body: { phone, lang, turnstileToken: 'mock-ok' } }), 30_000);
  const code = await waitFor(`sms ${phone}`, async () => {
    const r = await call('GET', `${MOCK}/sms/messages?to=${encodeURIComponent(phone)}`);
    const m = r.json.items.find((it) => new Date(it.at).getTime() >= t0)?.text.match(/\b(\d{6})\b/);
    return m?.[1];
  }, 30_000);
  return (await call('POST', `${GW}/api/identity/auth/verify`, { body: { requestId: json.requestId, otp: code } })).json;
}

async function refresh(tokens) {
  return (await call('POST', `${GW}/api/identity/auth/refresh`, { body: { refreshToken: tokens.refresh } })).json;
}

async function registerNew(m) {
  let tk = await otpLogin(m.phone, m.lang);
  const body = {
    firstName: m.firstName, lastName: m.lastName, nationality: m.nationality,
    idNumber: m.idPrefix ? thaiId(m.idPrefix) : m.idNumber, passportCountry: m.passportCountry ?? '',
    inviteCode: m.inviteCode ?? '', lang: m.lang, channel: 'web',
    consents: { terms: true, pdpa: true, marketing: false },
  };
  await call('POST', `${GW}/api/member/members`, { token: tk.access, body });
  tk = await refresh(tk); // token now carries mid
  log(`member registered ${m.phone}`);
  return tk.access;
}

function bangkokToday() {
  return new Date(Date.now() + 7 * 3600_000).toISOString().slice(0, 10);
}

async function seedHistory() {
  const staff = (email) => call('POST', `${GW}/api/identity/auth/staff/login`, { body: { email, password: 'Demo1234!' } }).then((r) => r.json.access);
  const admin = await staff('admin@demo.nongnooch.local');
  const before = (await call('GET', `${GW}/api/commerce/admin/stats/today`, { token: admin })).json;
  if (before.orders >= fx.orders.length) {
    log(`history: ${before.orders} paid orders today already, skipping`);
    return;
  }

  const tokens = {};
  for (const phone of ['+66811111111', '+8613800138000']) tokens[phone] = (await otpLogin(phone)).access;
  for (const m of fx.newMembers) tokens[m.phone] = await registerNew(m);

  const products = (await call('GET', `${GW}/api/commerce/products?lang=en`)).json.items;
  const pid = (key) => {
    const [kind, age] = key === 'annual' ? ['annual', 'adult'] : key.split('-');
    const p = products.find((x) => x.kind === kind && x.ageGroup === age);
    if (!p) throw new Error(`product ${key} not found`);
    return p.productId;
  };
  const visitDate = bangkokToday();
  const orders = [];
  for (const [i, o] of fx.orders.entries()) {
    const token = tokens[o.buyer];
    const items = o.items.map(([prod, audience, holder]) => {
      if (holder === 'me') return { productId: pid(prod), audience, holder: { companionId: 'me' } };
      const [name, nationality, idLast4] = holder.split('/');
      return { productId: pid(prod), audience, holder: { name, nationality, idLast4 } };
    });
    const key = `demo-seed-${visitDate}-${i}`;
    const ord = (await call('POST', `${GW}/api/commerce/orders`, { token, body: { visitDate, items }, headers: { 'Idempotency-Key': key } })).json;
    const pay = (await call('POST', `${GW}/api/commerce/orders/${ord.orderId}/payments`, {
      token, headers: { 'Idempotency-Key': `${key}-pay` },
      body: { method: i % 3 === 2 ? 'promptpay' : 'card', returnUrl: `http://localhost:3000/th/checkout/result/${ord.orderId}` },
    })).json;
    // paymentUrl points at the host-facing mockext; reach it inside the network instead.
    const url = new URL(pay.paymentUrl);
    const r = await fetch(`${MOCK}${url.pathname}/pay`, {
      method: 'POST', redirect: 'manual',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, body: 'action=success',
    });
    if (r.status >= 400) throw new Error(`mock pay ${ord.orderNo} -> ${r.status}`);
    orders.push({ ...ord, token, count: items.length });
    log(`order ${ord.orderNo} paid (${items.length} tickets)`);
  }

  // Tickets are issued asynchronously (order.paid -> ticket service).
  const tickets = [];
  for (const o of orders) {
    const mine = await waitFor(`tickets for ${o.orderNo}`, async () => {
      const items = (await call('GET', `${GW}/api/ticket/tickets/me`, { token: o.token })).json.items.filter((t) => t.orderId === o.orderId);
      return items.length === o.count ? items : null;
    }, 60_000);
    tickets.push(...mine);
  }
  log(`${tickets.length} tickets issued`);

  const gate = await staff('gate@demo.nongnooch.local');
  const scannable = tickets.filter((t) => t.kind === 'day' && t.status !== 'used').slice(0, fx.scans);
  for (const t of scannable) {
    const res = (await call('POST', `${GW}/api/ticket/gate/scans`, { token: gate, body: { qr: t.qr, gateId: fx.gateId } })).json;
    if (res.result !== 'ok') throw new Error(`scan ${t.ticketId} -> ${res.result}`);
  }
  log(`${scannable.length} gate scans`);
}

const t0 = Date.now();
await seedServices();
if (!process.argv.includes('--no-history')) await seedHistory();

// Summary as the dashboard sees it.
const admin = (await call('POST', `${GW}/api/identity/auth/staff/login`, { body: { email: 'admin@demo.nongnooch.local', password: 'Demo1234!' } })).json.access;
const stat = async (svc) => (await call('GET', `${GW}/api/${svc}/admin/stats/today`, { token: admin })).json;
const [c, t, m] = await Promise.all([stat('commerce'), stat('ticket'), stat('member')]);
log(`today: orders=${c.orders} sales=฿${c.salesSatang / 100} tickets=${t.ticketsIssuedToday} scans=${t.scansToday} newMembers=${m.newMembersToday}`);
log(`done in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
