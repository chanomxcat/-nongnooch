export type ScanApi = {
  result?: string;
  kind?: string;
  holderName?: string;
  idLast4?: string;
  visitDate?: string;
  usedAt?: string;
};

export type Tone = 'ok' | 'used' | 'warn' | 'bad';
export type ScanView = { tone: Tone; title: string; en: string; lines: string[] };

const KIND: Record<string, string> = { day: 'บัตรรายวัน / Day Pass', annual: 'บัตรรายปี / Annual Pass' };
export const kindLabel = (k?: string) => (k ? KIND[k] ?? k : '');

/** Maps the ticket service verdict to what the big result panel shows. `hhmm` formats usedAt (Bangkok). */
export function mapScan(r: ScanApi, hhmm: (iso?: string) => string): ScanView {
  switch (r.result) {
    case 'ok':
      return {
        tone: 'ok',
        title: 'ผ่าน',
        en: 'PASS',
        lines: [r.holderName ?? '', r.idLast4 ? `•••• ${r.idLast4}` : '', kindLabel(r.kind), r.visitDate ? `วันเข้าชม ${r.visitDate}` : ''].filter(Boolean),
      };
    case 'used':
      return { tone: 'used', title: 'ใช้แล้ว', en: 'ALREADY USED', lines: [`ใช้ไปเมื่อ ${hhmm(r.usedAt)}`, r.holderName ?? ''].filter(Boolean) };
    case 'wrong_date':
      return { tone: 'warn', title: 'ไม่ใช่วันเข้าชม', en: 'WRONG DATE', lines: r.visitDate ? [`วันเข้าชม ${r.visitDate}`] : [] };
    default:
      return { tone: 'bad', title: 'บัตรไม่ถูกต้อง', en: 'INVALID TICKET', lines: [] };
  }
}

/**
 * Treats continuous sightings of one QR as a single presentation. `accept(qr)` is true only for a
 * new presentation: the QR was not seen in the last `windowMs`. Every sighting (accepted or not)
 * refreshes the timestamp, so a code held in front of the camera is never re-submitted; it counts as
 * new again only after it has been out of view for `windowMs`. `forget(qr)` (failed request, explicit
 * manual entry) and `reset()` ("scan next") let it be submitted immediately.
 */
export function createDebouncer(windowMs = 3000, now: () => number = Date.now) {
  const lastSeen = new Map<string, number>();
  return {
    accept(qr: string): boolean {
      const t = now();
      for (const [k, at] of lastSeen) if (t - at >= windowMs && k !== qr) lastSeen.delete(k);
      const prev = lastSeen.get(qr);
      lastSeen.set(qr, t);
      return prev === undefined || t - prev >= windowMs;
    },
    forget(qr: string) {
      lastSeen.delete(qr);
    },
    reset() {
      lastSeen.clear();
    },
  };
}
