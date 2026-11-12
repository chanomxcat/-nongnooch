'use client';

import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import {
  canEdit, cms, errText, fmtTime, headline, LANG_LABEL, LANGS, STATUS_TH, TYPE_TH, who,
  type Field, type ItemDetail, type Texts, type TypeDef,
} from '@/lib/cms';

type Toast = { kind: 'ok' | 'err'; text: string } | null;

function attrsToForm(def: TypeDef, attrs: Record<string, unknown> = {}) {
  const f: Record<string, string> = {};
  for (const a of def.attrs) {
    const v = attrs[a.key];
    f[a.key] = v === undefined || v === null ? '' : Array.isArray(v) ? v.join(', ') : String(v);
  }
  return f;
}

function formToAttrs(def: TypeDef, f: Record<string, string>) {
  const out: Record<string, unknown> = {};
  for (const a of def.attrs) {
    const v = (f[a.key] ?? '').trim();
    if (!v) continue;
    if (a.kind === 'int') out[a.key] = Number.isFinite(Number(v)) ? Math.trunc(Number(v)) : v;
    else if (a.kind === 'times') out[a.key] = v.split(/[,\s]+/).filter(Boolean);
    else out[a.key] = v;
  }
  return out;
}

export default function Editor({ id, roles }: { id: string; roles: string[] }) {
  const [def, setDef] = useState<TypeDef | null>(null);
  const [d, setD] = useState<ItemDetail | null>(null);
  const [attrs, setAttrs] = useState<Record<string, string>>({});
  const [texts, setTexts] = useState<Texts>({});
  const [lang, setLang] = useState<string>('en');
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<Toast>(null);
  const [previewUrl, setPreviewUrl] = useState('');
  const [loadErr, setLoadErr] = useState('');

  const load = useCallback(async () => {
    const [t, r] = await Promise.all([cms<{ types: TypeDef[] }>('GET', 'admin/types'), cms<ItemDetail>('GET', `admin/items/${id}`)]);
    if (!r.ok) return setLoadErr(errText(r));
    if (!t.ok) return setLoadErr(errText(t));
    const td = t.data.types.find((x) => x.type === r.data.type) ?? null;
    setDef(td);
    setD(r.data);
    const src = r.data.draft?.after ?? r.data.published;
    if (td) setAttrs(attrsToForm(td, src?.attrs));
    setTexts(structuredClone(src?.texts ?? {}));
    setDirty(false);
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 6000);
    return () => clearTimeout(t);
  }, [toast]);

  if (loadErr) return <p className="alert">{loadErr}</p>;
  if (!def || !d) return <div className="skel" />;

  const status = d.draft?.status ?? 'published';
  const locked = status === 'pending' || !canEdit(roles);
  const fail = (r: { error: string; fields?: string[] }) => setToast({ kind: 'err', text: errText(r) });

  function setText(l: string, k: string, v: string) {
    setTexts((p) => ({ ...p, [l]: { ...(p[l] ?? {}), [k]: v } }));
    setDirty(true);
  }

  async function save(quiet = false) {
    const clean: Texts = {};
    for (const [l, m] of Object.entries(texts)) {
      for (const [k, v] of Object.entries(m ?? {})) if (v.trim()) (clean[l] ??= {})[k] = v.trim();
    }
    const r = await cms<ItemDetail>('POST', `admin/items/${id}/draft`, { attrs: formToAttrs(def!, attrs), texts: clean });
    if (!r.ok) {
      fail(r);
      return false;
    }
    setD(r.data);
    setDirty(false);
    if (!quiet) setToast({ kind: 'ok', text: 'บันทึกร่างแล้ว' });
    return true;
  }

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    try {
      await fn();
    } finally {
      setBusy(false);
    }
  }

  const onSave = () => run(async () => void (await save()));

  const onPreview = () => {
    // Open the tab inside the click so popup blockers allow it, then point it at the preview.
    const w = window.open('about:blank', '_blank');
    return run(async () => {
      if ((dirty || !d.draft) && !(await save(true))) return w?.close();
      const r = await cms<{ url: string }>('POST', `admin/items/${id}/preview`);
      if (!r.ok) {
        w?.close();
        return fail(r);
      }
      setPreviewUrl(r.data.url);
      if (w) w.location.href = r.data.url;
    });
  };

  const onSubmit = () =>
    run(async () => {
      if ((dirty || !d.draft) && !(await save(true))) return;
      const r = await cms<{ approvalId: string }>('POST', `admin/items/${id}/submit`);
      if (!r.ok) return fail(r);
      setToast({ kind: 'ok', text: 'ส่งอนุมัติแล้ว รอผู้อนุมัติตรวจสอบ' });
      await load();
    });

  const fieldInput = (f: Field, l: string, primary: boolean) => {
    const v = texts[l]?.[f.key] ?? '';
    const fallback = texts.en?.[f.key] || texts.th?.[f.key] || '';
    const ph = l === 'th' || l === 'en' ? (f.required ? 'จำเป็นต้องกรอก' : '') : fallback ? `ว่างไว้ = ใช้ EN: ${fallback}` : '';
    const idf = `${primary ? 'p' : 's'}-${l}-${f.key}`;
    const label = `${f.label} (${l.toUpperCase()})`;
    return (
      <label className="field" key={idf} htmlFor={idf}>
        <span>
          {label}
          {f.required && (l === 'th' || l === 'en') ? <b className="req"> *</b> : null}
        </span>
        {f.long ? (
          <textarea id={idf} className="in" rows={3} value={v} placeholder={ph} disabled={locked} onChange={(e) => setText(l, f.key, e.target.value)} />
        ) : (
          <input id={idf} className="in" value={v} placeholder={ph} disabled={locked} onChange={(e) => setText(l, f.key, e.target.value)} />
        )}
      </label>
    );
  };

  const filled = (l: string) => Object.values(texts[l] ?? {}).some((x) => x.trim());
  const rejected = d.lastDecision?.status === 'rejected' && status !== 'pending' ? d.lastDecision : null;

  return (
    <>
      <div className="page-h">
        <h1>
          {headline({ ...texts.en, ...texts.th })}
          <small>
            <Link href={`/content?type=${d.type}`}>← {TYPE_TH[d.type] ?? d.type}</Link>
          </small>
        </h1>
        <span className={`badge b-${status}`} data-testid="status">
          {STATUS_TH[status]}
          {dirty ? ' · มีการแก้ไขที่ยังไม่บันทึก' : ''}
        </span>
      </div>

      {status === 'pending' ? (
        <p className="notice">รายการนี้รออนุมัติอยู่ จะแก้ไขได้อีกครั้งเมื่อผู้อนุมัติตัดสินแล้ว (ผู้แก้ไข: {d.draft?.editors.map((e) => who(d.names, e)).join(', ')})</p>
      ) : null}
      {rejected ? (
        <p className="alert">
          ไม่อนุมัติโดย {who(d.names, rejected.decidedBy)} เมื่อ {fmtTime(rejected.updatedAt)} — เหตุผล: {rejected.reason}
        </p>
      ) : null}

      <section className="panel">
        <h2>ข้อมูลทั่วไป</h2>
        <div className="attrs">
          {def.attrs.map((f) => (
            <label className="field" key={f.key} htmlFor={`a-${f.key}`}>
              <span>
                {f.label}
                {f.required ? <b className="req"> *</b> : null}
              </span>
              <input
                id={`a-${f.key}`}
                className="in"
                inputMode={f.kind === 'int' ? 'numeric' : undefined}
                value={attrs[f.key] ?? ''}
                disabled={locked}
                onChange={(e) => {
                  setAttrs((p) => ({ ...p, [f.key]: e.target.value }));
                  setDirty(true);
                }}
              />
            </label>
          ))}
          {attrs.imageUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img className="thumb" alt="" src={attrs.imageUrl.startsWith('/') ? `http://localhost:3000${attrs.imageUrl}` : attrs.imageUrl} />
          ) : null}
        </div>
      </section>

      <div className="editor">
        <section className="panel">
          <h2>ภาษาไทย (ต้นฉบับ)</h2>
          {def.texts.map((f) => fieldInput(f, 'th', true))}
        </section>
        <section className="panel">
          <div className="tabs" role="tablist" aria-label="ภาษา">
            {LANGS.map((l) => (
              <button key={l} type="button" role="tab" aria-selected={lang === l} className="tab" onClick={() => setLang(l)} title={LANG_LABEL[l]}>
                {l.toUpperCase()}
                {filled(l) ? <span className="dot-on" aria-label="มีข้อความ" /> : null}
              </button>
            ))}
          </div>
          <h2>{LANG_LABEL[lang]}</h2>
          {lang !== 'th' && lang !== 'en' ? <p className="hint">ช่องที่เว้นว่าง เว็บไซต์จะแสดงภาษาอังกฤษแทน</p> : null}
          {def.texts.map((f) => fieldInput(f, lang, false))}
        </section>
      </div>

      {canEdit(roles) ? (
        <div className="actions">
          <button className="btn btn-ghost" disabled={busy || locked} onClick={onSave}>
            บันทึกร่าง
          </button>
          <button className="btn btn-ghost" disabled={busy || !canEdit(roles)} onClick={onPreview}>
            ดูตัวอย่าง
          </button>
          <button className="btn btn-gold" disabled={busy || locked} onClick={onSubmit}>
            ส่งอนุมัติ
          </button>
          {status === 'pending' ? (
            <Link className="btn" href="/approvals">
              ไปที่รออนุมัติ
            </Link>
          ) : null}
          {previewUrl ? (
            <a className="hint" href={previewUrl} target="_blank" rel="noreferrer">
              ลิงก์ตัวอย่าง (ใช้ได้ 30 นาที)
            </a>
          ) : null}
        </div>
      ) : null}

      {toast ? (
        <div className={`toast t-${toast.kind}`} role={toast.kind === 'err' ? 'alert' : 'status'} onClick={() => setToast(null)}>
          {toast.text}
        </div>
      ) : null}
    </>
  );
}
