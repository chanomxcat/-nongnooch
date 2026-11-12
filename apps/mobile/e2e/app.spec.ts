import { expect, test, type Page } from '@playwright/test';

const PHONE = '+66811111111';
const MOCK = process.env.MOCKEXT_URL ?? 'http://localhost:8090';

// Flutter web with semantics on (demo builds): tappables are flt-semantics[flt-tappable] / role=button
// holding their label as text, text fields are real <input aria-label=...>.
const tap = (page: Page, text: string) =>
  page.locator('flt-semantics[flt-tappable], flt-semantics[role=button]').filter({ hasText: text }).first().click();
const type = async (page: Page, label: string, value: string) => {
  const input = page.locator(`input[aria-label*="${label}"]`);
  await input.click({ force: true });
  await input.focus();
  await page.keyboard.type(value);
  await expect(input).toHaveValue(value);
};

async function latestOtp(since: number): Promise<string> {
  for (let i = 0; i < 30; i++) {
    const r = await fetch(`${MOCK}/sms/messages?to=${encodeURIComponent(PHONE)}`);
    const j = (await r.json()) as { items: { at: string; text: string }[] };
    const m = j.items.find((it) => new Date(it.at).getTime() >= since)?.text.match(/\b(\d{6})\b/);
    if (m) return m[1];
    await new Promise((res) => setTimeout(res, 1000));
  }
  throw new Error('OTP not found in mockext');
}

test('tourist logs in with OTP and sees a ticket QR', async ({ page }) => {
  const external: string[] = [];
  page.on('request', (r) => {
    const h = new URL(r.url()).hostname;
    if (h !== 'localhost' && h !== '127.0.0.1') external.push(r.url());
  });

  await page.goto('/');
  await page.locator('flt-semantics[flt-tappable]').first().waitFor({ timeout: 60_000 });
  await tap(page, 'บัตรของฉัน');

  await type(page, 'เบอร์โทรศัพท์', '0811111111');
  const since = Date.now() - 2000;
  await tap(page, 'ขอรหัส OTP');

  const otp = await latestOtp(since);
  await type(page, 'รหัส OTP', otp);
  await tap(page, 'ยืนยัน');

  // Each ticket card is one semantics node whose aria-label holds kind, status, "QR ****<last4>", holder.
  const card = page.locator('flt-semantics[role=img][aria-label*="QR ****"]').first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await expect(card).toHaveAttribute('aria-label', /ใช้แล้ว|ยังไม่ใช้/);
  expect(external, 'no requests to external hosts').toEqual([]);
});
