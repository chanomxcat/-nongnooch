// CMS demo path: editor edits the hero title (TH + EN, clears JA) -> preview -> submit -> editor's own approve is
// blocked (four-eyes) -> approver approves -> website shows the new title, JA falls back to EN.
// Re-runnable: unique title per run, stale pending requests are rejected first, the original slide is restored after.
import { expect, test, type Browser, type Page } from '@playwright/test';

const GW = process.env.GATEWAY_URL ?? 'http://localhost:8000';
const WEB = 'http://localhost:3000';

async function api(method: string, path: string, token?: string, body?: unknown) {
  const r = await fetch(`${GW}${path}`, {
    method,
    headers: { ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}), ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await r.text();
  if (!r.ok) throw new Error(`${method} ${path} -> ${r.status} ${text}`);
  return text ? JSON.parse(text) : null;
}

const staffToken = async (who: string) =>
  (await api('POST', '/api/identity/auth/staff/login', undefined, { email: `${who}@demo.nongnooch.local`, password: 'Demo1234!' })).access as string;

async function login(browser: Browser, who: string, next: string): Promise<Page> {
  const page = await (await browser.newContext()).newPage();
  await page.goto(`/login?next=${next}`);
  await page.getByLabel('อีเมล / Email').fill(`${who}@demo.nongnooch.local`);
  await page.getByLabel('รหัสผ่าน / Password').fill('Demo1234!');
  await page.getByRole('button', { name: 'เข้าสู่ระบบ / Sign in' }).click();
  await expect(page).toHaveURL(new RegExp(next));
  return page;
}

type Rev = { attrs: Record<string, unknown>; texts: Record<string, Record<string, string>> };
let restore: { id: string; rev: Rev } | null = null;

test.afterAll(async () => {
  if (!restore) return;
  // Put the original slide back through the same workflow (editor saves + submits, approver approves).
  const [ed, ap] = await Promise.all([staffToken('editor'), staffToken('approver')]);
  for (const a of (await api('GET', '/api/content/admin/approvals?status=pending', ap)).items as { id: string; itemId: string }[]) {
    if (a.itemId === restore.id) await api('POST', `/api/content/admin/approvals/${a.id}/reject`, ap, { reason: 'e2e cleanup' });
  }
  await api('POST', `/api/content/admin/items/${restore.id}/draft`, ed, restore.rev);
  const { approvalId } = await api('POST', `/api/content/admin/items/${restore.id}/submit`, ed);
  await api('POST', `/api/content/admin/approvals/${approvalId}/approve`, ap);
});

test('cms: edit hero -> preview -> four-eyes block -> approve -> live on website', async ({ browser }) => {
  const stamp = Date.now().toString(36);
  const th = `สวนสวยทดสอบ ${stamp}`;
  const en = `Garden test ${stamp}`;

  // The hero headline is the hero_slide with the lowest order. Remember it and clear stale pending requests.
  const ap = await staffToken('approver');
  const heroId = (await api('GET', '/api/content/public/items?type=hero_slide&lang=th')).items[0].id as string;
  for (const a of (await api('GET', '/api/content/admin/approvals?status=pending', ap)).items as { id: string; itemId: string }[]) {
    if (a.itemId === heroId) await api('POST', `/api/content/admin/approvals/${a.id}/reject`, ap, { reason: 'e2e: stale request' });
  }
  const item = await api('GET', `/api/content/admin/items/${heroId}`, ap);
  restore = { id: heroId, rev: { attrs: item.published.attrs, texts: item.published.texts } };

  // Editor: content list -> first hero slide -> edit TH + EN, clear JA.
  const editor = await login(browser, 'editor', '/content');
  await editor.locator('.crow').first().click();
  await expect(editor).toHaveURL(new RegExp(`/content/${heroId}`));
  await editor.locator('#p-th-title').fill(th);
  await editor.locator('#s-en-title').fill(en);
  await editor.getByRole('tab', { name: /^JA/ }).click();
  await editor.locator('#s-ja-title').fill('');
  await editor.getByRole('button', { name: 'บันทึกร่าง' }).click();
  await expect(editor.getByRole('status').filter({ hasText: 'บันทึกร่างแล้ว' })).toBeVisible();

  // Preview opens the real website with the draft.
  const [preview] = await Promise.all([editor.waitForEvent('popup'), editor.getByRole('button', { name: 'ดูตัวอย่าง' }).click()]);
  await preview.waitForURL(/\/th\?preview=/);
  await expect(preview.getByText('โหมดตัวอย่าง (ยังไม่เผยแพร่)')).toBeVisible();
  await expect(preview.locator('#hero-title')).toHaveText(th);
  await preview.close();

  // Not live yet.
  const web = await (await browser.newContext()).newPage();
  await web.goto(`${WEB}/th`);
  await expect(web.locator('#hero-title')).not.toHaveText(th);

  await editor.getByRole('button', { name: 'ส่งอนุมัติ' }).click();
  await expect(editor.getByTestId('status')).toHaveText(/รออนุมัติ/);

  // Editor tries to approve their own change -> four-eyes.
  await editor.goto('/approvals');
  const mine = editor.getByTestId('approval').filter({ hasText: th });
  await expect(mine).toContainText(en); // field-level diff
  await mine.getByRole('button', { name: 'อนุมัติ', exact: true }).click();
  await expect(editor.locator('.toast.t-err')).toContainText('หลักสี่ตา');

  // Approver approves.
  const approver = await login(browser, 'approver', '/approvals');
  const card = approver.getByTestId('approval').filter({ hasText: th });
  await card.getByRole('button', { name: 'อนุมัติ', exact: true }).click();
  await expect(approver.getByRole('status').filter({ hasText: 'อนุมัติแล้ว' })).toBeVisible();

  // Live immediately; JA has no title now -> EN fallback.
  await web.goto(`${WEB}/th`);
  await expect(web.locator('#hero-title')).toHaveText(th);
  await web.goto(`${WEB}/ja`);
  await expect(web.locator('#hero-title')).toHaveText(en);

  // Audit log shows the publish by the approver.
  await approver.goto('/audit');
  await expect(approver.locator('tbody tr').filter({ hasText: 'อนุมัติและเผยแพร่' }).first()).toContainText('Demo approver');
});
