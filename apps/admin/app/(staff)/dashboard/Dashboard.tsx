'use client';

import { useCallback, useEffect, useState } from 'react';
import { formatBaht, num, timeHHmmss } from '@/lib/format';

type Commerce = { salesSatang: number; orders: number; tickets: number; byKind?: Record<string, number> };
type Member = { newMembersToday: number; totalMembers: number };
type Ticket = { scansToday: number; ticketsIssuedToday: number };
type Slot<T> = { data?: T; error?: boolean };

async function get<T>(path: string): Promise<Slot<T>> {
  try {
    const r = await fetch(`/bff/${path}`, { cache: 'no-store' });
    if (r.status === 401) {
      window.location.assign('/login?next=/dashboard');
      return { error: true };
    }
    if (!r.ok) return { error: true };
    return { data: (await r.json()) as T };
  } catch {
    return { error: true };
  }
}

export default function Dashboard() {
  const [c, setC] = useState<Slot<Commerce> | null>(null);
  const [m, setM] = useState<Slot<Member> | null>(null);
  const [t, setT] = useState<Slot<Ticket> | null>(null);
  const [at, setAt] = useState<Date | null>(null);

  const load = useCallback(async () => {
    const [a, b, d] = await Promise.all([
      get<Commerce>('commerce/admin/stats/today'),
      get<Member>('member/admin/stats/today'),
      get<Ticket>('ticket/admin/stats/today'),
    ]);
    // Keep the last good numbers when a refresh fails; only show the error state if there is nothing to show.
    setC((p) => (a.error && p?.data ? p : a));
    setM((p) => (b.error && p?.data ? p : b));
    setT((p) => (d.error && p?.data ? p : d));
    if (!a.error || !b.error || !d.error) setAt(new Date());
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 10_000);
    return () => clearInterval(id);
  }, [load]);

  const day = c?.data?.byKind?.day;
  const annual = c?.data?.byKind?.annual;

  return (
    <>
      <div className="page-h">
        <h1>
          ภาพรวมวันนี้
          <small>Today at a glance</small>
        </h1>
        <span className="stamp" aria-live="polite">
          <span className="dot" aria-hidden />
          อัปเดตล่าสุด {at ? timeHHmmss(at) : '--:--:--'}
        </span>
      </div>
      <div className="grid">
        <Card hero k="ยอดขายวันนี้" en="Sales today" slot={c} value={(x) => formatBaht(x.salesSatang)} />
        <Card k="ออเดอร์" en="Orders" slot={c} value={(x) => num(x.orders)} />
        <Card
          k="บัตรที่ขาย"
          en="Tickets sold"
          slot={c}
          value={(x) => num(x.tickets)}
          sub={() => `รายวัน ${num(day ?? 0)} · รายปี ${num(annual ?? 0)}`}
        />
        <Card
          k="สมาชิกใหม่วันนี้"
          en="New members"
          slot={m}
          value={(x) => num(x.newMembersToday)}
          sub={(x) => `ทั้งหมด ${num(x.totalMembers)} คน`}
        />
        <Card
          k="เข้าประตูวันนี้"
          en="Gate entries"
          slot={t}
          value={(x) => num(x.scansToday)}
          sub={(x) => `ออกบัตรวันนี้ ${num(x.ticketsIssuedToday)}`}
        />
      </div>
    </>
  );
}

function Card<T>({ k, en, slot, value, sub, hero }: { k: string; en: string; slot: Slot<T> | null; value: (x: T) => string; sub?: (x: T) => string; hero?: boolean }) {
  const failed = slot?.error && !slot.data;
  return (
    <section className={`card${hero ? ' hero' : ''}${failed ? ' err' : ''}`}>
      <div className="k">
        {k}
        <small>{en}</small>
      </div>
      {!slot ? (
        <div className="skel" aria-label="loading" />
      ) : failed ? (
        <>
          <div className="v">—</div>
          <div className="errmsg">โหลดข้อมูลไม่ได้ · Unavailable</div>
        </>
      ) : (
        <>
          <div className="v">{value(slot.data as T)}</div>
          {sub && <div className="sub">{sub(slot.data as T)}</div>}
        </>
      )}
    </section>
  );
}
