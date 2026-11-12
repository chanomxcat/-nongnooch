'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useEffect, useState } from 'react';
import { canEdit, cms, errText, fmtTime, headline, STATUS_TH, type ItemSummary, type TypeDef } from '@/lib/cms';

export default function ContentList({ roles }: { roles: string[] }) {
  const router = useRouter();
  const sp = useSearchParams();
  const [types, setTypes] = useState<TypeDef[]>([]);
  const type = sp.get('type') ?? 'hero_slide';
  const [items, setItems] = useState<ItemSummary[] | null>(null);
  const [err, setErr] = useState('');

  useEffect(() => {
    cms<{ types: TypeDef[] }>('GET', 'admin/types').then((r) => r.ok && setTypes(r.data.types));
  }, []);
  useEffect(() => {
    setItems(null);
    cms<{ items: ItemSummary[] }>('GET', `admin/items?type=${type}`).then((r) => {
      if (r.ok) setItems(r.data.items);
      else setErr(errText(r));
    });
  }, [type]);

  async function create() {
    const order = (items?.length ?? 0) + 1;
    const r = await cms<{ id: string }>('POST', 'admin/items', { type, attrs: { order }, texts: {} });
    if (r.ok) router.push(`/content/${r.data.id}`);
    else setErr(errText(r));
  }

  return (
    <>
      <div className="page-h">
        <h1>
          เนื้อหาเว็บไซต์
          <small>Content</small>
        </h1>
        {canEdit(roles) ? (
          <button className="btn" onClick={create}>
            + เพิ่มรายการ
          </button>
        ) : null}
      </div>
      <div className="tabs" role="tablist" aria-label="ประเภทเนื้อหา">
        {types.map((t) => (
          <Link key={t.type} role="tab" aria-selected={t.type === type} href={`/content?type=${t.type}`} className="tab">
            {t.label}
          </Link>
        ))}
      </div>
      {err ? <p className="alert">{err}</p> : null}
      {items === null ? (
        <div className="skel" />
      ) : items.length === 0 ? (
        <p className="hint">ยังไม่มีรายการ</p>
      ) : (
        <ul className="clist">
          {items.map((it) => (
            <li key={it.id}>
              <Link href={`/content/${it.id}`} className="crow">
                <span className="cnum">{String(it.attrs.order ?? '-')}</span>
                <span className="ctitle">{headline(it.texts)}</span>
                <span className={`badge b-${it.status}`}>{STATUS_TH[it.status]}</span>
                {!it.published ? <span className="badge b-draft">ยังไม่เคยเผยแพร่</span> : null}
                <small className="hint">{fmtTime(it.updatedAt)}</small>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
