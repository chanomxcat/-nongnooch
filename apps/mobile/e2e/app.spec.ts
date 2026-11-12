import { expect, test, type Page } from '@playwright/test';

const PHONE = '+66811111111';
const MOCK = process.env.MOCKEXT_URL ?? 'http://localhost:8090';

// Flutter web (semantics on in demo builds): text shows up as DOM text or aria-label.
const el = (page: Page, text: string) =>
  page.locator(`[aria-label*="${text}"]`).or(page.getByText(text)).first();

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
  await el(page, 'บัตรของฉัน').click({ timeout: 60_000 });

  await el(page, 'เบอร์โทรศัพท์').click();
  await page.keyboard.type('0811111111');
  const since = Date.now() - 2000;
  await el(page, 'ขอรหัส OTP').click();

  const otp = await latestOtp(since);
  await el(page, 'รหัส OTP 6 หลัก').click();
  await page.keyboard.type(otp);
  await el(page, 'ยืนยัน').click();

  await expect(page.locator('[aria-label^="QR "]').first()).toBeVisible({ timeout: 30_000 });
  await expect(el(page, 'ใช้แล้ว').or(el(page, 'ยังไม่ใช้'))).toBeVisible();
  expect(external, 'no requests to external hosts').toEqual([]);
});
