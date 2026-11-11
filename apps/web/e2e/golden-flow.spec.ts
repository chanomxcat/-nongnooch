// Demo golden path (run with `make e2e APP=web` against `make up && make seed`; full hardening in Task 10).
import { expect, test } from '@playwright/test';

const MOCKEXT = process.env.MOCKEXT_URL ?? 'http://localhost:8090';

function randomThaiId(): string {
  const d = Array.from({ length: 12 }, (_, i) => (i === 0 ? 1 + Math.floor(Math.random() * 8) : Math.floor(Math.random() * 10)));
  const sum = d.reduce((n, v, i) => n + v * (13 - i), 0);
  return [...d, (11 - (sum % 11)) % 10].join('');
}

async function latestOtp(request: import('@playwright/test').APIRequestContext, phone: string): Promise<string> {
  for (let i = 0; i < 30; i++) {
    const r = await request.get(`${MOCKEXT}/sms/messages?to=${encodeURIComponent(phone)}`);
    const items = ((await r.json()).items ?? []) as { text: string }[];
    const m = items.at(-1)?.text.match(/\b(\d{6})\b/);
    if (m) return m[1];
    await new Promise((res) => setTimeout(res, 1000));
  }
  throw new Error('no OTP');
}

test('register with invite → buy → pay on mockext → see QR', async ({ page, request }) => {
  const local = `08${Math.floor(10_000_000 + Math.random() * 89_999_999)}`;
  const e164 = `+66${local.slice(1)}`;

  await page.goto('/th/r/DEMOGUIDE1');
  await expect(page).toHaveURL(/\/th\/register/);
  await page.getByLabel('เบอร์มือถือ').fill(local);
  await page.getByText('ฉันไม่ใช่โปรแกรมอัตโนมัติ').click();
  await page.getByRole('button', { name: 'ขอรหัส OTP' }).click();
  await page.getByLabel('รหัส OTP').fill(await latestOtp(request, e164));
  await page.getByRole('button', { name: 'ยืนยัน' }).click();

  await expect(page).toHaveURL(/register\/profile/);
  await page.getByLabel('ชื่อ', { exact: true }).fill('สมชาย');
  await page.getByLabel('นามสกุล').fill('ใจดี');
  await page.getByLabel('เลขบัตรประชาชน').fill(randomThaiId());
  await expect(page.getByLabel('รหัสเชิญ (ถ้ามี)')).toHaveValue('DEMOGUIDE1');
  await page.getByText('ฉันยอมรับข้อกำหนดการใช้บริการ').click();
  await page.getByText(/ฉันยินยอมให้ประมวลผลข้อมูลส่วนบุคคล/).click();
  await page.getByRole('button', { name: 'สมัครสมาชิก' }).click();
  await expect(page.getByText('ได้รับ 150 พ้อย')).toBeVisible();

  await page.getByRole('link', { name: 'ซื้อบัตร' }).first().click();
  await page.locator('[data-date]:not([disabled])').first().click();
  await page.getByRole('button', { name: 'ดำเนินการต่อ' }).click();
  await page.getByRole('button', { name: 'จองบัตรและไปชำระเงิน' }).click();
  await page.getByRole('button', { name: /ชำระ/ }).click();

  await expect(page).toHaveURL(/\/pg\/checkout\//);
  await page.locator('button[value="success"]').click();
  await expect(page).toHaveURL(/\/th\/checkout\/result\//);
  await expect(page.locator('[data-qr^="NN1."]').first()).toBeVisible({ timeout: 60_000 });
});
