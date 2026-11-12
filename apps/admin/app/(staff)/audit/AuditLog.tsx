'use client';

import { useCallback, useEffect, useState } from 'react';
import { ACTION_TH, cms, diffRevs, errText, fmtTime, who, type AuditRow, type Rev } from '@/lib/cms';

type Page = { items: AuditRow[]; nextCursor: string; names: Record<string, string> };

const asRev = (v: unknown): Rev | null => (v && typeof v === 'object' && 'texts' in v ? (v as Rev) : null);

export default function AuditLog() {
  const [rows, setRows] = useState<AuditRow[] | null>(null);
  const [names, setNames] = useState<Record<string, string>>({});
  const [next, setNext] = useState('');
  const [err, setErr] = useState('');
  const [open, setOpen] = useState('');

  const load = useCallback(async (cursor = '') => {
    const r = await cms<Page>('GET', `admin/audit?limit=30${cursor ? `&cursor=${cursor}` : ''}`);
    if (!r.ok) return setErr(errText(r));
    setRows((p) => (cursor && p ? [...p, ...r.data.items] : r.data.items));
    setNames((p) => ({ ...p, ...r.data.names }));
    setNext(r.data.nextCursor);
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <>
      <div className="page-h">
        <h1>
          Audit log
          <small>ใคร ทำอะไร เมื่อไร ก่อน/หลัง</small>
        </h1>
        <button className="btn btn-ghost" onClick={() => load()}>
          รีเฟรช
        </button>
      </div>
      {err ? <p className="alert">{err}</p> : null}
      {rows === null ? (
        <div className="skel" />
      ) : rows.length === 0 ? (
        <p className="hint">ยังไม่มีรายการ</p>
      ) : (
        <div className="panel">
          <table className="diff audit">
            <thead>
              <tr>
                <th scope="col">เวลา</th>
                <th scope="col">ผู้ทำ</th>
                <th scope="col">การกระทำ</th>
                <th scope="col">รายการ</th>
                <th scope="col">รายละเอียด</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => {
                const b = asRev(r.before);
                const a = asRev(r.after);
                const diff = a ? diffRevs(b, a) : [];
                return (
                  <tr key={r.id}>
                    <td>{fmtTime(r.at)}</td>
                    <td>
                      {who(names, r.actorId)}
                      <br />
                      <small className="hint">{r.actorRoles.join(', ')}</small>
                    </td>
                    <td>{ACTION_TH[r.action] ?? r.action}</td>
                    <td>
                      <small className="hint">
                        {r.entity}
                        <br />
                        {r.entityId.slice(0, 8)}…
                      </small>
                    </td>
                    <td>
                      {r.reason ? <div>เหตุผล: {r.reason}</div> : null}
                      {diff.length ? (
                        <button className="linkbtn" onClick={() => setOpen(open === r.id ? '' : r.id)}>
                          {open === r.id ? 'ซ่อน' : `ดูก่อน/หลัง (${diff.length} ช่อง)`}
                        </button>
                      ) : null}
                      {open === r.id ? (
                        <ul className="mini">
                          {diff.map((d) => (
                            <li key={d.key}>
                              <b>{d.key}</b>: <del>{d.before || '—'}</del> → <ins>{d.after || '—'}</ins>
                            </li>
                          ))}
                        </ul>
                      ) : null}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {next ? (
            <button className="btn btn-ghost" onClick={() => load(next)}>
              โหลดเพิ่ม
            </button>
          ) : null}
        </div>
      )}
    </>
  );
}
