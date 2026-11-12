'use client';

import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';
import { canDecide, cms, diffRevs, errText, fmtTime, headline, STATUS_TH, TYPE_TH, who, type Approval } from '@/lib/cms';

const TABS = ['pending', 'approved', 'rejected'] as const;

export default function Approvals({ me, roles }: { me: string; roles: string[] }) {
  const [status, setStatus] = useState<(typeof TABS)[number]>('pending');
  const [items, setItems] = useState<Approval[] | null>(null);
  const [names, setNames] = useState<Record<string, string>>({});
  const [toast, setToast] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);
  const [busy, setBusy] = useState('');
  const [rejecting, setRejecting] = useState<string>('');
  const [reason, setReason] = useState('');

  const load = useCallback(async () => {
    const r = await cms<{ items: Approval[]; names: Record<string, string> }>('GET', `admin/approvals?status=${status}`);
    if (r.ok) {
      setItems(r.data.items);
      setNames(r.data.names);
    } else setToast({ kind: 'err', text: errText(r) });
  }, [status]);

  useEffect(() => {
    setItems(null);
    load();
  }, [load]);
  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 7000);
    return () => clearTimeout(t);
  }, [toast]);

  async function decide(a: Approval, approve: boolean) {
    setBusy(a.id);
    const r = approve
      ? await cms('POST', `admin/approvals/${a.id}/approve`)
      : await cms('POST', `admin/approvals/${a.id}/reject`, { reason });
    setBusy('');
    if (!r.ok) return setToast({ kind: 'err', text: errText(r) });
    setToast({ kind: 'ok', text: approve ? 'อนุมัติแล้ว เนื้อหาเผยแพร่บนเว็บไซต์ทันที' : 'ไม่อนุมัติแล้ว ส่งกลับให้ผู้แก้ไข' });
    setRejecting('');
    setReason('');
    load();
  }

  return (
    <>
      <div className="page-h">
        <h1>
          รออนุมัติ
          <small>Approvals · หลักสี่ตา: ผู้แก้ไขอนุมัติงานของตัวเองไม่ได้</small>
        </h1>
      </div>
      <div className="tabs" role="tablist" aria-label="สถานะ">
        {TABS.map((t) => (
          <button key={t} type="button" role="tab" className="tab" aria-selected={status === t} onClick={() => setStatus(t)}>
            {STATUS_TH[t]}
          </button>
        ))}
      </div>
      {items === null ? (
        <div className="skel" />
      ) : items.length === 0 ? (
        <p className="hint">ไม่มีรายการ</p>
      ) : (
        <div className="alist">
          {items.map((a) => {
            const rows = diffRevs(a.before, a.after);
            const mine = a.editors.includes(me);
            return (
              <section key={a.id} className="panel approval" data-testid="approval">
                <div className="ahead">
                  <div>
                    <span className="hint">{TYPE_TH[a.type] ?? a.type}</span>
                    <h2>
                      <Link href={`/content/${a.itemId}`}>{headline({ ...a.after?.texts.en, ...a.after?.texts.th })}</Link>
                    </h2>
                    <p className="hint">
                      แก้ไขโดย {a.editors.map((e) => who(names, e)).join(', ')} · ส่งเมื่อ {fmtTime(a.updatedAt)}
                      {a.decidedBy ? ` · ตัดสินโดย ${who(names, a.decidedBy)}` : ''}
                      {mine ? ' · (คุณเป็นผู้แก้ไข)' : ''}
                    </p>
                    {a.reason ? <p className="alert">เหตุผล: {a.reason}</p> : null}
                  </div>
                  <span className={`badge b-${a.status}`}>{STATUS_TH[a.status]}</span>
                </div>
                <table className="diff">
                  <thead>
                    <tr>
                      <th scope="col">ช่อง</th>
                      <th scope="col">ก่อน (เผยแพร่อยู่)</th>
                      <th scope="col">หลัง (ฉบับร่าง)</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.length === 0 ? (
                      <tr>
                        <td colSpan={3} className="hint">
                          ไม่มีการเปลี่ยนแปลง
                        </td>
                      </tr>
                    ) : (
                      rows.map((r) => (
                        <tr key={r.key}>
                          <th scope="row">{r.key}</th>
                          <td>
                            <del>{r.before || '—'}</del>
                          </td>
                          <td>
                            <ins>{r.after || '—'}</ins>
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
                {a.status === 'pending' ? (
                  <div className="actions">
                    <button className="btn" disabled={busy === a.id} onClick={() => decide(a, true)}>
                      อนุมัติ
                    </button>
                    <button className="btn btn-ghost" disabled={busy === a.id} onClick={() => setRejecting(rejecting === a.id ? '' : a.id)}>
                      ไม่อนุมัติ
                    </button>
                    {!canDecide(roles) ? <span className="hint">บัญชีนี้ไม่มีสิทธิ์อนุมัติ</span> : null}
                  </div>
                ) : null}
                {rejecting === a.id ? (
                  <div className="row">
                    <label className="nn-sr" htmlFor={`r-${a.id}`}>
                      เหตุผลที่ไม่อนุมัติ
                    </label>
                    <input id={`r-${a.id}`} className="in" placeholder="เหตุผลที่ไม่อนุมัติ" value={reason} onChange={(e) => setReason(e.target.value)} />
                    <button className="btn btn-gold" disabled={busy === a.id} onClick={() => decide(a, false)}>
                      ยืนยันไม่อนุมัติ
                    </button>
                  </div>
                ) : null}
              </section>
            );
          })}
        </div>
      )}
      {toast ? (
        <div className={`toast t-${toast.kind}`} role={toast.kind === 'err' ? 'alert' : 'status'} onClick={() => setToast(null)}>
          {toast.text}
        </div>
      ) : null}
    </>
  );
}
