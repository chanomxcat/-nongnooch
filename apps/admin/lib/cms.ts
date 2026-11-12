/** Admin client for the content service (via the BFF at /bff/content/...). */

export const LANGS = ['th', 'en', 'zh', 'ja', 'ko', 'de', 'fr'] as const;
export const LANG_LABEL: Record<string, string> = { th: 'ไทย', en: 'English', zh: '中文', ja: '日本語', ko: '한국어', de: 'Deutsch', fr: 'Français' };

export type Field = { key: string; kind?: 'int' | 'string' | 'times'; label: string; required: boolean; long?: boolean };
export type TypeDef = { type: string; label: string; attrs: Field[]; texts: Field[] };
export type Texts = Record<string, Record<string, string>>;
export type Rev = { revId: string; attrs: Record<string, unknown>; texts: Texts };
export type Approval = {
  id: string; itemId: string; type: string; status: 'draft' | 'pending' | 'approved' | 'rejected';
  editors: string[]; submittedBy?: string; reason?: string; decidedBy?: string; createdAt: string; updatedAt: string;
  before: Rev | null; after: Rev | null;
};
export type ItemDetail = { id: string; type: string; published: Rev | null; draft: Approval | null; lastDecision: Approval | null; names: Record<string, string> };
export type ItemSummary = { id: string; type: string; status: 'published' | 'draft' | 'pending'; published: boolean; attrs: Record<string, unknown>; texts: Record<string, string>; updatedAt: string };
export type AuditRow = {
  id: string; at: string; actorId: string; actorRoles: string[]; action: string; entity: string; entityId: string;
  before: unknown; after: unknown; reason?: string;
};

export type Res<T> = { ok: true; data: T } | { ok: false; status: number; error: string; message: string; fields?: string[] };

const ERR_TH: Record<string, string> = {
  four_eyes_violation: 'อนุมัติไม่ได้: คุณเป็นผู้แก้ไขรายการนี้เอง ต้องให้ผู้อนุมัติคนอื่นอนุมัติ (หลักสี่ตา)',
  forbidden: 'บัญชีนี้ไม่มีสิทธิ์ทำรายการนี้',
  not_editable: 'รายการนี้รออนุมัติอยู่ จึงแก้ไขไม่ได้',
  not_pending: 'รายการนี้ได้รับการตัดสินไปแล้ว',
  no_draft: 'ยังไม่มีฉบับร่าง กรุณาบันทึกร่างก่อน',
  reason_required: 'กรุณาระบุเหตุผลที่ไม่อนุมัติ',
  missing_required: 'กรอกข้อมูลที่จำเป็นให้ครบทั้งภาษาไทยและอังกฤษ',
  invalid_attrs: 'ข้อมูลไม่ถูกต้อง (ตรวจสอบลำดับและเวลา HH:MM)',
  unknown_type: 'ไม่รู้จักประเภทเนื้อหานี้',
  not_found: 'ไม่พบรายการ',
  upstream_unavailable: 'ระบบเนื้อหาไม่ตอบสนอง ลองใหม่อีกครั้ง',
  unauthenticated: 'เซสชันหมดอายุ กรุณาเข้าสู่ระบบใหม่ในแท็บใหม่ (http://localhost:3001/login) แล้วกดอีกครั้ง',
};

export function errText(r: { error: string; fields?: string[] }) {
  const base = ERR_TH[r.error] ?? `เกิดข้อผิดพลาด (${r.error})`;
  return r.fields?.length ? `${base}: ${r.fields.join(', ')}` : base;
}

export async function cms<T>(method: 'GET' | 'POST', path: string, body?: unknown): Promise<Res<T>> {
  try {
    const r = await fetch(`/bff/content/${path}`, {
      method,
      cache: 'no-store',
      headers: { 'X-Requested-With': 'nn-web', ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    // Staff tokens last 15 min and there is no refresh: never navigate away, so typed edits stay on the page.
    if (r.status === 401) return { ok: false, status: 401, error: 'unauthenticated', message: '' };
    const j = await r.json().catch(() => ({}));
    if (!r.ok) return { ok: false, status: r.status, error: j.error ?? 'error', message: j.message ?? '', fields: j.fields };
    return { ok: true, data: j as T };
  } catch {
    return { ok: false, status: 0, error: 'upstream_unavailable', message: '' };
  }
}

export const STATUS_TH: Record<string, string> = {
  published: 'เผยแพร่แล้ว', draft: 'ฉบับร่าง', pending: 'รออนุมัติ', approved: 'อนุมัติแล้ว', rejected: 'ไม่อนุมัติ',
};

export const ACTION_TH: Record<string, string> = {
  'content.draft_saved': 'บันทึกร่าง', 'content.submitted': 'ส่งอนุมัติ', 'content.published': 'อนุมัติและเผยแพร่', 'content.rejected': 'ไม่อนุมัติ',
};

export const TYPE_TH: Record<string, string> = { hero_slide: 'สไลด์หน้าแรก', show: 'รอบการแสดง', faq: 'คำถามที่พบบ่อย' };

/** Title-ish text of a revision for lists. */
export function headline(texts: Record<string, string> | undefined) {
  return texts?.title || texts?.name || texts?.question || '(ไม่มีชื่อ)';
}

export const who = (names: Record<string, string>, id?: string) => (id ? names[id] ?? `${id.slice(0, 8)}…` : '-');

export function fmtTime(iso: string) {
  return new Date(iso).toLocaleString('th-TH', { timeZone: 'Asia/Bangkok', dateStyle: 'short', timeStyle: 'short' });
}

/** Field-level diff rows between two revisions (attrs + every language). */
export function diffRevs(before: Rev | null, after: Rev | null) {
  const rows: { key: string; before: string; after: string }[] = [];
  const show = (v: unknown) => (v === undefined || v === null || v === '' ? '' : Array.isArray(v) ? v.join(', ') : String(v));
  const ak = new Set([...Object.keys(before?.attrs ?? {}), ...Object.keys(after?.attrs ?? {})]);
  for (const k of ak) {
    const b = show(before?.attrs?.[k]);
    const a = show(after?.attrs?.[k]);
    if (a !== b) rows.push({ key: `attrs.${k}`, before: b, after: a });
  }
  for (const l of LANGS) {
    const fk = new Set([...Object.keys(before?.texts?.[l] ?? {}), ...Object.keys(after?.texts?.[l] ?? {})]);
    for (const f of fk) {
      const b = before?.texts?.[l]?.[f] ?? '';
      const a = after?.texts?.[l]?.[f] ?? '';
      if (a !== b) rows.push({ key: `${l.toUpperCase()} · ${f}`, before: b, after: a });
    }
  }
  return rows;
}

export const canEdit = (roles: string[]) => roles.includes('editor') || roles.includes('admin');
export const canDecide = (roles: string[]) => roles.includes('approver') || roles.includes('admin');
export const canCms = (roles: string[]) => canEdit(roles) || canDecide(roles) || roles.includes('viewer');
