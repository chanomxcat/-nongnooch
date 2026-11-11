// Staff demo path: login -> paste a fresh ticket QR -> PASS -> same QR again -> ALREADY USED -> dashboard numbers.
// Run with `make e2e APP=admin` against `make up && make seed`. The ticket is bought through the real APIs.
import { expect, test } from '@playwright/test';

const GW = process.env.GATEWAY_URL ?? 'http://localhost:8000';
const MOCK = process.env.MOCKEXT_URL ?? 'http://localhost:8090';

async function api(method: string, path: string, opts: { token?: string; body?: unknown; headers?: Record<string, string> } = {}) {
  const r = await fetch(path.startsWith('http') ? path : `${GW}${path}`, {
    method,
    headers: {
      ...(opts.body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...(opts.token ? { Authorization: `Bearer ${opts.token}` } : {}),
      ...opts.headers,
    },
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });
  const text = await r.text();
  if (!r.ok) throw new Error(`${method} ${path} -> ${r.status} ${text}`);
  return text ? JSON.parse(text) : null;
}

async function until<T>(fn: () => Promise<T | undefined>, ms = 30_000): Promise<T> {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    const v = await fn().catch(() => undefined);
    if (v) return v;
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error('timeout');
}

/** Registers a fresh foreign member, buys one day ticket for today, pays on mockext and returns its QR. */
async function freshTicketQr(): Promise<string> {
  const phone = `+668${Math.floor(10_000_000 + Math.random() * 89_999_999)}`;
  const since = Date.now() - 2000;
  const { requestId } = await api('POST', '/api/identity/auth/otp', { body: { phone, lang: 'en', turnstileToken: 'mock-ok' } });
  const otp = await until(async () => {
    const { items } = await api('GET', `${MOCK}/sms/messages?to=${encodeURIComponent(phone)}`);
    const it = (items as { text: string; at: string }[]).find((m) => new Date(m.at).getTime() >= since);
    return it?.text.match(/\b(\d{6})\b/)?.[1];
  });
  let tk = await api('POST', '/api/identity/auth/verify', { body: { requestId, otp } });
  await api('POST', '/api/member/members', {
    token: tk.access,
    body: {
      firstName: 'Gate', lastName: 'Test', nationality: 'US', idNumber: `E2E${phone.slice(-6)}`, passportCountry: 'US',
      lang: 'en', channel: 'web', consents: { terms: true, pdpa: true, marketing: false },
    },
  });
  tk = await api('POST', '/api/identity/auth/refresh', { body: { refreshToken: tk.refresh } });
  const token = tk.access;
  const { items: products } = await api('GET', '/api/commerce/products?lang=en');
  const day = (products as { productId: string; kind: string; ageGroup: string }[]).find((p) => p.kind === 'day' && p.ageGroup === 'adult')!;
  const visitDate = new Date(Date.now() + 7 * 3600_000).toISOString().slice(0, 10);
  const order = await api('POST', '/api/commerce/orders', {
    token,
    headers: { 'Idempotency-Key': `e2e-${phone}` },
    body: { visitDate, items: [{ productId: day.productId, audience: 'foreign', holder: { companionId: 'me' } }] },
  });
  const pay = await api('POST', `/api/commerce/orders/${order.orderId}/payments`, {
    token,
    headers: { 'Idempotency-Key': `e2e-${phone}-pay` },
    body: { method: 'card', returnUrl: 'http://localhost:3000/th' },
  });
  const r = await fetch(`${MOCK}${new URL(pay.paymentUrl).pathname}/pay`, {
    method: 'POST',
    redirect: 'manual',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: 'action=success',
  });
  expect(r.status).toBeLessThan(400);
  return until(async () => {
    const { items } = await api('GET', '/api/ticket/tickets/me', { token });
    return (items as { orderId: string; qr: string }[]).find((t) => t.orderId === order.orderId)?.qr;
  });
}

test('staff: scan fresh QR -> PASS -> ALREADY USED -> dashboard', async ({ page }) => {
  const qr = await freshTicketQr();
  expect(qr).toMatch(/^NN1\./);

  await page.goto('/login?next=/gate');
  await page.getByLabel('อีเมล / Email').fill('admin@demo.nongnooch.local');
  await page.getByLabel('รหัสผ่าน / Password').fill('Demo1234!');
  await page.getByRole('button', { name: 'เข้าสู่ระบบ / Sign in' }).click();
  await expect(page).toHaveURL(/\/gate/);

  const paste = page.getByPlaceholder('NN1.…');
  const result = page.locator('.result');

  await paste.fill(qr);
  await page.getByRole('button', { name: 'ตรวจ' }).click();
  await expect(result).toContainText('PASS');
  await result.click();
  await expect(result).toBeHidden();

  await paste.fill(qr);
  await page.getByRole('button', { name: 'ตรวจ' }).click();
  await expect(result).toContainText('ALREADY USED');
  await result.click();

  await page.goto('/dashboard');
  const entries = page.locator('section.card', { hasText: 'Gate entries' }).locator('.v');
  await expect(entries).toHaveText(/^[1-9][\d,]*$/, { timeout: 15_000 });
  const sales = page.locator('section.card', { hasText: 'Sales today' }).locator('.v');
  await expect(sales).toContainText('฿');
});
